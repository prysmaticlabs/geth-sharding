package interchangeformat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/validator/db"
	"github.com/prysmaticlabs/prysm/validator/db/kv"
	attestinghistory "github.com/prysmaticlabs/prysm/validator/slashing-protection/local/attesting-history"
)

// ImportStandardProtectionJSON takes in EIP-3076 compliant JSON file used for slashing protection
// by eth2 validators and imports its data into Prysm's internal representation of slashing
// protection in the validator client's database. For more information, see the EIP document here:
// https://eips.ethereum.org/EIPS/eip-3076.
func ImportStandardProtectionJSON(ctx context.Context, validatorDB db.Database, r io.Reader) error {
	encodedJSON, err := ioutil.ReadAll(r)
	if err != nil {
		return errors.Wrap(err, "could not read slashing protection JSON file")
	}
	interchangeJSON := &EIPSlashingProtectionFormat{}
	if err := json.Unmarshal(encodedJSON, interchangeJSON); err != nil {
		return errors.Wrap(err, "could not unmarshal slashing protection JSON file")
	}
	if interchangeJSON.Data == nil {
		log.Warn("No slashing protection data to import")
		return nil
	}

	// We validate the `Metadata` field of the slashing protection JSON file.
	if err := validateMetadata(ctx, validatorDB, interchangeJSON); err != nil {
		return errors.Wrap(err, "slashing protection JSON metadata was incorrect")
	}

	// We need to handle duplicate public keys in the JSON file, with potentially
	// different signing histories for both attestations and blocks.
	signedBlocksByPubKey, err := parseBlocksForUniquePublicKeys(interchangeJSON.Data)
	if err != nil {
		return errors.Wrap(err, "could not parse unique entries for blocks by public key")
	}
	signedAttsByPubKey, err := parseAttestationsForUniquePublicKeys(interchangeJSON.Data)
	if err != nil {
		return errors.Wrap(err, "could not parse unique entries for attestations by public key")
	}

	attestingHistoryByPubKey := make(map[[48]byte]kv.EncHistoryData)
	proposalHistoryByPubKey := make(map[[48]byte]kv.ProposalHistoryForPubkey)
	for pubKey, signedBlocks := range signedBlocksByPubKey {
		// Transform the processed signed blocks data from the JSON
		// file into the internal Prysm representation of proposal history.
		proposalHistory, err := transformSignedBlocks(ctx, signedBlocks)
		if err != nil {
			return errors.Wrapf(err, "could not parse signed blocks in JSON file for key %#x", pubKey)
		}
		proposalHistoryByPubKey[pubKey] = *proposalHistory
	}

	for pubKey, signedAtts := range signedAttsByPubKey {
		// Transform the processed signed attestation data from the JSON
		// file into the internal Prysm representation of attesting history.
		attestingHistory, err := transformSignedAttestations(ctx, signedAtts)
		if err != nil {
			return errors.Wrapf(err, "could not parse signed attestations in JSON file for key %#x", pubKey)
		}
		attestingHistoryByPubKey[pubKey] = *attestingHistory
	}

	// We validate and filter out public keys parsed from JSON to ensure we are
	// not importing those which are slashable with respect to other data within the same JSON.
	slashableProposerKeys := filterSlashablePubKeysFromBlocks(ctx, proposalHistoryByPubKey)
	slashableAttesterKeys, err := filterSlashablePubKeysFromAttestations(
		ctx, attestingHistoryByPubKey, signedAttsByPubKey,
	)
	if err != nil {
		return errors.Wrap(err, "could not filter slashable attester public keys from JSON data")
	}

	slashablePublicKeys := make([][48]byte, 0, len(slashableAttesterKeys)+len(slashableProposerKeys))
	for _, pubKey := range slashableProposerKeys {
		delete(proposalHistoryByPubKey, pubKey)
		slashablePublicKeys = append(slashablePublicKeys, pubKey)
	}
	for _, pubKey := range slashableAttesterKeys {
		delete(attestingHistoryByPubKey, pubKey)
		slashablePublicKeys = append(slashablePublicKeys, pubKey)
	}

	if err := validatorDB.SaveEIPImportBlacklistedPublicKeys(ctx, slashablePublicKeys); err != nil {
		return errors.Wrap(err, "could not save slashable public keys to database")
	}

	// We save the histories to disk as atomic operations, ensuring that this only occurs
	// until after we successfully parse all data from the JSON file. If there is any error
	// in parsing the JSON proposal and attesting histories, we will not reach this point.
	for pubKey, proposalHistory := range proposalHistoryByPubKey {
		bar := initializeProgressBar(
			len(proposalHistory.Proposals),
			fmt.Sprintf("Importing proposals for validator public key %#x", bytesutil.Trunc(pubKey[:])),
		)
		for _, proposal := range proposalHistory.Proposals {
			if err := bar.Add(1); err != nil {
				log.WithError(err).Debug("Could not increase progress bar")
			}
			if err = validatorDB.SaveProposalHistoryForSlot(ctx, pubKey, proposal.Slot, proposal.SigningRoot); err != nil {
				return errors.Wrap(err, "could not save proposal history from imported JSON to database")
			}
		}
	}
	bar := initializeProgressBar(
		len(attestingHistoryByPubKey),
		"Importing attesting history for validator public keys",
	)
	for pubKey, history := range attestingHistoryByPubKey {
		if err := bar.Add(1); err != nil {
			log.WithError(err).Debug("Could not increase progress bar")
		}
		if err := validatorDB.SaveAttestationHistoryForPubKeyV2(ctx, pubKey, history); err != nil {
			return errors.Wrap(err, "could not save attesting history from imported JSON to database")
		}
	}
	return saveLowestSourceTargetToDB(ctx, validatorDB, signedAttsByPubKey)
}

func validateMetadata(ctx context.Context, validatorDB db.Database, interchangeJSON *EIPSlashingProtectionFormat) error {
	// We need to ensure the version in the metadata field matches the one we support.
	version := interchangeJSON.Metadata.InterchangeFormatVersion
	if version != INTERCHANGE_FORMAT_VERSION {
		return fmt.Errorf(
			"slashing protection JSON version '%s' is not supported, wanted '%s'",
			version,
			INTERCHANGE_FORMAT_VERSION,
		)
	}

	// We need to verify the genesis validators root matches that of our chain data, otherwise
	// the imported slashing protection JSON was created on a different chain.
	gvr, err := RootFromHex(interchangeJSON.Metadata.GenesisValidatorsRoot)
	if err != nil {
		return fmt.Errorf("%#x is not a valid root: %v", interchangeJSON.Metadata.GenesisValidatorsRoot, err)
	}
	dbGvr, err := validatorDB.GenesisValidatorsRoot(ctx)
	if err != nil {
		return errors.Wrap(err, "could not retrieve genesis validator root to db")
	}
	if dbGvr == nil {
		if err = validatorDB.SaveGenesisValidatorsRoot(ctx, gvr[:]); err != nil {
			return errors.Wrap(err, "could not save genesis validator root to db")
		}
		return nil
	}
	if !bytes.Equal(dbGvr, gvr[:]) {
		return errors.New("genesis validator root doesnt match the one that is stored in slashing protection db. " +
			"Please make sure you import the protection data that is relevant to the chain you are on")
	}
	return nil
}

// We create a map of pubKey -> []*SignedBlock. Then, for each public key we observe,
// we append to this map. This allows us to handle valid input JSON data such as:
//
// "0x2932232930: {
//   SignedBlocks: [Slot: 5, Slot: 6, Slot: 7],
//  },
// "0x2932232930: {
//   SignedBlocks: [Slot: 5, Slot: 10, Slot: 11],
//  }
//
// Which should be properly parsed as:
//
// "0x2932232930: {
//   SignedBlocks: [Slot: 5, Slot: 5, Slot: 6, Slot: 7, Slot: 10, Slot: 11],
//  }
func parseBlocksForUniquePublicKeys(data []*ProtectionData) (map[[48]byte][]*SignedBlock, error) {
	signedBlocksByPubKey := make(map[[48]byte][]*SignedBlock)
	for _, validatorData := range data {
		pubKey, err := PubKeyFromHex(validatorData.Pubkey)
		if err != nil {
			return nil, fmt.Errorf("%s is not a valid public key: %v", validatorData.Pubkey, err)
		}
		for _, sBlock := range validatorData.SignedBlocks {
			if sBlock == nil {
				continue
			}
			signedBlocksByPubKey[pubKey] = append(signedBlocksByPubKey[pubKey], sBlock)
		}
	}
	return signedBlocksByPubKey, nil
}

// We create a map of pubKey -> []*SignedAttestation. Then, for each public key we observe,
// we append to this map. This allows us to handle valid input JSON data such as:
//
// "0x2932232930: {
//   SignedAttestations: [{Source: 5, Target: 6}, {Source: 6, Target: 7}],
//  },
// "0x2932232930: {
//   SignedAttestations: [{Source: 5, Target: 6}],
//  }
//
// Which should be properly parsed as:
//
// "0x2932232930: {
//   SignedAttestations: [{Source: 5, Target: 6}, {Source: 5, Target: 6}, {Source: 6, Target: 7}],
//  }
func parseAttestationsForUniquePublicKeys(data []*ProtectionData) (map[[48]byte][]*SignedAttestation, error) {
	signedAttestationsByPubKey := make(map[[48]byte][]*SignedAttestation)
	for _, validatorData := range data {
		pubKey, err := PubKeyFromHex(validatorData.Pubkey)
		if err != nil {
			return nil, fmt.Errorf("%s is not a valid public key: %v", validatorData.Pubkey, err)
		}
		for _, sAtt := range validatorData.SignedAttestations {
			if sAtt == nil {
				continue
			}
			signedAttestationsByPubKey[pubKey] = append(signedAttestationsByPubKey[pubKey], sAtt)
		}
	}
	return signedAttestationsByPubKey, nil
}

func filterSlashablePubKeysFromBlocks(ctx context.Context, historyByPubKey map[[48]byte]kv.ProposalHistoryForPubkey) [][48]byte {
	// Given signing roots are optional in the EIP standard, we behave as follows:
	// For a given block:
	//   If we have a previous block with the same slot in our history:
	//     If signing root is nil, we consider that proposer public key as slashable
	//     If signing root is not nil , then we compare signing roots. If they are different,
	//     then we consider that proposer public key as slashable.
	slashablePubKeys := make([][48]byte, 0)
	for pubKey, proposals := range historyByPubKey {
		seenSigningRootsBySlot := make(map[uint64][]byte)
		for _, blk := range proposals.Proposals {
			if signingRoot, ok := seenSigningRootsBySlot[blk.Slot]; ok {
				if signingRoot == nil || !bytes.Equal(signingRoot, blk.SigningRoot) {
					slashablePubKeys = append(slashablePubKeys, pubKey)
					break
				}
			}
			seenSigningRootsBySlot[blk.Slot] = blk.SigningRoot
		}
	}
	return slashablePubKeys
}

func filterSlashablePubKeysFromAttestations(
	ctx context.Context,
	historyByPubKey map[[48]byte]kv.EncHistoryData,
	signedAttsByPubKey map[[48]byte][]*SignedAttestation,
) ([][48]byte, error) {
	slashablePubKeys := make([][48]byte, 0)
	for pubKey, signedAtts := range signedAttsByPubKey {
		history, ok := historyByPubKey[pubKey]
		if !ok {
			// This should not happen, as the map is populated prior to calling this function.
			return nil, fmt.Errorf("could not find history for public key %#x", pubKey)
		}
		for _, att := range signedAtts {
			// Malformed data should not prevent us from completing this function.
			source, err := Uint64FromString(att.SourceEpoch)
			if err != nil {
				continue
			}
			target, err := Uint64FromString(att.TargetEpoch)
			if err != nil {
				continue
			}
			signingRoot, err := RootFromHex(att.SigningRoot)
			if err != nil {
				continue
			}
			slashable, err := attestinghistory.IsNewAttSlashable(ctx, history, source, target, signingRoot)
			if err != nil {
				continue
			}
			if slashable {
				slashablePubKeys = append(slashablePubKeys, pubKey)
				break
			}
		}
	}
	return slashablePubKeys, nil
}

func transformSignedBlocks(ctx context.Context, signedBlocks []*SignedBlock) (*kv.ProposalHistoryForPubkey, error) {
	proposals := make([]kv.Proposal, len(signedBlocks))
	for i, proposal := range signedBlocks {
		slot, err := Uint64FromString(proposal.Slot)
		if err != nil {
			return nil, fmt.Errorf("%d is not a valid slot: %v", slot, err)
		}
		var signingRoot [32]byte
		// Signing roots are optional in the standard JSON file.
		if proposal.SigningRoot != "" {
			signingRoot, err = RootFromHex(proposal.SigningRoot)
			if err != nil {
				return nil, fmt.Errorf("%#x is not a valid root: %v", signingRoot, err)
			}
		}
		proposals[i] = kv.Proposal{
			Slot:        slot,
			SigningRoot: signingRoot[:],
		}
	}
	return &kv.ProposalHistoryForPubkey{
		Proposals: proposals,
	}, nil
}

func transformSignedAttestations(ctx context.Context, atts []*SignedAttestation) (*kv.EncHistoryData, error) {
	attestingHistory := kv.NewAttestationHistoryArray(0)
	highestEpochWritten := uint64(0)
	var err error
	for _, attestation := range atts {
		target, err := Uint64FromString(attestation.TargetEpoch)
		if err != nil {
			return nil, fmt.Errorf("%d is not a valid epoch: %v", target, err)
		}
		// Keep track of the highest epoch written from the imported JSON.
		if target > highestEpochWritten {
			highestEpochWritten = target
		}
		source, err := Uint64FromString(attestation.SourceEpoch)
		if err != nil {
			return nil, fmt.Errorf("%d is not a valid epoch: %v", source, err)
		}
		var signingRoot [32]byte
		// Signing roots are optional in the standard JSON file.
		if attestation.SigningRoot != "" {
			signingRoot, err = RootFromHex(attestation.SigningRoot)
			if err != nil {
				return nil, fmt.Errorf("%#x is not a valid root: %v", signingRoot, err)
			}
		}
		attestingHistory, err = attestingHistory.SetTargetData(
			ctx, target, &kv.HistoryData{Source: source, SigningRoot: signingRoot[:]},
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not set target data for attesting history")
		}
	}
	attestingHistory, err = attestingHistory.SetLatestEpochWritten(ctx, highestEpochWritten)
	if err != nil {
		return nil, errors.Wrap(err, "could not set latest epoch written")
	}
	return &attestingHistory, nil
}

// This saves the lowest source and target epoch from the individual validator to the DB.
func saveLowestSourceTargetToDB(ctx context.Context, validatorDB db.Database, signedAttsByPubKey map[[48]byte][]*SignedAttestation) error {
	validatorLowestSourceEpoch := make(map[[48]byte]uint64) // Validator public key to lowest attested source epoch.
	validatorLowestTargetEpoch := make(map[[48]byte]uint64) // Validator public key to lowest attested target epoch.
	for pubKey, signedAtts := range signedAttsByPubKey {
		for _, att := range signedAtts {
			source, err := Uint64FromString(att.SourceEpoch)
			if err != nil {
				return fmt.Errorf("%d is not a valid source: %v", source, err)
			}
			target, err := Uint64FromString(att.TargetEpoch)
			if err != nil {
				return fmt.Errorf("%d is not a valid target: %v", target, err)
			}
			se, ok := validatorLowestSourceEpoch[pubKey]
			if !ok {
				validatorLowestSourceEpoch[pubKey] = source
			} else if source < se {
				validatorLowestSourceEpoch[pubKey] = source
			}
			te, ok := validatorLowestTargetEpoch[pubKey]
			if !ok {
				validatorLowestTargetEpoch[pubKey] = target
			} else if target < te {
				validatorLowestTargetEpoch[pubKey] = target
			}
		}
	}

	// This should not happen.
	if len(validatorLowestTargetEpoch) != len(validatorLowestSourceEpoch) {
		return errors.New("incorrect source and target map length")
	}

	// Save lowest source and target epoch to DB for every validator in the map.
	for k, v := range validatorLowestSourceEpoch {
		if err := validatorDB.SaveLowestSignedSourceEpoch(ctx, k, v); err != nil {
			return err
		}
	}
	for k, v := range validatorLowestTargetEpoch {
		if err := validatorDB.SaveLowestSignedTargetEpoch(ctx, k, v); err != nil {
			return err
		}
	}
	return nil
}

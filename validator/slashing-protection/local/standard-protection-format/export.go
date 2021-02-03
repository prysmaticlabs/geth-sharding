package interchangeformat

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/progressutil"
	"github.com/prysmaticlabs/prysm/validator/db"
	"github.com/prysmaticlabs/prysm/validator/slashing-protection/local/standard-protection-format/format"
)

// ExportStandardProtectionJSON extracts all slashing protection data from a validator database
// and packages it into an EIP-3076 compliant, standard
func ExportStandardProtectionJSON(ctx context.Context, validatorDB db.Database) (*format.EIPSlashingProtectionFormat, error) {
	interchangeJSON := &format.EIPSlashingProtectionFormat{}
	genesisValidatorsRoot, err := validatorDB.GenesisValidatorsRoot(ctx)
	if err != nil {
		return nil, err
	}
	genesisRootHex, err := rootToHexString(genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	interchangeJSON.Metadata.GenesisValidatorsRoot = genesisRootHex
	interchangeJSON.Metadata.InterchangeFormatVersion = format.InterchangeFormatVersion

	// Extract the existing public keys in our database.
	proposedPublicKeys, err := validatorDB.ProposedPublicKeys(ctx)
	if err != nil {
		return nil, err
	}
	attestedPublicKeys, err := validatorDB.AttestedPublicKeys(ctx)
	if err != nil {
		return nil, err
	}
	dataByPubKey := make(map[[48]byte]*format.ProtectionData)

	// Extract the signed proposals by public key.
	progress := progressutil.InitializeProgressBar(
		len(proposedPublicKeys), "Extracting signed blocks by validator public key",
	)
	for _, pubKey := range proposedPublicKeys {
		pubKeyHex, err := pubKeyToHexString(pubKey[:])
		if err != nil {
			return nil, err
		}
		signedBlocks, err := signedBlocksByPubKey(ctx, validatorDB, pubKey)
		if err != nil {
			return nil, err
		}
		dataByPubKey[pubKey] = &format.ProtectionData{
			Pubkey:             pubKeyHex,
			SignedBlocks:       signedBlocks,
			SignedAttestations: nil,
		}
		if err := progress.Add(1); err != nil {
			return nil, err
		}
	}

	// Extract the signed attestations by public key.
	progress = progressutil.InitializeProgressBar(
		len(proposedPublicKeys), "Extracting signed attestations by validator public key",
	)
	for _, pubKey := range attestedPublicKeys {
		pubKeyHex, err := pubKeyToHexString(pubKey[:])
		if err != nil {
			return nil, err
		}
		signedAttestations, err := signedAttestationsByPubKey(ctx, validatorDB, pubKey)
		if err != nil {
			return nil, err
		}
		if _, ok := dataByPubKey[pubKey]; ok {
			dataByPubKey[pubKey].SignedAttestations = signedAttestations
		} else {
			dataByPubKey[pubKey] = &format.ProtectionData{
				Pubkey:             pubKeyHex,
				SignedBlocks:       nil,
				SignedAttestations: signedAttestations,
			}
		}
		if err := progress.Add(1); err != nil {
			return nil, err
		}
	}

	// Next we turn our map into a slice as expected by the EIP-3076 JSON standard.
	dataList := make([]*format.ProtectionData, 0)
	for _, item := range dataByPubKey {
		if item.SignedAttestations == nil {
			item.SignedAttestations = make([]*format.SignedAttestation, 0)
		}
		if item.SignedBlocks == nil {
			item.SignedBlocks = make([]*format.SignedBlock, 0)
		}
		dataList = append(dataList, item)
	}
	sort.Slice(dataList, func(i, j int) bool {
		return strings.Compare(dataList[i].Pubkey, dataList[j].Pubkey) < 0
	})
	interchangeJSON.Data = dataList
	return interchangeJSON, nil
}

func signedAttestationsByPubKey(ctx context.Context, validatorDB db.Database, pubKey [48]byte) ([]*format.SignedAttestation, error) {
	// If a key does not have an attestation history in our database, we return nil.
	// This way, a user will be able to export their slashing protection history
	// even if one of their keys does not have a history of signed attestations.
	history, err := validatorDB.AttestationHistoryForPubKey(ctx, pubKey)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get attestation history for public key")
	}
	if history == nil {
		return nil, nil
	}
	signedAttestations := make([]*format.SignedAttestation, 0)
	for _, att := range history {
		var root string
		if !bytes.Equal(att.SigningRoot[:], params.BeaconConfig().ZeroHash[:]) {
			root, err = rootToHexString(att.SigningRoot[:])
			if err != nil {
				return nil, err
			}
		}
		signedAttestations = append(signedAttestations, &format.SignedAttestation{
			TargetEpoch: fmt.Sprintf("%d", att.Target),
			SourceEpoch: fmt.Sprintf("%d", att.Source),
			SigningRoot: root,
		})
	}
	return signedAttestations, nil
}

func signedBlocksByPubKey(ctx context.Context, validatorDB db.Database, pubKey [48]byte) ([]*format.SignedBlock, error) {
	// If a key does not have a lowest or highest signed proposal history
	// in our database, we return nil. This way, a user will be able to export their
	// slashing protection history even if one of their keys does not have a history
	// of signed blocks.
	proposalHistory, err := validatorDB.ProposalHistoryForPubKey(ctx, pubKey)
	if err != nil {
		return nil, err
	}
	signedBlocks := make([]*format.SignedBlock, 0)
	for _, proposal := range proposalHistory {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		signingRootHex, err := rootToHexString(proposal.SigningRoot)
		if err != nil {
			return nil, err
		}
		signedBlocks = append(signedBlocks, &format.SignedBlock{
			Slot:        fmt.Sprintf("%d", proposal.Slot),
			SigningRoot: signingRootHex,
		})
	}
	return signedBlocks, nil
}

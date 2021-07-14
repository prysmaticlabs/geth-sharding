// Package client represents a gRPC polling-based implementation
// of an Ethereum validator client.
package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	accountsiface "github.com/prysmaticlabs/prysm/validator/accounts/iface"
	"github.com/prysmaticlabs/prysm/validator/accounts/wallet"
	"github.com/prysmaticlabs/prysm/validator/client/iface"
	vdb "github.com/prysmaticlabs/prysm/validator/db"
	"github.com/prysmaticlabs/prysm/validator/db/kv"
	"github.com/prysmaticlabs/prysm/validator/graffiti"
	"github.com/prysmaticlabs/prysm/validator/keymanager"
	slashingiface "github.com/prysmaticlabs/prysm/validator/slashing-protection/iface"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// reconnectPeriod is the frequency that we try to restart our
// slasher connection when the slasher client connection is not ready.
var reconnectPeriod = 5 * time.Second

// keyFetchPeriod is the frequency that we try to refetch validating keys
// in case no keys were fetched previously.
var keyRefetchPeriod = 30 * time.Second

var (
	msgCouldNotFetchKeys = "could not fetch validating keys"
	msgNoKeysFetched     = "No validating keys fetched. Trying again"
)

type validator struct {
	logValidatorBalances               bool
	useWeb                             bool
	emitAccountMetrics                 bool
	logDutyCountDown                   bool
	domainDataLock                     sync.Mutex
	attLogsLock                        sync.Mutex
	aggregatedSlotCommitteeIDCacheLock sync.Mutex
	prevBalanceLock                    sync.RWMutex
	slashableKeysLock                  sync.RWMutex
	walletInitializedFeed              *event.Feed
	blockFeed                          *event.Feed
	genesisTime                        uint64
	highestValidSlot                   types.Slot
	domainDataCache                    *ristretto.Cache
	aggregatedSlotCommitteeIDCache     *lru.Cache
	ticker                             slotutil.Ticker
	prevBalance                        map[[48]byte]uint64
	duties                             *ethpb.DutiesResponse
	startBalances                      map[[48]byte]uint64
	attLogs                            map[[32]byte]*attSubmitted
	node                               ethpb.NodeClient
	keyManager                         keymanager.IKeymanager
	beaconClient                       ethpb.BeaconChainClient
	validatorClient                    ethpb.BeaconNodeValidatorClient
	protector                          slashingiface.Protector
	db                                 vdb.Database
	graffiti                           []byte
	voteStats                          voteStats
	graffitiStruct                     *graffiti.Graffiti
	graffitiOrderedIndex               uint64
	eipImportBlacklistedPublicKeys     map[[48]byte]bool
}

type validatorStatus struct {
	publicKey []byte
	status    *ethpb.ValidatorStatusResponse
	index     types.ValidatorIndex
}

// Done cleans up the validator.
func (v *validator) Done() {
	v.ticker.Done()
}

// WaitForWalletInitialization checks if the validator needs to wait for
func (v *validator) WaitForWalletInitialization(ctx context.Context) error {
	// This function should only run if we are using managing the
	// validator client using the Prysm web UI.
	if !v.useWeb {
		return nil
	}
	if v.keyManager != nil {
		return nil
	}
	walletChan := make(chan *wallet.Wallet)
	sub := v.walletInitializedFeed.Subscribe(walletChan)
	defer sub.Unsubscribe()
	for {
		select {
		case w := <-walletChan:
			keyManager, err := w.InitializeKeymanager(ctx, accountsiface.InitKeymanagerConfig{ListenForChanges: true})
			if err != nil {
				return errors.Wrap(err, "could not read keymanager")
			}
			v.keyManager = keyManager
			return nil
		case <-ctx.Done():
			return errors.New("context canceled")
		case <-sub.Err():
			log.Error("Subscriber closed, exiting goroutine")
			return nil
		}
	}
}

// WaitForChainStart checks whether the beacon node has started its runtime. That is,
// it calls to the beacon node which then verifies the ETH1.0 deposit contract logs to check
// for the ChainStart log to have been emitted. If so, it starts a ticker based on the ChainStart
// unix timestamp which will be used to keep track of time within the validator client.
func (v *validator) WaitForChainStart(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "validator.WaitForChainStart")
	defer span.End()
	// First, check if the beacon chain has started.
	stream, err := v.validatorClient.WaitForChainStart(ctx, &emptypb.Empty{})
	if err != nil {
		return errors.Wrap(
			iface.ErrConnectionIssue,
			errors.Wrap(err, "could not setup beacon chain ChainStart streaming client").Error(),
		)
	}

	log.Info("Waiting for beacon chain start log from the ETH 1.0 deposit contract")
	chainStartRes, err := stream.Recv()
	if err != io.EOF {
		if ctx.Err() == context.Canceled {
			return errors.Wrap(ctx.Err(), "context has been canceled so shutting down the loop")
		}
		if err != nil {
			return errors.Wrap(
				iface.ErrConnectionIssue,
				errors.Wrap(err, "could not receive ChainStart from stream").Error(),
			)
		}
		v.genesisTime = chainStartRes.GenesisTime
		curGenValRoot, err := v.db.GenesisValidatorsRoot(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get current genesis validators root")
		}
		if len(curGenValRoot) == 0 {
			if err := v.db.SaveGenesisValidatorsRoot(ctx, chainStartRes.GenesisValidatorsRoot); err != nil {
				return errors.Wrap(err, "could not save genesis validator root")
			}
		} else {
			if !bytes.Equal(curGenValRoot, chainStartRes.GenesisValidatorsRoot) {
				log.Errorf("The genesis validators root received from the beacon node does not match what is in " +
					"your validator database. This could indicate that this is a database meant for another network. If " +
					"you were previously running this validator database on another network, please run --clear-db to " +
					"clear the database. If not, please file an issue at https://github.com/prysmaticlabs/prysm/issues")
				return fmt.Errorf(
					"genesis validators root from beacon node (%#x) does not match root saved in validator db (%#x)",
					chainStartRes.GenesisValidatorsRoot,
					curGenValRoot,
				)
			}
		}
	} else {
		return iface.ErrConnectionIssue
	}

	// Once the ChainStart log is received, we update the genesis time of the validator client
	// and begin a slot ticker used to track the current slot the beacon node is in.
	v.ticker = slotutil.NewSlotTicker(time.Unix(int64(v.genesisTime), 0), params.BeaconConfig().SecondsPerSlot)
	log.WithField("genesisTime", time.Unix(int64(v.genesisTime), 0)).Info("Beacon chain started")
	return nil
}

// WaitForSync checks whether the beacon node has sync to the latest head.
func (v *validator) WaitForSync(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "validator.WaitForSync")
	defer span.End()

	s, err := v.node.GetSyncStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return errors.Wrap(iface.ErrConnectionIssue, errors.Wrap(err, "could not get sync status").Error())
	}
	if !s.Syncing {
		return nil
	}

	for {
		select {
		// Poll every half slot.
		case <-time.After(slotutil.DivideSlotBy(2 /* twice per slot */)):
			s, err := v.node.GetSyncStatus(ctx, &emptypb.Empty{})
			if err != nil {
				return errors.Wrap(iface.ErrConnectionIssue, errors.Wrap(err, "could not get sync status").Error())
			}
			if !s.Syncing {
				return nil
			}
			log.Info("Waiting for beacon node to sync to latest chain head")
		case <-ctx.Done():
			return errors.New("context has been canceled, exiting goroutine")
		}
	}
}

// SlasherReady checks if slasher that was configured as external protection
// is reachable.
func (v *validator) SlasherReady(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "validator.SlasherReady")
	defer span.End()
	if featureconfig.Get().SlasherProtection {
		err := v.protector.Status()
		if err == nil {
			return nil
		}
		ticker := time.NewTicker(reconnectPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.WithError(err).Info("Slasher connection wasn't ready. Trying again")
				err = v.protector.Status()
				if err != nil {
					continue
				}
				log.Info("Slasher connection is ready")
				return nil
			case <-ctx.Done():
				log.Debug("Context closed, exiting reconnect external protection")
				return ctx.Err()
			}
		}
	}
	return nil
}

// ReceiveBlocks starts a gRPC client stream listener to obtain
// blocks from the beacon node. Upon receiving a block, the service
// broadcasts it to a feed for other usages to subscribe to.
func (v *validator) ReceiveBlocks(ctx context.Context, connectionErrorChannel chan<- error) {
	stream, err := v.beaconClient.StreamBlocks(ctx, &ethpb.StreamBlocksRequest{VerifiedOnly: true})
	if err != nil {
		log.WithError(err).Error("Failed to retrieve blocks stream, " + iface.ErrConnectionIssue.Error())
		connectionErrorChannel <- errors.Wrap(iface.ErrConnectionIssue, err.Error())
		return
	}

	for {
		if ctx.Err() == context.Canceled {
			log.WithError(ctx.Err()).Error("Context canceled - shutting down blocks receiver")
			return
		}
		res, err := stream.Recv()
		if err != nil {
			log.WithError(err).Error("Could not receive blocks from beacon node, " + iface.ErrConnectionIssue.Error())
			connectionErrorChannel <- errors.Wrap(iface.ErrConnectionIssue, err.Error())
			return
		}
		if res == nil || res.Block == nil {
			continue
		}
		if res.Block.Slot > v.highestValidSlot {
			v.highestValidSlot = res.Block.Slot
		}

		v.blockFeed.Send(res)
	}
}

func (v *validator) checkAndLogValidatorStatus(statuses []*validatorStatus) bool {
	nonexistentIndex := types.ValidatorIndex(^uint64(0))
	var validatorActivated bool
	for _, status := range statuses {
		fields := logrus.Fields{
			"pubKey": fmt.Sprintf("%#x", bytesutil.Trunc(status.publicKey)),
			"status": status.status.Status.String(),
		}
		if status.index != nonexistentIndex {
			fields["index"] = status.index
		}
		log := log.WithFields(fields)
		if v.emitAccountMetrics {
			fmtKey := fmt.Sprintf("%#x", status.publicKey)
			ValidatorStatusesGaugeVec.WithLabelValues(fmtKey).Set(float64(status.status.Status))
		}
		switch status.status.Status {
		case ethpb.ValidatorStatus_UNKNOWN_STATUS:
			log.Info("Waiting for deposit to be observed by beacon node")
		case ethpb.ValidatorStatus_DEPOSITED:
			if status.status.PositionInActivationQueue != 0 {
				log.WithField(
					"positionInActivationQueue", status.status.PositionInActivationQueue,
				).Info("Deposit processed, entering activation queue after finalization")
			}
		case ethpb.ValidatorStatus_PENDING:
			if status.status.ActivationEpoch == params.BeaconConfig().FarFutureEpoch {
				log.WithFields(logrus.Fields{
					"positionInActivationQueue": status.status.PositionInActivationQueue,
				}).Info("Waiting to be assigned activation epoch")
			} else {
				log.WithFields(logrus.Fields{
					"activationEpoch": status.status.ActivationEpoch,
				}).Info("Waiting for activation")
			}
		case ethpb.ValidatorStatus_ACTIVE, ethpb.ValidatorStatus_EXITING:
			validatorActivated = true
		case ethpb.ValidatorStatus_EXITED:
			log.Info("Validator exited")
		case ethpb.ValidatorStatus_INVALID:
			log.Warn("Invalid Eth1 deposit")
		default:
			log.WithFields(logrus.Fields{
				"activationEpoch": status.status.ActivationEpoch,
			}).Info("Validator status")
		}
	}
	return validatorActivated
}

func logActiveValidatorStatus(statuses []*validatorStatus) {
	for _, s := range statuses {
		if s.status.Status != ethpb.ValidatorStatus_ACTIVE {
			continue
		}
		log.WithFields(logrus.Fields{
			"publicKey": fmt.Sprintf("%#x", bytesutil.Trunc(s.publicKey)),
			"index":     s.index,
		}).Info("Validator activated")
	}
}

// CanonicalHeadSlot returns the slot of canonical block currently found in the
// beacon chain via RPC.
func (v *validator) CanonicalHeadSlot(ctx context.Context) (types.Slot, error) {
	ctx, span := trace.StartSpan(ctx, "validator.CanonicalHeadSlot")
	defer span.End()
	head, err := v.beaconClient.GetChainHead(ctx, &emptypb.Empty{})
	if err != nil {
		return 0, errors.Wrap(iface.ErrConnectionIssue, err.Error())
	}
	return head.HeadSlot, nil
}

// NextSlot emits the next slot number at the start time of that slot.
func (v *validator) NextSlot() <-chan types.Slot {
	return v.ticker.C()
}

// SlotDeadline is the start time of the next slot.
func (v *validator) SlotDeadline(slot types.Slot) time.Time {
	secs := time.Duration((slot + 1).Mul(params.BeaconConfig().SecondsPerSlot))
	return time.Unix(int64(v.genesisTime), 0 /*ns*/).Add(secs * time.Second)
}

// CheckDoppelGanger checks if the current actively provided keys have
// any duplicates active in the network.
func (v *validator) CheckDoppelGanger(ctx context.Context) error {
	if !featureconfig.Get().EnableDoppelGanger {
		return nil
	}
	pubkeys, err := v.keyManager.FetchValidatingPublicKeys(ctx)
	if err != nil {
		return err
	}
	log.WithField("keys", len(pubkeys)).Info("Running doppelganger check")
	// Exit early if no validating pub keys are found.
	if len(pubkeys) == 0 {
		return nil
	}
	req := &ethpb.DoppelGangerRequest{ValidatorRequests: []*ethpb.DoppelGangerRequest_ValidatorRequest{}}
	for _, pkey := range pubkeys {
		attRec, err := v.db.AttestationHistoryForPubKey(ctx, pkey)
		if err != nil {
			return err
		}
		if len(attRec) == 0 {
			// If no history exists we simply send in a zero
			// value for the request epoch and root.
			req.ValidatorRequests = append(req.ValidatorRequests,
				&ethpb.DoppelGangerRequest_ValidatorRequest{
					PublicKey:  pkey[:],
					Epoch:      0,
					SignedRoot: make([]byte, 32),
				})
			continue
		}
		r := retrieveLatestRecord(attRec)
		if pkey != r.PubKey {
			return errors.New("attestation record mismatched public key")
		}
		req.ValidatorRequests = append(req.ValidatorRequests,
			&ethpb.DoppelGangerRequest_ValidatorRequest{
				PublicKey:  r.PubKey[:],
				Epoch:      r.Target,
				SignedRoot: r.SigningRoot[:],
			})
	}
	resp, err := v.validatorClient.CheckDoppelGanger(ctx, req)
	if err != nil {
		return err
	}
	// If nothing is returned by the beacon node, we return an
	// error as it is unsafe for us to proceed.
	if resp == nil || resp.Responses == nil || len(resp.Responses) == 0 {
		return errors.New("beacon node returned 0 responses for doppelganger check")
	}
	return buildDuplicateError(resp.Responses)
}

func buildDuplicateError(respones []*ethpb.DoppelGangerResponse_ValidatorResponse) error {
	duplicates := make([][]byte, 0)
	for _, valRes := range respones {
		if valRes.DuplicateExists {
			duplicates = append(duplicates, valRes.PublicKey)
		}
	}
	if len(duplicates) == 0 {
		return nil
	}
	return errors.Errorf("Duplicate instances exists in the network for validator keys: %#x", duplicates)
}

// Ensures that the latest attestion history is retrieved.
func retrieveLatestRecord(recs []*kv.AttestationRecord) *kv.AttestationRecord {
	if len(recs) == 0 {
		return nil
	}
	lastSource := recs[len(recs)-1].Source
	chosenRec := recs[len(recs)-1]
	for i := len(recs) - 1; i >= 0; i-- {
		// Exit if we are now on a different source
		// as it is assumed that all source records are
		// byte sorted.
		if recs[i].Source != lastSource {
			break
		}
		// If we have a smaller target, we do
		// change our chosen record.
		if chosenRec.Target < recs[i].Target {
			chosenRec = recs[i]
		}
	}
	return chosenRec
}

// UpdateDuties checks the slot number to determine if the validator's
// list of upcoming assignments needs to be updated. For example, at the
// beginning of a new epoch.
func (v *validator) UpdateDuties(ctx context.Context, slot types.Slot) error {
	if slot%params.BeaconConfig().SlotsPerEpoch != 0 && v.duties != nil {
		// Do nothing if not epoch start AND assignments already exist.
		return nil
	}
	// Set deadline to end of epoch.
	ss, err := helpers.StartSlot(helpers.SlotToEpoch(slot) + 1)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(ctx, v.SlotDeadline(ss))
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "validator.UpdateAssignments")
	defer span.End()

	validatingKeys, err := v.keyManager.FetchValidatingPublicKeys(ctx)
	if err != nil {
		return err
	}

	// Filter out the slashable public keys from the duties request.
	filteredKeys := make([][48]byte, 0, len(validatingKeys))
	v.slashableKeysLock.RLock()
	for _, pubKey := range validatingKeys {
		if ok := v.eipImportBlacklistedPublicKeys[pubKey]; !ok {
			filteredKeys = append(filteredKeys, pubKey)
		} else {
			log.WithField(
				"publicKey", fmt.Sprintf("%#x", bytesutil.Trunc(pubKey[:])),
			).Warn("Not including slashable public key from slashing protection import " +
				"in request to update validator duties")
		}
	}
	v.slashableKeysLock.RUnlock()

	req := &ethpb.DutiesRequest{
		Epoch:      types.Epoch(slot / params.BeaconConfig().SlotsPerEpoch),
		PublicKeys: bytesutil.FromBytes48Array(filteredKeys),
	}

	// If duties is nil it means we have had no prior duties and just started up.
	resp, err := v.validatorClient.GetDuties(ctx, req)
	if err != nil {
		v.duties = nil // Clear assignments so we know to retry the request.
		log.Error(err)
		return err
	}

	v.duties = resp
	v.logDuties(slot, v.duties.CurrentEpochDuties)

	// Non-blocking call for beacon node to start subscriptions for aggregators.
	go func() {
		if err := v.subscribeToSubnets(context.Background(), resp); err != nil {
			log.WithError(err).Error("Failed to subscribe to subnets")
		}
	}()

	return nil
}

// subscribeToSubnets iterates through each validator duty, signs each slot, and asks beacon node
// to eagerly subscribe to subnets so that the aggregator has attestations to aggregate.
func (v *validator) subscribeToSubnets(ctx context.Context, res *ethpb.DutiesResponse) error {
	subscribeSlots := make([]types.Slot, 0, len(res.CurrentEpochDuties)+len(res.NextEpochDuties))
	subscribeCommitteeIndices := make([]types.CommitteeIndex, 0, len(res.CurrentEpochDuties)+len(res.NextEpochDuties))
	subscribeIsAggregator := make([]bool, 0, len(res.CurrentEpochDuties)+len(res.NextEpochDuties))
	alreadySubscribed := make(map[[64]byte]bool)

	for _, duty := range res.CurrentEpochDuties {
		pk := bytesutil.ToBytes48(duty.PublicKey)
		if duty.Status == ethpb.ValidatorStatus_ACTIVE || duty.Status == ethpb.ValidatorStatus_EXITING {
			attesterSlot := duty.AttesterSlot
			committeeIndex := duty.CommitteeIndex

			alreadySubscribedKey := validatorSubscribeKey(attesterSlot, committeeIndex)
			if _, ok := alreadySubscribed[alreadySubscribedKey]; ok {
				continue
			}

			aggregator, err := v.isAggregator(ctx, duty.Committee, attesterSlot, pk)
			if err != nil {
				return errors.Wrap(err, "could not check if a validator is an aggregator")
			}
			if aggregator {
				alreadySubscribed[alreadySubscribedKey] = true
			}

			subscribeSlots = append(subscribeSlots, attesterSlot)
			subscribeCommitteeIndices = append(subscribeCommitteeIndices, committeeIndex)
			subscribeIsAggregator = append(subscribeIsAggregator, aggregator)
		}
	}

	for _, duty := range res.NextEpochDuties {
		if duty.Status == ethpb.ValidatorStatus_ACTIVE || duty.Status == ethpb.ValidatorStatus_EXITING {
			attesterSlot := duty.AttesterSlot
			committeeIndex := duty.CommitteeIndex

			alreadySubscribedKey := validatorSubscribeKey(attesterSlot, committeeIndex)
			if _, ok := alreadySubscribed[alreadySubscribedKey]; ok {
				continue
			}

			aggregator, err := v.isAggregator(ctx, duty.Committee, attesterSlot, bytesutil.ToBytes48(duty.PublicKey))
			if err != nil {
				return errors.Wrap(err, "could not check if a validator is an aggregator")
			}
			if aggregator {
				alreadySubscribed[alreadySubscribedKey] = true
			}

			subscribeSlots = append(subscribeSlots, attesterSlot)
			subscribeCommitteeIndices = append(subscribeCommitteeIndices, committeeIndex)
			subscribeIsAggregator = append(subscribeIsAggregator, aggregator)
		}
	}

	_, err := v.validatorClient.SubscribeCommitteeSubnets(ctx, &ethpb.CommitteeSubnetsSubscribeRequest{
		Slots:        subscribeSlots,
		CommitteeIds: subscribeCommitteeIndices,
		IsAggregator: subscribeIsAggregator,
	})

	return err
}

// RolesAt slot returns the validator roles at the given slot. Returns nil if the
// validator is known to not have a roles at the slot. Returns UNKNOWN if the
// validator assignments are unknown. Otherwise returns a valid ValidatorRole map.
func (v *validator) RolesAt(ctx context.Context, slot types.Slot) (map[[48]byte][]iface.ValidatorRole, error) {
	rolesAt := make(map[[48]byte][]iface.ValidatorRole)
	for _, duty := range v.duties.Duties {
		var roles []iface.ValidatorRole

		if duty == nil {
			continue
		}
		if len(duty.ProposerSlots) > 0 {
			for _, proposerSlot := range duty.ProposerSlots {
				if proposerSlot != 0 && proposerSlot == slot {
					roles = append(roles, iface.RoleProposer)
					break
				}
			}
		}
		if duty.AttesterSlot == slot {
			roles = append(roles, iface.RoleAttester)

			aggregator, err := v.isAggregator(ctx, duty.Committee, slot, bytesutil.ToBytes48(duty.PublicKey))
			if err != nil {
				return nil, errors.Wrap(err, "could not check if a validator is an aggregator")
			}
			if aggregator {
				roles = append(roles, iface.RoleAggregator)
			}

		}
		if len(roles) == 0 {
			roles = append(roles, iface.RoleUnknown)
		}

		var pubKey [48]byte
		copy(pubKey[:], duty.PublicKey)
		rolesAt[pubKey] = roles
	}
	return rolesAt, nil
}

// GetKeymanager returns the underlying validator's keymanager.
func (v *validator) GetKeymanager() keymanager.IKeymanager {
	return v.keyManager
}

// isAggregator checks if a validator is an aggregator of a given slot and committee,
// it uses a modulo calculated by validator count in committee and samples randomness around it.
func (v *validator) isAggregator(ctx context.Context, committee []types.ValidatorIndex, slot types.Slot, pubKey [48]byte) (bool, error) {
	modulo := uint64(1)
	if len(committee)/int(params.BeaconConfig().TargetAggregatorsPerCommittee) > 1 {
		modulo = uint64(len(committee)) / params.BeaconConfig().TargetAggregatorsPerCommittee
	}

	slotSig, err := v.signSlotWithSelectionProof(ctx, pubKey, slot)
	if err != nil {
		return false, err
	}

	b := hashutil.Hash(slotSig)

	return binary.LittleEndian.Uint64(b[:8])%modulo == 0, nil
}

// UpdateDomainDataCaches by making calls for all of the possible domain data. These can change when
// the fork version changes which can happen once per epoch. Although changing for the fork version
// is very rare, a validator should check these data every epoch to be sure the validator is
// participating on the correct fork version.
func (v *validator) UpdateDomainDataCaches(ctx context.Context, slot types.Slot) {
	for _, d := range [][]byte{
		params.BeaconConfig().DomainRandao[:],
		params.BeaconConfig().DomainBeaconAttester[:],
		params.BeaconConfig().DomainBeaconProposer[:],
		params.BeaconConfig().DomainSelectionProof[:],
		params.BeaconConfig().DomainAggregateAndProof[:],
	} {
		_, err := v.domainData(ctx, helpers.SlotToEpoch(slot), d)
		if err != nil {
			log.WithError(err).Errorf("Failed to update domain data for domain %v", d)
		}
	}
}

// AllValidatorsAreExited informs whether all validators have already exited.
func (v *validator) AllValidatorsAreExited(ctx context.Context) (bool, error) {
	validatingKeys, err := v.keyManager.FetchValidatingPublicKeys(ctx)
	if err != nil {
		return false, errors.Wrap(err, "could not fetch validating keys")
	}
	if len(validatingKeys) == 0 {
		return false, nil
	}
	var publicKeys [][]byte
	for _, key := range validatingKeys {
		copyKey := key
		publicKeys = append(publicKeys, copyKey[:])
	}
	request := &ethpb.MultipleValidatorStatusRequest{
		PublicKeys: publicKeys,
	}
	response, err := v.validatorClient.MultipleValidatorStatus(ctx, request)
	if err != nil {
		return false, err
	}
	if len(response.Statuses) != len(request.PublicKeys) {
		return false, errors.New("number of status responses did not match number of requested keys")
	}
	for _, status := range response.Statuses {
		if status.Status != ethpb.ValidatorStatus_EXITED {
			return false, nil
		}
	}
	return true, nil
}

func (v *validator) domainData(ctx context.Context, epoch types.Epoch, domainType []byte) (*ethpb.DomainResponse, error) {
	v.domainDataLock.Lock()
	defer v.domainDataLock.Unlock()

	req := &ethpb.DomainRequest{
		Epoch:  epoch,
		Domain: domainType,
	}

	key := strings.Join([]string{strconv.FormatUint(uint64(req.Epoch), 10), hex.EncodeToString(req.Domain)}, ",")

	if val, ok := v.domainDataCache.Get(key); ok {
		return proto.Clone(val.(proto.Message)).(*ethpb.DomainResponse), nil
	}

	res, err := v.validatorClient.DomainData(ctx, req)
	if err != nil {
		return nil, err
	}

	v.domainDataCache.Set(key, proto.Clone(res), 1)

	return res, nil
}

func (v *validator) logDuties(slot types.Slot, duties []*ethpb.DutiesResponse_Duty) {
	attesterKeys := make([][]string, params.BeaconConfig().SlotsPerEpoch)
	for i := range attesterKeys {
		attesterKeys[i] = make([]string, 0)
	}
	proposerKeys := make([]string, params.BeaconConfig().SlotsPerEpoch)
	slotOffset := slot - (slot % params.BeaconConfig().SlotsPerEpoch)
	var totalAttestingKeys uint64
	for _, duty := range duties {
		validatorNotTruncatedKey := fmt.Sprintf("%#x", duty.PublicKey)
		if v.emitAccountMetrics {
			ValidatorStatusesGaugeVec.WithLabelValues(validatorNotTruncatedKey).Set(float64(duty.Status))
		}

		// Only interested in validators who are attesting/proposing.
		// Note that SLASHING validators will have duties but their results are ignored by the network so we don't bother with them.
		if duty.Status != ethpb.ValidatorStatus_ACTIVE && duty.Status != ethpb.ValidatorStatus_EXITING {
			continue
		}

		validatorKey := fmt.Sprintf("%#x", bytesutil.Trunc(duty.PublicKey))
		attesterIndex := duty.AttesterSlot - slotOffset
		if attesterIndex >= params.BeaconConfig().SlotsPerEpoch {
			log.WithField("duty", duty).Warn("Invalid attester slot")
		} else {
			attesterKeys[duty.AttesterSlot-slotOffset] = append(attesterKeys[duty.AttesterSlot-slotOffset], validatorKey)
			totalAttestingKeys++
			if v.emitAccountMetrics {
				ValidatorNextAttestationSlotGaugeVec.WithLabelValues(validatorNotTruncatedKey).Set(float64(duty.AttesterSlot))
			}
		}

		for _, proposerSlot := range duty.ProposerSlots {
			proposerIndex := proposerSlot - slotOffset
			if proposerIndex >= params.BeaconConfig().SlotsPerEpoch {
				log.WithField("duty", duty).Warn("Invalid proposer slot")
			} else {
				proposerKeys[proposerIndex] = validatorKey
			}
			if v.emitAccountMetrics {
				ValidatorNextProposalSlotGaugeVec.WithLabelValues(validatorNotTruncatedKey).Set(float64(proposerSlot))
			}
		}
	}
	for i := types.Slot(0); i < params.BeaconConfig().SlotsPerEpoch; i++ {
		if len(attesterKeys[i]) > 0 {
			log.WithFields(logrus.Fields{
				"slot":                  slotOffset + i,
				"slotInEpoch":           (slotOffset + i) % params.BeaconConfig().SlotsPerEpoch,
				"attesterDutiesAtSlot":  len(attesterKeys[i]),
				"totalAttestersInEpoch": totalAttestingKeys,
				"pubKeys":               attesterKeys[i],
			}).Info("Attestation schedule")
		}
		if proposerKeys[i] != "" {
			log.WithField("slot", slotOffset+i).WithField("pubKey", proposerKeys[i]).Info("Proposal schedule")
		}
	}
}

// This constructs a validator subscribed key, it's used to track
// which subnet has already been pending requested.
func validatorSubscribeKey(slot types.Slot, committeeID types.CommitteeIndex) [64]byte {
	return bytesutil.ToBytes64(append(bytesutil.Bytes32(uint64(slot)), bytesutil.Bytes32(uint64(committeeID))...))
}

// This tracks all validators' voting status.
type voteStats struct {
	startEpoch            types.Epoch
	includedAttestedCount uint64
	totalAttestedCount    uint64
	totalDistance         types.Slot
	correctSources        uint64
	totalSources          uint64
	correctTargets        uint64
	totalTargets          uint64
	correctHeads          uint64
	totalHeads            uint64
}

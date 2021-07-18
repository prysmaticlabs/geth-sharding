package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/validator/client/iface"
	"github.com/prysmaticlabs/prysm/validator/keymanager/remote"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// time to wait before trying to reconnect with beacon node.
var backOffPeriod = 10 * time.Second

// Run the main validator routine. This routine exits if the context is
// canceled.
//
// Order of operations:
// 1 - Initialize validator data
// 2 - Wait for validator activation
// 3 - Wait for the next slot start
// 4 - Update assignments
// 5 - Determine role at current slot
// 6 - Perform assigned role, if any
func run(ctx context.Context, v iface.Validator) {
	cleanup := v.Done
	defer cleanup()
	if err := v.WaitForWalletInitialization(ctx); err != nil {
		// log.Fatalf will prevent defer from being called
		cleanup()
		log.Fatalf("Wallet is not ready: %v", err)
	}
	if featureconfig.Get().SlasherProtection {
		if err := v.SlasherReady(ctx); err != nil {
			log.Fatalf("Slasher is not ready: %v", err)
		}
	}
	ticker := time.NewTicker(backOffPeriod)
	defer ticker.Stop()

	var headSlot types.Slot
	firstTime := true
	for {
		if !firstTime {
			if ctx.Err() != nil {
				log.Info("Context canceled, stopping validator")
				return // Exit if context is canceled.
			}
			<-ticker.C
		} else {
			firstTime = false
		}
		err := v.WaitForChainStart(ctx)
		if isConnectionError(err) {
			log.Warnf("Could not determine if beacon chain started: %v", err)
			continue
		}
		if err != nil {
			log.Fatalf("Could not determine if beacon chain started: %v", err)
		}
		err = v.WaitForSync(ctx)
		if isConnectionError(err) {
			log.Warnf("Could not determine if beacon chain started: %v", err)
			continue
		}
		if err != nil {
			log.Fatalf("Could not determine if beacon node synced: %v", err)
		}
		err = v.WaitForActivation(ctx, nil /* accountsChangedChan */)
		if isConnectionError(err) {
			log.Warnf("Could not wait for validator activation: %v", err)
			continue
		}
		if err != nil {
			log.Fatalf("Could not wait for validator activation: %v", err)
		}
		err = v.CheckDoppelGanger(ctx)
		if isConnectionError(err) {
			log.Warnf("Could not wait for checking doppelganger: %v", err)
			continue
		}
		if err != nil {
			log.Fatalf("Could not succeed with doppelganger check: %v", err)
		}
		headSlot, err = v.CanonicalHeadSlot(ctx)
		if isConnectionError(err) {
			log.Warnf("Could not get current canonical head slot: %v", err)
			continue
		}
		if err != nil {
			log.Fatalf("Could not get current canonical head slot: %v", err)
		}
		break
	}

	connectionErrorChannel := make(chan error, 1)
	go v.ReceiveBlocks(ctx, connectionErrorChannel)
	if err := v.UpdateDuties(ctx, headSlot); err != nil {
		handleAssignmentError(err, headSlot)
	}

	accountsChangedChan := make(chan [][48]byte, 1)
	sub := v.GetKeymanager().SubscribeAccountChanges(accountsChangedChan)
	for {
		slotCtx, cancel := context.WithCancel(ctx)
		ctx, span := trace.StartSpan(ctx, "validator.processSlot")

		select {
		case <-ctx.Done():
			log.Info("Context canceled, stopping validator")
			span.End()
			cancel()
			sub.Unsubscribe()
			close(accountsChangedChan)
			return // Exit if context is canceled.
		case blocksError := <-connectionErrorChannel:
			if blocksError != nil {
				log.WithError(blocksError).Warn("block stream interrupted")
				go v.ReceiveBlocks(ctx, connectionErrorChannel)
				continue
			}
		case newKeys := <-accountsChangedChan:
			anyActive, err := v.HandleKeyReload(ctx, newKeys)
			if err != nil {
				log.WithError(err).Error("Could not properly handle reloaded keys")
			}
			if !anyActive {
				log.Info("No active keys found. Waiting for activation...")
				err := v.WaitForActivation(ctx, accountsChangedChan)
				if err != nil {
					log.Fatalf("Could not wait for validator activation: %v", err)
				}
			}
		case slot := <-v.NextSlot():
			span.AddAttributes(trace.Int64Attribute("slot", int64(slot)))

			remoteKm, ok := v.GetKeymanager().(remote.RemoteKeymanager)
			if ok {
				_, err := remoteKm.ReloadPublicKeys(ctx)
				if err != nil {
					log.WithError(err).Error(msgCouldNotFetchKeys)
				}
			}

			allExited, err := v.AllValidatorsAreExited(ctx)
			if err != nil {
				log.WithError(err).Error("Could not check if validators are exited")
			}
			if allExited {
				log.Info("All validators are exited, no more work to perform...")
				continue
			}

			deadline := v.SlotDeadline(slot)
			slotCtx, cancel = context.WithDeadline(ctx, deadline)
			log := log.WithField("slot", slot)
			log.WithField("deadline", deadline).Debug("Set deadline for proposals and attestations")

			// Keep trying to update assignments if they are nil or if we are past an
			// epoch transition in the beacon node's state.
			if err := v.UpdateDuties(ctx, slot); err != nil {
				handleAssignmentError(err, slot)
				cancel()
				span.End()
				continue
			}

			// Start fetching domain data for the next epoch.
			if helpers.IsEpochEnd(slot) {
				go v.UpdateDomainDataCaches(ctx, slot+1)
			}

			var wg sync.WaitGroup

			allRoles, err := v.RolesAt(ctx, slot)
			if err != nil {
				log.WithError(err).Error("Could not get validator roles")
				span.End()
				continue
			}
			for pubKey, roles := range allRoles {
				wg.Add(len(roles))
				for _, role := range roles {
					go func(role iface.ValidatorRole, pubKey [48]byte) {
						defer wg.Done()
						switch role {
						case iface.RoleAttester:
							v.SubmitAttestation(slotCtx, slot, pubKey)
						case iface.RoleProposer:
							v.ProposeBlock(slotCtx, slot, pubKey)
						case iface.RoleAggregator:
							v.SubmitAggregateAndProof(slotCtx, slot, pubKey)
						case iface.RoleSyncCommittee:
							v.SubmitSyncCommitteeMessage(slotCtx, slot, pubKey)
						case iface.RoleSyncCommitteeAggregator:
							v.SubmitSignedContributionAndProof(slotCtx, slot, pubKey)
						case iface.RoleUnknown:
							log.WithField("pubKey", fmt.Sprintf("%#x", bytesutil.Trunc(pubKey[:]))).Trace("No active roles, doing nothing")
						default:
							log.Warnf("Unhandled role %v", role)
						}
					}(role, pubKey)
				}
			}
			// Wait for all processes to complete, then report span complete.

			go func() {
				wg.Wait()
				// Log this client performance in the previous epoch
				v.LogAttestationsSubmitted()
				if err := v.LogValidatorGainsAndLosses(slotCtx, slot); err != nil {
					log.WithError(err).Error("Could not report validator's rewards/penalties")
				}
				if err := v.LogNextDutyTimeLeft(slot); err != nil {
					log.WithError(err).Error("Could not report next count down")
				}
				span.End()
			}()
		}
	}
}

func isConnectionError(err error) bool {
	return err != nil && errors.Is(err, iface.ErrConnectionIssue)
}

func handleAssignmentError(err error, slot types.Slot) {
	if errCode, ok := status.FromError(err); ok && errCode.Code() == codes.NotFound {
		log.WithField(
			"epoch", slot/params.BeaconConfig().SlotsPerEpoch,
		).Warn("Validator not yet assigned to epoch")
	} else {
		log.WithField("error", err).Error("Failed to update assignments")
	}
}

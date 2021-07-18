package sync

import (
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/shared/p2putils"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
)

// Is a background routine that observes for new incoming forks. Depending on the epoch
// it will be in charge of subscribing/unsubscribing the relevant topics at the fork boundaries.
func (s *Service) forkWatcher() {
	slotTicker := slotutil.NewSlotTicker(s.cfg.Chain.GenesisTime(), params.BeaconConfig().SecondsPerSlot)
	for {
		select {
		case currSlot := <-slotTicker.C():
			currEpoch := helpers.SlotToEpoch(currSlot)
			if err := s.checkForNextEpochFork(currEpoch); err != nil {
				log.WithError(err).Error("Unable to check for fork in the next epoch")
				continue
			}
			if err := s.checkForPreviousEpochFork(currEpoch); err != nil {
				log.WithError(err).Error("Unable to check for fork in the previous epoch")
				continue
			}
		case <-s.ctx.Done():
			log.Debug("Context closed, exiting goroutine")
			slotTicker.Done()
			return
		}
	}
}

// Checks if there is a fork in the next epoch.
func (s *Service) checkForNextEpochFork(currEpoch types.Epoch) error {
	genRoot := s.cfg.Chain.GenesisValidatorRoot()
	isNextForkEpoch, err := p2putils.IsForkNextEpoch(s.cfg.Chain.GenesisTime(), genRoot[:])
	if err != nil {
		return errors.Wrap(err, "Could not retrieve next fork epoch")
	}
	// In preparation for the upcoming fork
	// in the following epoch, the node
	// will subscribe the new topics in advance.
	if isNextForkEpoch {
		nextEpoch := currEpoch + 1
		if nextEpoch == params.BeaconConfig().AltairForkEpoch {
			digest, err := p2putils.ForkDigestFromEpoch(nextEpoch, genRoot[:])
			if err != nil {
				return errors.Wrap(err, "Could not retrieve fork digest")
			}
			if s.subHandler.digestExists(digest) {
				return nil
			}
			s.registerSubscribers(nextEpoch, digest)
			s.registerRPCHandlersAltair()
		}
	}
	return nil
}

// Checks if there is a fork in the previous epoch.
func (s *Service) checkForPreviousEpochFork(currEpoch types.Epoch) error {
	genRoot := s.cfg.Chain.GenesisValidatorRoot()
	// This method takes care of the de-registration of
	// old gossip pubsub handlers. Once we are at the epoch
	// after the fork, we de-register from all the outdated topics.
	currFork, err := p2putils.Fork(currEpoch)
	if err != nil {
		return err
	}
	epochAfterFork := currFork.Epoch + 1
	nonGenesisFork := currFork.Epoch > 1
	// If we are in the epoch after the fork, we start de-registering.
	if epochAfterFork == currEpoch && nonGenesisFork {
		// Look at the previous fork's digest.
		epochBeforeFork := currFork.Epoch - 1
		prevDigest, err := p2putils.ForkDigestFromEpoch(epochBeforeFork, genRoot[:])
		if err != nil {
			return errors.Wrap(err, "Failed to determine previous epoch fork digest")
		}

		// Exit early if there are no topics with that particular
		// digest.
		if !s.subHandler.digestExists(prevDigest) {
			return nil
		}
		// Run through all our current active topics and see
		// if there are any subscriptions to be removed.
		for _, t := range s.subHandler.allTopics() {
			retDigest, err := p2p.ExtractGossipDigest(t)
			if err != nil {
				log.WithError(err).Error("Could not retrieve digest")
				continue
			}
			if retDigest == prevDigest {
				if err := s.cfg.P2P.PubSub().UnregisterTopicValidator(t); err != nil {
					log.WithError(err).Error("Could not unregister topic validator")
				}
				s.subHandler.removeTopic(t)
			}
		}
	}
	return nil
}

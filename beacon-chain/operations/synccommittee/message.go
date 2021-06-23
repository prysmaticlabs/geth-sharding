package synccommittee

import (
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/copyutil"
	"github.com/prysmaticlabs/prysm/shared/queue"
)

// SaveSyncCommitteeMessage saves a sync committee message in to a priority queue.
// The priority queue capped at syncCommitteeMaxQueueSize contributions.
func (s *Store) SaveSyncCommitteeMessage(msg *prysmv2.SyncCommitteeMessage) error {
	if msg == nil {
		return nilMessageErr
	}

	messages, err := s.SyncCommitteeMessages(msg.Slot)
	if err != nil {
		return err
	}

	s.messageLock.Lock()
	defer s.messageLock.Unlock()
	copied := copyutil.CopySyncCommitteeMessage(msg)

	// Messages exist in the queue. Append instead of insert new.
	if messages != nil {
		messages = append(messages, copied)
		return s.messageCache.Push(&queue.Item{
			Key:      syncCommitteeKey(msg.Slot),
			Value:    messages,
			Priority: int64(msg.Slot),
		})
	}

	// Message does not exist. Insert new.
	if err := s.messageCache.Push(&queue.Item{
		Key:      syncCommitteeKey(msg.Slot),
		Value:    []*prysmv2.SyncCommitteeMessage{copied},
		Priority: int64(msg.Slot),
	}); err != nil {
		return err
	}

	// Trim messages in queue down to syncCommitteeMaxQueueSize.
	if s.messageCache.Len() > syncCommitteeMaxQueueSize {
		if _, err := s.messageCache.Pop(); err != nil {
			return err
		}
	}

	return nil
}

// SyncCommitteeMessages returns sync committee messages by slot from the priority queue.
// Upon retrieval, the message is removed from the queue.
func (s *Store) SyncCommitteeMessages(slot types.Slot) ([]*prysmv2.SyncCommitteeMessage, error) {
	s.messageLock.Lock()
	defer s.messageLock.Unlock()

	item, err := s.messageCache.PopByKey(syncCommitteeKey(slot))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}

	messages, ok := item.Value.([]*prysmv2.SyncCommitteeMessage)
	if !ok {
		return nil, errors.New("not typed []prysmv2.SyncCommitteeMessage")
	}

	return messages, nil
}

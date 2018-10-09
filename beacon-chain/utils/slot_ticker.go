package utils

import (
	"time"

	"github.com/prysmaticlabs/prysm/beacon-chain/params"
)

// SlotTicker is a special ticker for the beacon chain block.
// The channel emits over the slot interval, and ensures that
// the ticks are in line with the genesis time. This means that
// the duration between the ticks and the genesis time are always a
// multiple of the slot duration.
// In addition, the channel returns the new slot number.
type SlotTicker struct {
	c    chan uint64
	done chan struct{}
}

// C returns the ticker channel. Call Cancel afterwards to ensure
// that the goroutine exits cleanly.
func (s *SlotTicker) C() <-chan uint64 {
	return s.c
}

// Done should be called to clean up the ticker.
func (s *SlotTicker) Done() {
	go func() {
		s.done <- struct{}{}
	}()
}

// GetSlotTicker is the constructor for SlotTicker
func GetSlotTicker(genesisTime time.Time) SlotTicker {
	ticker := SlotTicker{
		c:    make(chan uint64),
		done: make(chan struct{}),
	}
	ticker.start(genesisTime, params.GetConfig().SlotDuration, time.Since, time.Until, time.After)

	return ticker
}

func (s *SlotTicker) start(
	genesisTime time.Time,
	slotDuration uint64,
	since func(time.Time) time.Duration,
	until func(time.Time) time.Duration,
	after func(time.Duration) <-chan time.Time) {
	d := time.Duration(slotDuration) * time.Second

	go func() {
		sinceGenesis := since(genesisTime)

		var nextTickTime time.Time
		var slot uint64
		if sinceGenesis < 0 {
			// Handle when the current time is before the genesis time
			nextTickTime = genesisTime
			slot = 0
		} else {
			nextTick := sinceGenesis.Truncate(d) + d
			nextTickTime = genesisTime.Add(nextTick)
			slot = uint64(nextTick / d)
		}

		for {
			waitTime := until(nextTickTime)
			select {
			case <-after(waitTime):
				s.c <- slot
				slot++
				nextTickTime = nextTickTime.Add(d)
			case <-s.done:
				return
			}
		}
	}()
}

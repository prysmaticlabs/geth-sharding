package slotutil

import (
	"testing"
	"time"

	types "github.com/prysmaticlabs/eth2-types"
)

func TestEpochTicker(t *testing.T) {
	ticker := &EpochTicker{
		c:    make(chan types.Epoch),
		done: make(chan struct{}),
	}
	defer ticker.Done()

	var sinceDuration time.Duration
	since := func(time.Time) time.Duration {
		return sinceDuration
	}

	var untilDuration time.Duration
	until := func(time.Time) time.Duration {
		return untilDuration
	}

	var tick chan time.Time
	after := func(time.Duration) <-chan time.Time {
		return tick
	}

	genesisTime := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	secondsPerEpoch := uint64(8)

	// Test when the ticker starts immediately after genesis time.
	sinceDuration = 1 * time.Second
	untilDuration = 7 * time.Second

	// Make this a buffered channel to prevent a deadlock since
	// the other goroutine calls a function in this goroutine.
	tick = make(chan time.Time, 2)
	ticker.start(genesisTime, secondsPerEpoch, since, until, after)

	// Tick once.
	tick <- time.Now()
	epoch := <-ticker.C()
	if epoch != 1 {
		t.Fatalf("Expected %d, got %d", 1, epoch)
	}

	// Tick twice.
	tick <- time.Now()
	epoch = <-ticker.C()
	if epoch != 2 {
		t.Fatalf("Expected %d, got %d", 2, epoch)
	}
}

func TestEpochTickerGenesis(t *testing.T) {
	ticker := &EpochTicker{
		c:    make(chan types.Epoch),
		done: make(chan struct{}),
	}
	defer ticker.Done()

	var sinceDuration time.Duration
	since := func(time.Time) time.Duration {
		return sinceDuration
	}

	var untilDuration time.Duration
	until := func(time.Time) time.Duration {
		return untilDuration
	}

	var tick chan time.Time
	after := func(time.Duration) <-chan time.Time {
		return tick
	}

	genesisTime := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	secondsPerEpoch := uint64(8)

	// Test when the ticker starts before genesis time.
	sinceDuration = -1 * time.Second
	untilDuration = 1 * time.Second
	// Make this a buffered channel to prevent a deadlock since
	// the other goroutine calls a function in this goroutine.
	tick = make(chan time.Time, 2)
	ticker.start(genesisTime, secondsPerEpoch, since, until, after)

	// Tick once.
	tick <- time.Now()
	epoch := <-ticker.C()
	if epoch != 0 {
		t.Fatalf("Expected %d, got %d", 0, epoch)
	}

	// Tick twice.
	tick <- time.Now()
	epoch = <-ticker.C()
	if epoch != 1 {
		t.Fatalf("Expected %d, got %d", 1, epoch)
	}
}

package cache

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"k8s.io/client-go/tools/cache"
)

var (
	// Delay parameters
	minDelay    = float64(10)        // 10 nanoseconds
	maxDelay    = float64(100000000) // 0.1 second
	delayFactor = 1.1

	// Metrics
	attestationCacheMiss = promauto.NewCounter(prometheus.CounterOpts{
		Name: "attestation_cache_miss",
		Help: "The number of attestation data requests that aren't present in the cache.",
	})
	attestationCacheHit = promauto.NewCounter(prometheus.CounterOpts{
		Name: "attestation_cache_hit",
		Help: "The number of attestation data requests that are present in the cache.",
	})
	attestationCacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "attestation_cache_size",
		Help: "The number of attestation data in the attestations cache",
	})
)

// AttestationCache is used to store the cached results of an AttestationData request.
type AttestationCache struct {
	cache      *cache.FIFO
	lock       sync.RWMutex
	inProgress map[string]bool
}

// NewAttestationCache initializes the map and underlying cache.
func NewAttestationCache() *AttestationCache {
	return &AttestationCache{
		cache:      cache.NewFIFO(wrapperToKey),
		inProgress: make(map[string]bool),
	}
}

// Get waits for any in progress calculation to complete before returning a
// cached response, if any.
func (c *AttestationCache) Get(ctx context.Context, req *pb.AttestationDataRequest) (*pb.AttestationDataResponse, error) {
	if req == nil {
		return nil, errors.New("nil attestation data request")
	}

	s, e := reqToKey(req)
	if e != nil {
		return nil, e
	}

	delay := minDelay

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		c.lock.RLock()
		if !c.inProgress[s] {
			c.lock.RUnlock()
			break
		}
		c.lock.RUnlock()

		time.Sleep(time.Duration(delay) * time.Nanosecond)
		delay *= delayFactor
		delay = math.Min(delay, maxDelay)
	}

	item, exists, err := c.cache.GetByKey(s)
	if err != nil {
		return nil, err
	}

	if exists && item != nil && item.(*attestationReqResWrapper).res != nil {
		attestationCacheHit.Inc()
		return item.(*attestationReqResWrapper).res, nil
	}
	attestationCacheMiss.Inc()
	return nil, nil
}

// MarkInProgress a request so that any other similar requests will block on
// Get until MarkNotInProgress is called.
func (c *AttestationCache) MarkInProgress(req *pb.AttestationDataRequest) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	s, e := reqToKey(req)
	if e != nil {
		return e
	}
	if c.inProgress[s] {
		return errors.New("already in progress")
	}
	c.inProgress[s] = true
	return nil
}

// MarkNotInProgress will release the lock on a given request. This should be
// called after put.
func (c *AttestationCache) MarkNotInProgress(req *pb.AttestationDataRequest) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	s, e := reqToKey(req)
	if e != nil {
		return e
	}
	delete(c.inProgress, s)
	return nil
}

// Put the response in the cache.
func (c *AttestationCache) Put(ctx context.Context, req *pb.AttestationDataRequest, res *pb.AttestationDataResponse) error {
	data := &attestationReqResWrapper{
		req,
		res,
	}
	if err := c.cache.AddIfNotPresent(data); err != nil {
		return err
	}
	trim(c.cache, maxCacheSize)

	attestationCacheSize.Set(float64(len(c.cache.List())))
	return nil
}

func wrapperToKey(i interface{}) (string, error) {
	w := i.(*attestationReqResWrapper)
	if w == nil {
		return "", errors.New("nil wrapper")
	}
	if w.req == nil {
		return "", errors.New("nil wrapper.request")
	}
	return reqToKey(w.req)
}

func reqToKey(req *pb.AttestationDataRequest) (string, error) {
	return fmt.Sprintf("%d-%d", req.Shard, req.Slot), nil
}

type attestationReqResWrapper struct {
	req *pb.AttestationDataRequest
	res *pb.AttestationDataResponse
}

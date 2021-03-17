package slasher

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/feed"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func TestMain(m *testing.M) {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)

	m.Run()
}

func TestService_StartStop_ChainStartEvent(t *testing.T) {
	hook := logTest.NewGlobal()
	srv, err := New(context.Background(), &ServiceConfig{
		IndexedAttestationsFeed: new(event.Feed),
		StateNotifier:           &mock.MockStateNotifier{},
	})
	require.NoError(t, err)
	go srv.Start()
	time.Sleep(time.Second)
	srv.serviceCfg.StateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.ChainStarted,
		Data: &statefeed.ChainStartedData{StartTime: time.Now()},
	})
	time.Sleep(time.Second)
	srv.slotTicker = &slotutil.SlotTicker{}
	require.NoError(t, srv.Stop())
	require.NoError(t, srv.Status())
	require.LogsContain(t, hook, "received chain start event")
}

func TestService_StartStop_ChainAlreadyInitialized(t *testing.T) {
	hook := logTest.NewGlobal()
	srv, err := New(context.Background(), &ServiceConfig{
		IndexedAttestationsFeed: new(event.Feed),
		StateNotifier:           &mock.MockStateNotifier{},
	})
	require.NoError(t, err)
	go srv.Start()
	time.Sleep(time.Millisecond * 100)
	srv.serviceCfg.StateNotifier.StateFeed().Send(&feed.Event{
		Type: statefeed.Initialized,
		Data: &statefeed.InitializedData{StartTime: time.Now()},
	})
	srv.slotTicker = &slotutil.SlotTicker{}
	require.NoError(t, srv.Stop())
	require.NoError(t, srv.Status())
	require.LogsContain(t, hook, "chain already initialized")
}

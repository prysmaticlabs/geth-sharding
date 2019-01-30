package operations

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/prysmaticlabs/prysm/beacon-chain/internal"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func TestStop(t *testing.T) {
	hook := logTest.NewGlobal()
	opsService := NewOperationService(context.Background(), &Config{})

	if err := opsService.Stop(); err != nil {
		t.Fatalf("Unable to stop operation service: %v", err)
	}

	msg := hook.LastEntry().Message
	want := "Stopping service"
	if msg != want {
		t.Errorf("incorrect log, expected %s, got %s", want, msg)
	}

	// The context should have been canceled.
	if opsService.ctx.Err() != context.Canceled {
		t.Error("context was not canceled")
	}
	hook.Reset()
}

func TestErrorStatus_Ok(t *testing.T) {
	service := NewOperationService(context.Background(), &Config{})
	if service.Status() != nil {
		t.Errorf("service status should be nil to begin with, got: %v", service.errors)
	}
	errs := []error{errors.New("i"), errors.New("have"), errors.New("failed")}
	service.errors = errs

	if !reflect.DeepEqual(service.Status(), errs) {
		t.Error("service status did not return wanted errors")
	}
}

func TestIncomingExits_Ok(t *testing.T) {
	hook := logTest.NewGlobal()
	beaconDB := internal.SetupDB(t)
	defer internal.TeardownDB(t, beaconDB)
	service := NewOperationService(context.Background(), &Config{BeaconDB: beaconDB})

	exitRoutine := make(chan bool)
	go func() {
		service.saveOperations()
		<-exitRoutine
	}()
	exit := &pb.Exit{Slot: 100}
	hash, err := hashutil.HashProto(exit)
	if err != nil {
		t.Fatalf("Could not hash exit proto: %v", err)
	}

	service.incomingValidatorExit <- exit
	service.cancel()
	exitRoutine <- true

	want := fmt.Sprintf("Exit request %#x saved in db", hash)
	testutil.AssertLogsContain(t, hook, want)
}

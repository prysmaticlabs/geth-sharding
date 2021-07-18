package beacon

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1"
	"github.com/prysmaticlabs/prysm/shared/p2putils"
	"github.com/prysmaticlabs/prysm/shared/params"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GetForkSchedule retrieve all scheduled upcoming forks this node is aware of.
func (bs *Server) GetForkSchedule(ctx context.Context, _ *emptypb.Empty) (*ethpb.ForkScheduleResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.GetForkSchedule")
	defer span.End()

	schedule := params.BeaconConfig().ForkVersionSchedule
	if len(schedule) == 0 {
		return &ethpb.ForkScheduleResponse{
			Data: make([]*ethpb.Fork, 0),
		}, nil
	}

	versions := p2putils.SortedForkVersions(schedule)
	forks := make([]*ethpb.Fork, len(schedule))
	var previous, current []byte
	for i, v := range versions {
		if i == 0 {
			previous = params.BeaconConfig().GenesisForkVersion
		} else {
			previous = current
		}
		copyV := v
		current = copyV[:]
		forks[i] = &ethpb.Fork{
			PreviousVersion: previous,
			CurrentVersion:  current,
			Epoch:           schedule[v],
		}
	}

	return &ethpb.ForkScheduleResponse{
		Data: forks,
	}, nil
}

// GetSpec retrieves specification configuration (without Phase 1 params) used on this node. Specification params list
// Values are returned with following format:
// - any value starting with 0x in the spec is returned as a hex string.
// - all other values are returned as number.
func (bs *Server) GetSpec(ctx context.Context, _ *emptypb.Empty) (*ethpb.SpecResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beaconV1.GetSpec")
	defer span.End()

	data, err := prepareConfigSpec()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to prepare spec data: %v", err)
	}
	return &ethpb.SpecResponse{Data: data}, nil
}

// GetDepositContract retrieves deposit contract address and genesis fork version.
func (bs *Server) GetDepositContract(ctx context.Context, _ *emptypb.Empty) (*ethpb.DepositContractResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.GetDepositContract")
	defer span.End()

	return &ethpb.DepositContractResponse{
		Data: &ethpb.DepositContract{
			ChainId: params.BeaconConfig().DepositChainID,
			Address: params.BeaconConfig().DepositContractAddress,
		},
	}, nil
}

func prepareConfigSpec() (map[string]string, error) {
	data := make(map[string]string)
	config := *params.BeaconConfig()
	t := reflect.TypeOf(config)
	v := reflect.ValueOf(config)

	for i := 0; i < t.NumField(); i++ {
		tField := t.Field(i)
		_, isSpecField := tField.Tag.Lookup("spec")
		if !isSpecField {
			// Field should not be returned from API.
			continue
		}

		tagValue := strings.ToUpper(tField.Tag.Get("yaml"))
		vField := v.Field(i)
		switch vField.Kind() {
		case reflect.Int:
			data[tagValue] = strconv.FormatInt(vField.Int(), 10)
		case reflect.Uint64:
			data[tagValue] = strconv.FormatUint(vField.Uint(), 10)
		case reflect.Slice:
			data[tagValue] = hexutil.Encode(vField.Bytes())
		case reflect.Array:
			data[tagValue] = hexutil.Encode(reflect.ValueOf(&config).Elem().Field(i).Slice(0, vField.Len()).Bytes())
		case reflect.String:
			data[tagValue] = vField.String()
		case reflect.Uint8:
			data[tagValue] = hexutil.Encode([]byte{uint8(vField.Uint())})
		default:
			return nil, fmt.Errorf("unsupported config field type: %s", vField.Kind().String())
		}
	}

	return data, nil
}

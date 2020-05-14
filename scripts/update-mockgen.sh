#!/bin/bash

# Script to update mock files after proto/beacon/rpc/v1/services.proto changes.
# Use a space to separate mock destination from its interfaces.

mock_path="$GOPATH/src/github.com/prysmaticlabs/prysm/shared/mock"
mocks=(
      "$mock_path/beacon_service_mock.go BeaconChainClient,BeaconNodeValidatorClient"
      "$mock_path/node_service_mock.go NodeClient"
)

for ((i = 0; i < ${#mocks[@]}; i++)); do
    file=${mocks[i]% *};
    interfaces=${mocks[i]#* };
    echo "generating $file for interfaces: $interfaces";
    go run github.com/golang/mock/mockgen -package=mock -destination=$file github.com/prysmaticlabs/ethereumapis/eth/v1alpha1 $interfaces
done

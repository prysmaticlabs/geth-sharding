#!/bin/bash

# Script to update mock files after proto/beacon/rpc/v1/services.proto changes.
# Use a space to separate mock destination from its interfaces.

mock_path="shared/mock"
mocks=(
      "$mock_path/beacon_service_mock.go BeaconChainClient,BeaconChain_StreamChainHeadClient,BeaconChain_StreamAttestationsClient,BeaconChain_StreamBlocksClient,BeaconChain_StreamValidatorsInfoClient,BeaconChain_StreamIndexedAttestationsClient"
      "$mock_path/beacon_chain_service_mock.go BeaconChain_StreamChainHeadServer,BeaconChain_StreamAttestationsServer,BeaconChain_StreamBlocksServer,BeaconChain_StreamValidatorsInfoServer,BeaconChain_StreamIndexedAttestationsServer"
      "$mock_path/beacon_chain_altair_service_mock.go BeaconChainAltairServer"
      "$mock_path/beacon_chain_altair_client_mock.go BeaconChainAltairClient"
      "$mock_path/beacon_validator_server_mock.go BeaconNodeValidatorServer,BeaconNodeValidator_WaitForActivationServer,BeaconNodeValidator_WaitForChainStartServer,BeaconNodeValidator_StreamDutiesServer"
      "$mock_path/beacon_validator_client_mock.go BeaconNodeValidatorClient,BeaconNodeValidator_WaitForChainStartClient,BeaconNodeValidator_WaitForActivationClient,BeaconNodeValidator_StreamDutiesClient"
      "$mock_path/beacon_altair_validator_server_mock.go BeaconNodeValidatorAltair_StreamBlocksServer"
      "$mock_path/beacon_altair_validator_client_mock.go BeaconNodeValidatorAltair_StreamBlocksClient"
      "$mock_path/event_service_mock.go EventsClient,Events_StreamEventsClient,Events_StreamEventsServer"
      "$mock_path/node_service_mock.go NodeClient"
      "$mock_path/keymanager_mock.go RemoteSignerClient"
)

for ((i = 0; i < ${#mocks[@]}; i++)); do
    file=${mocks[i]% *};
    interfaces=${mocks[i]#* };
    echo "generating $file for interfaces: $interfaces";
    GO11MODULE=on mockgen -package=mock -destination="$file" github.com/prysmaticlabs/prysm/proto/eth/v1alpha1 "$interfaces"
    GO11MODULE=on mockgen -package=mock -destination="$file" github.com/prysmaticlabs/prysm/proto/validator/accounts/v2 "$interfaces"
    GO11MODULE=on mockgen -package=mock -destination="$file" github.com/prysmaticlabs/prysm/proto/prysm/v2 "$interfaces"
done

goimports -w "$mock_path/."
gofmt -s -w "$mock_path/."
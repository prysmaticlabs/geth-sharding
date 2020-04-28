package main

import (
	"flag"
	"io/ioutil"
	"log"

	"github.com/prysmaticlabs/go-ssz"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/roughtime"
)

var (
	genesisTime = flag.Uint64("genesis-time", 0, "Unix timestamp used as the genesis time in the generated genesis state (defaults to now)")
	inputSSZ    = flag.String("input-ssz", "", "Output filename of the SSZ marshaling of the generated genesis state")
)

func main() {
	flag.Parse()
	if *inputSSZ == "" {
		log.Fatal("Expected --input-ssz")
	}

	beaconState := &pb.BeaconState{}
	if err := unmarshalFile(*inputSSZ, beaconState); err != nil {
		log.Fatal(err)
	}
	if *genesisTime == 0 {
		log.Print("No --genesis-time specified, defaulting to now")
		beaconState.GenesisTime = uint64(roughtime.Now().Unix())
	} else {
		beaconState.GenesisTime = *genesisTime
	}

	encodedState, err := ssz.Marshal(beaconState)
	if err != nil {
		log.Fatalf("Could not ssz marshal the beacon state: %v", err)
	}
	if err := ioutil.WriteFile(*inputSSZ, encodedState, 0644); err != nil {
		log.Fatalf("Could not write encoded beacon state to file: %v", err)
	}
	log.Printf("Done writing to %s", *inputSSZ)
}

func unmarshalFile(fPath string, data interface{}) error {
	rawFile, err := ioutil.ReadFile(fPath)
	if err != nil {
		return err
	}
	return ssz.Unmarshal(rawFile, data)
}

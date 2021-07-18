package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	fssz "github.com/ferranbt/fastssz"
	"github.com/kr/pretty"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/sszutil"
	"github.com/prysmaticlabs/prysm/shared/version"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"gopkg.in/d4l3k/messagediff.v1"
)

func main() {
	var blockPath string
	var preStatePath string
	var expectedPostStatePath string
	var sszPath string
	var sszType string

	customFormatter := new(prefixed.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
	app := cli.App{}
	app.Name = "pcli"
	app.Usage = "A command line utility to run Ethereum consensus specific commands"
	app.Version = version.Version()
	app.Commands = []*cli.Command{
		{
			Name:    "pretty",
			Aliases: []string{"p"},
			Usage:   "pretty-print SSZ data",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "ssz-path",
					Usage:       "Path to file(ssz)",
					Required:    true,
					Destination: &sszPath,
				},
				&cli.StringFlag{
					Name: "data-type",
					Usage: "ssz file data type: " +
						"block|" +
						"signed_block|" +
						"attestation|" +
						"block_header|" +
						"deposit|" +
						"proposer_slashing|" +
						"signed_block_header|" +
						"signed_voluntary_exit|" +
						"voluntary_exit|" +
						"state",
					Required:    true,
					Destination: &sszType,
				},
			},
			Action: func(c *cli.Context) error {
				var data fssz.Unmarshaler
				switch sszType {
				case "block":
					data = &ethpb.BeaconBlock{}
				case "signed_block":
					data = &ethpb.SignedBeaconBlock{}
				case "attestation":
					data = &ethpb.Attestation{}
				case "block_header":
					data = &ethpb.BeaconBlockHeader{}
				case "deposit":
					data = &ethpb.Deposit{}
				case "deposit_message":
					data = &pb.DepositMessage{}
				case "proposer_slashing":
					data = &ethpb.ProposerSlashing{}
				case "signed_block_header":
					data = &ethpb.SignedBeaconBlockHeader{}
				case "signed_voluntary_exit":
					data = &ethpb.SignedVoluntaryExit{}
				case "voluntary_exit":
					data = &ethpb.VoluntaryExit{}
				case "state":
					data = &pb.BeaconState{}
				default:
					log.Fatal("Invalid type")
				}
				prettyPrint(sszPath, data)
				return nil
			},
		},
		{
			Name:     "state-transition",
			Category: "state-transition",
			Usage:    "Subcommand to run manual state transitions",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "block-path",
					Usage:       "Path to block file(ssz)",
					Destination: &blockPath,
				},
				&cli.StringFlag{
					Name:        "pre-state-path",
					Usage:       "Path to pre state file(ssz)",
					Destination: &preStatePath,
				},
				&cli.StringFlag{
					Name:        "expected-post-state-path",
					Usage:       "Path to expected post state file(ssz)",
					Destination: &expectedPostStatePath,
				},
			},
			Action: func(c *cli.Context) error {
				if blockPath == "" {
					log.Info("Block path not provided for state transition. " +
						"Please provide path")
					reader := bufio.NewReader(os.Stdin)
					text, err := reader.ReadString('\n')
					if err != nil {
						log.Fatal(err)
					}
					if text = strings.ReplaceAll(text, "\n", ""); text == "" {
						log.Fatal("Empty block path given")
					}
					blockPath = text
				}
				block := &ethpb.SignedBeaconBlock{}
				if err := dataFetcher(blockPath, block); err != nil {
					log.Fatal(err)
				}
				blkRoot, err := block.Block.HashTreeRoot()
				if err != nil {
					log.Fatal(err)
				}
				if preStatePath == "" {
					log.Info("Pre State path not provided for state transition. " +
						"Please provide path")
					reader := bufio.NewReader(os.Stdin)
					text, err := reader.ReadString('\n')
					if err != nil {
						log.Fatal(err)
					}
					if text = strings.ReplaceAll(text, "\n", ""); text == "" {
						log.Fatal("Empty state path given")
					}
					preStatePath = text
				}
				preState := &pb.BeaconState{}
				if err := dataFetcher(preStatePath, preState); err != nil {
					log.Fatal(err)
				}
				stateObj, err := v1.InitializeFromProto(preState)
				if err != nil {
					log.Fatal(err)
				}
				preStateRoot, err := stateObj.HashTreeRoot(context.Background())
				if err != nil {
					log.Fatal(err)
				}
				log.WithFields(log.Fields{
					"blockSlot":    fmt.Sprintf("%d", block.Block.Slot),
					"preStateSlot": fmt.Sprintf("%d", stateObj.Slot()),
				}).Infof(
					"Performing state transition with a block root of %#x and pre state root of %#x",
					blkRoot,
					preStateRoot,
				)
				postState, err := state.ExecuteStateTransition(context.Background(), stateObj, wrapper.WrappedPhase0SignedBeaconBlock(block))
				if err != nil {
					log.Fatal(err)
				}
				postRoot, err := postState.HashTreeRoot(context.Background())
				if err != nil {
					log.Fatal(err)
				}
				log.Infof("Finished state transition with post state root of %#x", postRoot)

				// Diff the state if a post state is provided.
				if expectedPostStatePath != "" {
					expectedState := &pb.BeaconState{}
					if err := dataFetcher(expectedPostStatePath, expectedState); err != nil {
						log.Fatal(err)
					}
					if !sszutil.DeepEqual(expectedState, postState.InnerStateUnsafe()) {
						diff, _ := messagediff.PrettyDiff(expectedState, postState.InnerStateUnsafe())
						log.Errorf("Derived state differs from provided post state: %s", diff)
					}
				}
				return nil
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

// dataFetcher fetches and unmarshals data from file to provided data structure.
func dataFetcher(fPath string, data fssz.Unmarshaler) error {
	rawFile, err := ioutil.ReadFile(fPath)
	if err != nil {
		return err
	}
	return data.UnmarshalSSZ(rawFile)
}

func prettyPrint(sszPath string, data fssz.Unmarshaler) {
	if err := dataFetcher(sszPath, data); err != nil {
		log.Fatal(err)
	}
	str := pretty.Sprint(data)
	re := regexp.MustCompile("(?m)[\r\n]+^.*XXX_.*$")
	str = re.ReplaceAllString(str, "")
	fmt.Print(str)
}

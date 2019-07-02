// Code generated by yaml_to_go. DO NOT EDIT.
// source: ssz_minimal_one.yaml

package testing

type SszMinimalTest struct {
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	ForksTimeline string   `json:"forks_timeline"`
	Forks         []string `json:"forks"`
	Config        string   `json:"config"`
	Runner        string   `json:"runner"`
	Handler       string   `json:"handler"`
	TestCases     []struct {
		Attestation struct {
			Value struct {
				AggregationBits []byte `json:"aggregation_bitfield"`
				Data            struct {
					BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
					SourceEpoch     uint64 `json:"source_epoch"`
					SourceRoot      []byte `json:"source_root" ssz:"size=32"`
					TargetEpoch     uint64 `json:"target_epoch"`
					TargetRoot      []byte `json:"target_root" ssz:"size=32"`
					Crosslink       struct {
						Shard      uint64 `json:"shard"`
						StartEpoch uint64 `json:"start_epoch"`
						EndEpoch   uint64 `json:"end_epoch"`
						ParentRoot []byte `json:"parent_root" ssz:"size=32"`
						DataRoot   []byte `json:"data_root" ssz:"size=32"`
					} `json:"crosslink"`
				} `json:"data"`
				CustodyBits []byte `json:"custody_bitfield"`
				Signature   []byte `json:"signature" ssz:"size=96"`
			} `json:"value"`
			Serialized  []byte `json:"serialized"`
			Root        []byte `json:"root" ssz:"size=32"`
			SigningRoot []byte `json:"signing_root" ssz:"size=32"`
		} `json:"Attestation,omitempty"`
		AttestationData struct {
			Value struct {
				BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
				SourceEpoch     uint64 `json:"source_epoch"`
				SourceRoot      []byte `json:"source_root" ssz:"size=32"`
				TargetEpoch     uint64 `json:"target_epoch"`
				TargetRoot      []byte `json:"target_root" ssz:"size=32"`
				Crosslink       struct {
					Shard      uint64 `json:"shard"`
					StartEpoch uint64 `json:"start_epoch"`
					EndEpoch   uint64 `json:"end_epoch"`
					ParentRoot []byte `json:"parent_root" ssz:"size=32"`
					DataRoot   []byte `json:"data_root" ssz:"size=32"`
				} `json:"crosslink"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"AttestationData,omitempty"`
		AttestationDataAndCustodyBit struct {
			Value struct {
				Data struct {
					BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
					SourceEpoch     uint64 `json:"source_epoch"`
					SourceRoot      []byte `json:"source_root" ssz:"size=32"`
					TargetEpoch     uint64 `json:"target_epoch"`
					TargetRoot      []byte `json:"target_root" ssz:"size=32"`
					Crosslink       struct {
						Shard      uint64 `json:"shard"`
						StartEpoch uint64 `json:"start_epoch"`
						EndEpoch   uint64 `json:"end_epoch"`
						ParentRoot []byte `json:"parent_root" ssz:"size=32"`
						DataRoot   []byte `json:"data_root" ssz:"size=32"`
					} `json:"crosslink"`
				} `json:"data"`
				CustodyBit bool `json:"custody_bit"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"AttestationDataAndCustodyBit,omitempty"`
		AttesterSlashing struct {
			Value struct {
				Attestation1 struct {
					CustodyBit0Indices []uint64 `json:"custody_bit_0_indices"`
					CustodyBit1Indices []uint64 `json:"custody_bit_1_indices"`
					Data               struct {
						BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
						SourceEpoch     uint64 `json:"source_epoch"`
						SourceRoot      []byte `json:"source_root" ssz:"size=32"`
						TargetEpoch     uint64 `json:"target_epoch"`
						TargetRoot      []byte `json:"target_root" ssz:"size=32"`
						Crosslink       struct {
							Shard      uint64 `json:"shard"`
							StartEpoch uint64 `json:"start_epoch"`
							EndEpoch   uint64 `json:"end_epoch"`
							ParentRoot []byte `json:"parent_root" ssz:"size=32"`
							DataRoot   []byte `json:"data_root" ssz:"size=32"`
						} `json:"crosslink"`
					} `json:"data"`
					Signature []byte `json:"signature" ssz:"size=96"`
				} `json:"attestation_1"`
				Attestation2 struct {
					CustodyBit0Indices []uint64 `json:"custody_bit_0_indices"`
					CustodyBit1Indices []uint64 `json:"custody_bit_1_indices"`
					Data               struct {
						BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
						SourceEpoch     uint64 `json:"source_epoch"`
						SourceRoot      []byte `json:"source_root" ssz:"size=32"`
						TargetEpoch     uint64 `json:"target_epoch"`
						TargetRoot      []byte `json:"target_root" ssz:"size=32"`
						Crosslink       struct {
							Shard      uint64 `json:"shard"`
							StartEpoch uint64 `json:"start_epoch"`
							EndEpoch   uint64 `json:"end_epoch"`
							ParentRoot []byte `json:"parent_root" ssz:"size=32"`
							DataRoot   []byte `json:"data_root" ssz:"size=32"`
						} `json:"crosslink"`
					} `json:"data"`
					Signature []byte `json:"signature" ssz:"size=96"`
				} `json:"attestation_2"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"AttesterSlashing,omitempty"`
		BeaconBlock struct {
			Value struct {
				Slot       uint64 `json:"slot"`
				ParentRoot []byte `json:"parent_root" ssz:"size=32"`
				StateRoot  []byte `json:"state_root" ssz:"size=32"`
				Body       struct {
					RandaoReveal []byte `json:"randao_reveal" ssz:"size=96"`
					Eth1Data     struct {
						DepositRoot  []byte `json:"deposit_root" ssz:"size=32"`
						DepositCount uint64 `json:"deposit_count"`
						BlockHash    []byte `json:"block_hash" ssz:"size=32"`
					} `json:"eth1_data"`
					Graffiti          []byte `json:"graffiti" ssz:"size=32"`
					ProposerSlashings []struct {
						ProposerIndex uint64 `json:"proposer_index"`
						Header1       struct {
							Slot       uint64 `json:"slot"`
							ParentRoot []byte `json:"parent_root" ssz:"size=32"`
							StateRoot  []byte `json:"state_root" ssz:"size=32"`
							BodyRoot   []byte `json:"body_root" ssz:"size=32"`
							Signature  []byte `json:"signature" ssz:"size=96"`
						} `json:"header_1"`
						Header2 struct {
							Slot       uint64 `json:"slot"`
							ParentRoot []byte `json:"parent_root" ssz:"size=32"`
							StateRoot  []byte `json:"state_root" ssz:"size=32"`
							BodyRoot   []byte `json:"body_root" ssz:"size=32"`
							Signature  []byte `json:"signature" ssz:"size=96"`
						} `json:"header_2"`
					} `json:"proposer_slashings"`
					AttesterSlashings []struct {
						Attestation1 struct {
							CustodyBit0Indices []uint64 `json:"custody_bit_0_indices"`
							CustodyBit1Indices []uint64 `json:"custody_bit_1_indices"`
							Data               struct {
								BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
								SourceEpoch     uint64 `json:"source_epoch"`
								SourceRoot      []byte `json:"source_root" ssz:"size=32"`
								TargetEpoch     uint64 `json:"target_epoch"`
								TargetRoot      []byte `json:"target_root" ssz:"size=32"`
								Crosslink       struct {
									Shard      uint64 `json:"shard"`
									StartEpoch uint64 `json:"start_epoch"`
									EndEpoch   uint64 `json:"end_epoch"`
									ParentRoot []byte `json:"parent_root" ssz:"size=32"`
									DataRoot   []byte `json:"data_root" ssz:"size=32"`
								} `json:"crosslink"`
							} `json:"data"`
							Signature []byte `json:"signature" ssz:"size=96"`
						} `json:"attestation_1"`
						Attestation2 struct {
							CustodyBit0Indices []uint64 `json:"custody_bit_0_indices"`
							CustodyBit1Indices []uint64 `json:"custody_bit_1_indices"`
							Data               struct {
								BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
								SourceEpoch     uint64 `json:"source_epoch"`
								SourceRoot      []byte `json:"source_root" ssz:"size=32"`
								TargetEpoch     uint64 `json:"target_epoch"`
								TargetRoot      []byte `json:"target_root" ssz:"size=32"`
								Crosslink       struct {
									Shard      uint64 `json:"shard"`
									StartEpoch uint64 `json:"start_epoch"`
									EndEpoch   uint64 `json:"end_epoch"`
									ParentRoot []byte `json:"parent_root" ssz:"size=32"`
									DataRoot   []byte `json:"data_root" ssz:"size=32"`
								} `json:"crosslink"`
							} `json:"data"`
							Signature []byte `json:"signature" ssz:"size=96"`
						} `json:"attestation_2"`
					} `json:"attester_slashings"`
					Attestations []struct {
						AggregationBits []byte `json:"aggregation_bitfield"`
						Data            struct {
							BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
							SourceEpoch     uint64 `json:"source_epoch"`
							SourceRoot      []byte `json:"source_root" ssz:"size=32"`
							TargetEpoch     uint64 `json:"target_epoch"`
							TargetRoot      []byte `json:"target_root" ssz:"size=32"`
							Crosslink       struct {
								Shard      uint64 `json:"shard"`
								StartEpoch uint64 `json:"start_epoch"`
								EndEpoch   uint64 `json:"end_epoch"`
								ParentRoot []byte `json:"parent_root" ssz:"size=32"`
								DataRoot   []byte `json:"data_root" ssz:"size=32"`
							} `json:"crosslink"`
						} `json:"data"`
						CustodyBits []byte `json:"custody_bitfield"`
						Signature   []byte `json:"signature" ssz:"size=96"`
					} `json:"attestations"`
					Deposits []struct {
						Proof [][]byte `json:"proof" ssz:"size=32,32"`
						Data  struct {
							Pubkey                []byte `json:"pubkey" ssz:"size=48"`
							WithdrawalCredentials []byte `json:"withdrawal_credentials" ssz:"size=32"`
							Amount                uint64 `json:"amount"`
							Signature             []byte `json:"signature" ssz:"size=96"`
						} `json:"data"`
					} `json:"deposits"`
					VoluntaryExits []struct {
						Epoch          uint64 `json:"epoch"`
						ValidatorIndex uint64 `json:"validator_index"`
						Signature      []byte `json:"signature" ssz:"size=96"`
					} `json:"voluntary_exits"`
					Transfers []struct {
						Sender    uint64 `json:"sender"`
						Recipient uint64 `json:"recipient"`
						Amount    uint64 `json:"amount"`
						Fee       uint64 `json:"fee"`
						Slot      uint64 `json:"slot"`
						Pubkey    []byte `json:"pubkey" ssz:"size=48"`
						Signature []byte `json:"signature" ssz:"size=96"`
					} `json:"transfers"`
				} `json:"body"`
				Signature []byte `json:"signature" ssz:"size=96"`
			} `json:"value"`
			Serialized  []byte `json:"serialized"`
			Root        []byte `json:"root" ssz:"size=32"`
			SigningRoot []byte `json:"signing_root" ssz:"size=32"`
		} `json:"BeaconBlock,omitempty"`
		BeaconBlockBody struct {
			Value struct {
				RandaoReveal []byte `json:"randao_reveal" ssz:"size=96"`
				Eth1Data     struct {
					DepositRoot  []byte `json:"deposit_root" ssz:"size=32"`
					DepositCount uint64 `json:"deposit_count"`
					BlockHash    []byte `json:"block_hash" ssz:"size=32"`
				} `json:"eth1_data"`
				Graffiti          []byte `json:"graffiti" ssz:"size=32"`
				ProposerSlashings []struct {
					ProposerIndex uint64 `json:"proposer_index"`
					Header1       struct {
						Slot       uint64 `json:"slot"`
						ParentRoot []byte `json:"parent_root" ssz:"size=32"`
						StateRoot  []byte `json:"state_root" ssz:"size=32"`
						BodyRoot   []byte `json:"body_root" ssz:"size=32"`
						Signature  []byte `json:"signature" ssz:"size=96"`
					} `json:"header_1"`
					Header2 struct {
						Slot       uint64 `json:"slot"`
						ParentRoot []byte `json:"parent_root" ssz:"size=32"`
						StateRoot  []byte `json:"state_root" ssz:"size=32"`
						BodyRoot   []byte `json:"body_root" ssz:"size=32"`
						Signature  []byte `json:"signature" ssz:"size=96"`
					} `json:"header_2"`
				} `json:"proposer_slashings"`
				AttesterSlashings []struct {
					Attestation1 struct {
						CustodyBit0Indices []uint64 `json:"custody_bit_0_indices"`
						CustodyBit1Indices []uint64 `json:"custody_bit_1_indices"`
						Data               struct {
							BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
							SourceEpoch     uint64 `json:"source_epoch"`
							SourceRoot      []byte `json:"source_root" ssz:"size=32"`
							TargetEpoch     uint64 `json:"target_epoch"`
							TargetRoot      []byte `json:"target_root" ssz:"size=32"`
							Crosslink       struct {
								Shard      uint64 `json:"shard"`
								StartEpoch uint64 `json:"start_epoch"`
								EndEpoch   uint64 `json:"end_epoch"`
								ParentRoot []byte `json:"parent_root" ssz:"size=32"`
								DataRoot   []byte `json:"data_root" ssz:"size=32"`
							} `json:"crosslink"`
						} `json:"data"`
						Signature []byte `json:"signature" ssz:"size=96"`
					} `json:"attestation_1"`
					Attestation2 struct {
						CustodyBit0Indices []uint64 `json:"custody_bit_0_indices"`
						CustodyBit1Indices []uint64 `json:"custody_bit_1_indices"`
						Data               struct {
							BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
							SourceEpoch     uint64 `json:"source_epoch"`
							SourceRoot      []byte `json:"source_root" ssz:"size=32"`
							TargetEpoch     uint64 `json:"target_epoch"`
							TargetRoot      []byte `json:"target_root" ssz:"size=32"`
							Crosslink       struct {
								Shard      uint64 `json:"shard"`
								StartEpoch uint64 `json:"start_epoch"`
								EndEpoch   uint64 `json:"end_epoch"`
								ParentRoot []byte `json:"parent_root" ssz:"size=32"`
								DataRoot   []byte `json:"data_root" ssz:"size=32"`
							} `json:"crosslink"`
						} `json:"data"`
						Signature []byte `json:"signature" ssz:"size=96"`
					} `json:"attestation_2"`
				} `json:"attester_slashings"`
				Attestations []struct {
					AggregationBits []byte `json:"aggregation_bitfield"`
					Data            struct {
						BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
						SourceEpoch     uint64 `json:"source_epoch"`
						SourceRoot      []byte `json:"source_root" ssz:"size=32"`
						TargetEpoch     uint64 `json:"target_epoch"`
						TargetRoot      []byte `json:"target_root" ssz:"size=32"`
						Crosslink       struct {
							Shard      uint64 `json:"shard"`
							StartEpoch uint64 `json:"start_epoch"`
							EndEpoch   uint64 `json:"end_epoch"`
							ParentRoot []byte `json:"parent_root" ssz:"size=32"`
							DataRoot   []byte `json:"data_root" ssz:"size=32"`
						} `json:"crosslink"`
					} `json:"data"`
					CustodyBits []byte `json:"custody_bitfield"`
					Signature   []byte `json:"signature" ssz:"size=96"`
				} `json:"attestations"`
				Deposits []struct {
					Proof [][]byte `json:"proof" ssz:"size=32,32"`
					Data  struct {
						Pubkey                []byte `json:"pubkey" ssz:"size=48"`
						WithdrawalCredentials []byte `json:"withdrawal_credentials" ssz:"size=32"`
						Amount                uint64 `json:"amount"`
						Signature             []byte `json:"signature" ssz:"size=96"`
					} `json:"data"`
				} `json:"deposits"`
				VoluntaryExits []struct {
					Epoch          uint64 `json:"epoch"`
					ValidatorIndex uint64 `json:"validator_index"`
					Signature      []byte `json:"signature" ssz:"size=96"`
				} `json:"voluntary_exits"`
				Transfers []struct {
					Sender    uint64 `json:"sender"`
					Recipient uint64 `json:"recipient"`
					Amount    uint64 `json:"amount"`
					Fee       uint64 `json:"fee"`
					Slot      uint64 `json:"slot"`
					Pubkey    []byte `json:"pubkey" ssz:"size=48"`
					Signature []byte `json:"signature" ssz:"size=96"`
				} `json:"transfers"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"BeaconBlockBody,omitempty"`
		BeaconBlockHeader struct {
			Value struct {
				Slot       uint64 `json:"slot"`
				ParentRoot []byte `json:"parent_root" ssz:"size=32"`
				StateRoot  []byte `json:"state_root" ssz:"size=32"`
				BodyRoot   []byte `json:"body_root" ssz:"size=32"`
				Signature  []byte `json:"signature" ssz:"size=96"`
			} `json:"value"`
			Serialized  []byte `json:"serialized"`
			Root        []byte `json:"root" ssz:"size=32"`
			SigningRoot []byte `json:"signing_root" ssz:"size=32"`
		} `json:"BeaconBlockHeader,omitempty"`
		BeaconState struct {
			Value struct {
				Slot        uint64 `json:"slot"`
				GenesisTime uint64 `json:"genesis_time"`
				Fork        struct {
					PreviousVersion []byte `json:"previous_version" ssz:"size=4"`
					CurrentVersion  []byte `json:"current_version" ssz:"size=4"`
					Epoch           uint64 `json:"epoch"`
				} `json:"fork"`
				Validators []struct {
					Pubkey                     []byte `json:"pubkey" ssz:"size=48"`
					WithdrawalCredentials      []byte `json:"withdrawal_credentials" ssz:"size=32"`
					ActivationEligibilityEpoch uint64 `json:"activation_eligibility_epoch"`
					ActivationEpoch            uint64 `json:"activation_epoch"`
					ExitEpoch                  uint64 `json:"exit_epoch"`
					WithdrawableEpoch          uint64 `json:"withdrawable_epoch"`
					Slashed                    bool   `json:"slashed"`
					EffectiveBalance           uint64 `json:"effective_balance"`
				} `json:"validator_registry"`
				Balances                  []uint64 `json:"balances"`
				RandaoMixes               [][]byte `json:"latest_randao_mixes" ssz:"size=64,32"`
				StartShard                uint64   `json:"latest_start_shard"`
				PreviousEpochAttestations []struct {
					AggregationBits []byte `json:"aggregation_bitfield"`
					Data            struct {
						BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
						SourceEpoch     uint64 `json:"source_epoch"`
						SourceRoot      []byte `json:"source_root" ssz:"size=32"`
						TargetEpoch     uint64 `json:"target_epoch"`
						TargetRoot      []byte `json:"target_root" ssz:"size=32"`
						Crosslink       struct {
							Shard      uint64 `json:"shard"`
							StartEpoch uint64 `json:"start_epoch"`
							EndEpoch   uint64 `json:"end_epoch"`
							ParentRoot []byte `json:"parent_root" ssz:"size=32"`
							DataRoot   []byte `json:"data_root" ssz:"size=32"`
						} `json:"crosslink"`
					} `json:"data"`
					InclusionDelay uint64 `json:"inclusion_delay"`
					ProposerIndex  uint64 `json:"proposer_index"`
				} `json:"previous_epoch_attestations"`
				CurrentEpochAttestations []struct {
					AggregationBits []byte `json:"aggregation_bitfield"`
					Data            struct {
						BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
						SourceEpoch     uint64 `json:"source_epoch"`
						SourceRoot      []byte `json:"source_root" ssz:"size=32"`
						TargetEpoch     uint64 `json:"target_epoch"`
						TargetRoot      []byte `json:"target_root" ssz:"size=32"`
						Crosslink       struct {
							Shard      uint64 `json:"shard"`
							StartEpoch uint64 `json:"start_epoch"`
							EndEpoch   uint64 `json:"end_epoch"`
							ParentRoot []byte `json:"parent_root" ssz:"size=32"`
							DataRoot   []byte `json:"data_root" ssz:"size=32"`
						} `json:"crosslink"`
					} `json:"data"`
					InclusionDelay uint64 `json:"inclusion_delay"`
					ProposerIndex  uint64 `json:"proposer_index"`
				} `json:"current_epoch_attestations"`
				PreviousJustifiedEpoch uint64 `json:"previous_justified_epoch"`
				CurrentJustifiedEpoch  uint64 `json:"current_justified_epoch"`
				PreviousJustifiedRoot  []byte `json:"previous_justified_root" ssz:"size=32"`
				CurrentJustifiedRoot   []byte `json:"current_justified_root" ssz:"size=32"`
				JustificationBits      uint64 `json:"justification_bitfield"`
				FinalizedEpoch         uint64 `json:"finalized_epoch"`
				FinalizedRoot          []byte `json:"finalized_root" ssz:"size=32"`
				CurrentCrosslinks      []struct {
					Shard      uint64 `json:"shard"`
					StartEpoch uint64 `json:"start_epoch"`
					EndEpoch   uint64 `json:"end_epoch"`
					ParentRoot []byte `json:"parent_root" ssz:"size=32"`
					DataRoot   []byte `json:"data_root" ssz:"size=32"`
				} `json:"current_crosslinks" ssz:"size=8"`
				PreviousCrosslinks []struct {
					Shard      uint64 `json:"shard"`
					StartEpoch uint64 `json:"start_epoch"`
					EndEpoch   uint64 `json:"end_epoch"`
					ParentRoot []byte `json:"parent_root" ssz:"size=32"`
					DataRoot   []byte `json:"data_root" ssz:"size=32"`
				} `json:"previous_crosslinks" ssz:"size=8"`
				BlockRoots        [][]byte `json:"latest_block_roots" ssz:"size=64,32"`
				StateRoots        [][]byte `json:"latest_state_roots" ssz:"size=64,32"`
				ActiveIndexRoots  [][]byte `json:"latest_active_index_roots" ssz:"size=64,32"`
				Slashings         []uint64 `json:"latest_slashed_balances" ssz:"size=64"`
				LatestBlockHeader struct {
					Slot       uint64 `json:"slot"`
					ParentRoot []byte `json:"parent_root" ssz:"size=32"`
					StateRoot  []byte `json:"state_root" ssz:"size=32"`
					BodyRoot   []byte `json:"body_root" ssz:"size=32"`
					Signature  []byte `json:"signature" ssz:"size=96"`
				} `json:"latest_block_header"`
				HistoricalRoots [][]byte `json:"historical_roots" ssz:"size=?,32"`
				Eth1Data        struct {
					DepositRoot  []byte `json:"deposit_root" ssz:"size=32"`
					DepositCount uint64 `json:"deposit_count"`
					BlockHash    []byte `json:"block_hash" ssz:"size=32"`
				} `json:"latest_eth1_data"`
				Eth1DataVotes []struct {
					DepositRoot  []byte `json:"deposit_root" ssz:"size=32"`
					DepositCount uint64 `json:"deposit_count"`
					BlockHash    []byte `json:"block_hash" ssz:"size=32"`
				} `json:"eth1_data_votes"`
				Eth1DepositIndex uint64 `json:"deposit_index"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"BeaconState,omitempty"`
		Crosslink struct {
			Value struct {
				Shard      uint64 `json:"shard"`
				StartEpoch uint64 `json:"start_epoch"`
				EndEpoch   uint64 `json:"end_epoch"`
				ParentRoot []byte `json:"parent_root" ssz:"size=32"`
				DataRoot   []byte `json:"data_root" ssz:"size=32"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"Crosslink,omitempty"`
		Deposit struct {
			Value struct {
				Proof [][]byte `json:"proof" ssz:"size=32,32"`
				Data  struct {
					Pubkey                []byte `json:"pubkey" ssz:"size=48"`
					WithdrawalCredentials []byte `json:"withdrawal_credentials" ssz:"size=32"`
					Amount                uint64 `json:"amount"`
					Signature             []byte `json:"signature" ssz:"size=96"`
				} `json:"data"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"Deposit,omitempty"`
		DepositData struct {
			Value struct {
				Pubkey                []byte `json:"pubkey" ssz:"size=48"`
				WithdrawalCredentials []byte `json:"withdrawal_credentials" ssz:"size=32"`
				Amount                uint64 `json:"amount"`
				Signature             []byte `json:"signature" ssz:"size=96"`
			} `json:"value"`
			Serialized  []byte `json:"serialized"`
			Root        []byte `json:"root" ssz:"size=32"`
			SigningRoot []byte `json:"signing_root" ssz:"size=32"`
		} `json:"DepositData,omitempty"`
		Eth1Data struct {
			Value struct {
				DepositRoot  []byte `json:"deposit_root" ssz:"size=32"`
				DepositCount uint64 `json:"deposit_count"`
				BlockHash    []byte `json:"block_hash" ssz:"size=32"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"Eth1Data,omitempty"`
		Fork struct {
			Value struct {
				PreviousVersion []byte `json:"previous_version" ssz:"size=4"`
				CurrentVersion  []byte `json:"current_version" ssz:"size=4"`
				Epoch           uint64 `json:"epoch"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"Fork,omitempty"`
		HistoricalBatch struct {
			Value struct {
				BlockRoots [][]byte `json:"block_roots" ssz:"size=64,32"`
				StateRoots [][]byte `json:"state_roots" ssz:"size=64,32"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"HistoricalBatch,omitempty"`
		IndexedAttestation struct {
			Value struct {
				CustodyBit0Indices []uint64 `json:"custody_bit_0_indices"`
				CustodyBit1Indices []uint64 `json:"custody_bit_1_indices"`
				Data               struct {
					BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
					SourceEpoch     uint64 `json:"source_epoch"`
					SourceRoot      []byte `json:"source_root" ssz:"size=32"`
					TargetEpoch     uint64 `json:"target_epoch"`
					TargetRoot      []byte `json:"target_root" ssz:"size=32"`
					Crosslink       struct {
						Shard      uint64 `json:"shard"`
						StartEpoch uint64 `json:"start_epoch"`
						EndEpoch   uint64 `json:"end_epoch"`
						ParentRoot []byte `json:"parent_root" ssz:"size=32"`
						DataRoot   []byte `json:"data_root" ssz:"size=32"`
					} `json:"crosslink"`
				} `json:"data"`
				Signature []byte `json:"signature" ssz:"size=96"`
			} `json:"value"`
			Serialized  []byte `json:"serialized"`
			Root        []byte `json:"root" ssz:"size=32"`
			SigningRoot []byte `json:"signing_root" ssz:"size=32"`
		} `json:"IndexedAttestation,omitempty"`
		PendingAttestation struct {
			Value struct {
				AggregationBits []byte `json:"aggregation_bitfield"`
				Data            struct {
					BeaconBlockRoot []byte `json:"beacon_block_root" ssz:"size=32"`
					SourceEpoch     uint64 `json:"source_epoch"`
					SourceRoot      []byte `json:"source_root" ssz:"size=32"`
					TargetEpoch     uint64 `json:"target_epoch"`
					TargetRoot      []byte `json:"target_root" ssz:"size=32"`
					Crosslink       struct {
						Shard      uint64 `json:"shard"`
						StartEpoch uint64 `json:"start_epoch"`
						EndEpoch   uint64 `json:"end_epoch"`
						ParentRoot []byte `json:"parent_root" ssz:"size=32"`
						DataRoot   []byte `json:"data_root" ssz:"size=32"`
					} `json:"crosslink"`
				} `json:"data"`
				InclusionDelay uint64 `json:"inclusion_delay"`
				ProposerIndex  uint64 `json:"proposer_index"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"PendingAttestation,omitempty"`
		ProposerSlashing struct {
			Value struct {
				ProposerIndex uint64 `json:"proposer_index"`
				Header1       struct {
					Slot       uint64 `json:"slot"`
					ParentRoot []byte `json:"parent_root" ssz:"size=32"`
					StateRoot  []byte `json:"state_root" ssz:"size=32"`
					BodyRoot   []byte `json:"body_root" ssz:"size=32"`
					Signature  []byte `json:"signature" ssz:"size=96"`
				} `json:"header_1"`
				Header2 struct {
					Slot       uint64 `json:"slot"`
					ParentRoot []byte `json:"parent_root" ssz:"size=32"`
					StateRoot  []byte `json:"state_root" ssz:"size=32"`
					BodyRoot   []byte `json:"body_root" ssz:"size=32"`
					Signature  []byte `json:"signature" ssz:"size=96"`
				} `json:"header_2"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"ProposerSlashing,omitempty"`
		Transfer struct {
			Value struct {
				Sender    uint64 `json:"sender"`
				Recipient uint64 `json:"recipient"`
				Amount    uint64 `json:"amount"`
				Fee       uint64 `json:"fee"`
				Slot      uint64 `json:"slot"`
				Pubkey    []byte `json:"pubkey" ssz:"size=48"`
				Signature []byte `json:"signature" ssz:"size=96"`
			} `json:"value"`
			Serialized  []byte `json:"serialized"`
			Root        []byte `json:"root" ssz:"size=32"`
			SigningRoot []byte `json:"signing_root" ssz:"size=32"`
		} `json:"Transfer,omitempty"`
		Validator struct {
			Value struct {
				Pubkey                     []byte `json:"pubkey" ssz:"size=48"`
				WithdrawalCredentials      []byte `json:"withdrawal_credentials" ssz:"size=32"`
				ActivationEligibilityEpoch uint64 `json:"activation_eligibility_epoch"`
				ActivationEpoch            uint64 `json:"activation_epoch"`
				ExitEpoch                  uint64 `json:"exit_epoch"`
				WithdrawableEpoch          uint64 `json:"withdrawable_epoch"`
				Slashed                    bool   `json:"slashed"`
				EffectiveBalance           uint64 `json:"effective_balance"`
			} `json:"value"`
			Serialized []byte `json:"serialized"`
			Root       []byte `json:"root" ssz:"size=32"`
		} `json:"Validator,omitempty"`
		VoluntaryExit struct {
			Value struct {
				Epoch          uint64 `json:"epoch"`
				ValidatorIndex uint64 `json:"validator_index"`
				Signature      []byte `json:"signature" ssz:"size=96"`
			} `json:"value"`
			Serialized  []byte `json:"serialized"`
			Root        []byte `json:"root" ssz:"size=32"`
			SigningRoot []byte `json:"signing_root" ssz:"size=32"`
		} `json:"VoluntaryExit,omitempty"`
	} `json:"test_cases"`
}

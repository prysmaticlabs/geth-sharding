// Code generated by yaml_to_go. DO NOT EDIT.
// source: fast_aggregate_verify.yaml

package spectest

type FastAggregateVerifyTest struct {
	Input struct {
		Pubkeys   []string `json:"pubkeys"`
		Message   string   `json:"message"`
		Signature string   `json:"signature"`
	} `json:"input"`
	Output bool `json:"output"`
}

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/prysmaticlabs/ethereumapis/eth/v1alpha1 (interfaces: BeaconChainClient,BeaconChain_StreamChainHeadClient,BeaconChain_StreamBlocksClient,BeaconChain_StreamAttestationsClient,BeaconChain_StreamValidatorsInfoClient)

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	empty "github.com/gogo/protobuf/types"
	gomock "github.com/golang/mock/gomock"
	v1alpha1 "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	grpc "google.golang.org/grpc"
	metadata "google.golang.org/grpc/metadata"
	reflect "reflect"
)

// MockBeaconChainClient is a mock of BeaconChainClient interface
type MockBeaconChainClient struct {
	ctrl     *gomock.Controller
	recorder *MockBeaconChainClientMockRecorder
}

// MockBeaconChainClientMockRecorder is the mock recorder for MockBeaconChainClient
type MockBeaconChainClientMockRecorder struct {
	mock *MockBeaconChainClient
}

// NewMockBeaconChainClient creates a new mock instance
func NewMockBeaconChainClient(ctrl *gomock.Controller) *MockBeaconChainClient {
	mock := &MockBeaconChainClient{ctrl: ctrl}
	mock.recorder = &MockBeaconChainClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBeaconChainClient) EXPECT() *MockBeaconChainClientMockRecorder {
	return m.recorder
}

// AttestationPool mocks base method
func (m *MockBeaconChainClient) AttestationPool(arg0 context.Context, arg1 *v1alpha1.AttestationPoolRequest, arg2 ...grpc.CallOption) (*v1alpha1.AttestationPoolResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "AttestationPool", varargs...)
	ret0, _ := ret[0].(*v1alpha1.AttestationPoolResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AttestationPool indicates an expected call of AttestationPool
func (mr *MockBeaconChainClientMockRecorder) AttestationPool(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AttestationPool", reflect.TypeOf((*MockBeaconChainClient)(nil).AttestationPool), varargs...)
}

// GetBeaconConfig mocks base method
func (m *MockBeaconChainClient) GetBeaconConfig(arg0 context.Context, arg1 *empty.Empty, arg2 ...grpc.CallOption) (*v1alpha1.BeaconConfig, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetBeaconConfig", varargs...)
	ret0, _ := ret[0].(*v1alpha1.BeaconConfig)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetBeaconConfig indicates an expected call of GetBeaconConfig
func (mr *MockBeaconChainClientMockRecorder) GetBeaconConfig(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBeaconConfig", reflect.TypeOf((*MockBeaconChainClient)(nil).GetBeaconConfig), varargs...)
}

// GetChainHead mocks base method
func (m *MockBeaconChainClient) GetChainHead(arg0 context.Context, arg1 *empty.Empty, arg2 ...grpc.CallOption) (*v1alpha1.ChainHead, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetChainHead", varargs...)
	ret0, _ := ret[0].(*v1alpha1.ChainHead)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetChainHead indicates an expected call of GetChainHead
func (mr *MockBeaconChainClientMockRecorder) GetChainHead(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetChainHead", reflect.TypeOf((*MockBeaconChainClient)(nil).GetChainHead), varargs...)
}

// GetValidator mocks base method
func (m *MockBeaconChainClient) GetValidator(arg0 context.Context, arg1 *v1alpha1.GetValidatorRequest, arg2 ...grpc.CallOption) (*v1alpha1.Validator, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetValidator", varargs...)
	ret0, _ := ret[0].(*v1alpha1.Validator)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetValidator indicates an expected call of GetValidator
func (mr *MockBeaconChainClientMockRecorder) GetValidator(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetValidator", reflect.TypeOf((*MockBeaconChainClient)(nil).GetValidator), varargs...)
}

// GetValidatorActiveSetChanges mocks base method
func (m *MockBeaconChainClient) GetValidatorActiveSetChanges(arg0 context.Context, arg1 *v1alpha1.GetValidatorActiveSetChangesRequest, arg2 ...grpc.CallOption) (*v1alpha1.ActiveSetChanges, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetValidatorActiveSetChanges", varargs...)
	ret0, _ := ret[0].(*v1alpha1.ActiveSetChanges)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetValidatorActiveSetChanges indicates an expected call of GetValidatorActiveSetChanges
func (mr *MockBeaconChainClientMockRecorder) GetValidatorActiveSetChanges(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetValidatorActiveSetChanges", reflect.TypeOf((*MockBeaconChainClient)(nil).GetValidatorActiveSetChanges), varargs...)
}

// GetValidatorParticipation mocks base method
func (m *MockBeaconChainClient) GetValidatorParticipation(arg0 context.Context, arg1 *v1alpha1.GetValidatorParticipationRequest, arg2 ...grpc.CallOption) (*v1alpha1.ValidatorParticipationResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetValidatorParticipation", varargs...)
	ret0, _ := ret[0].(*v1alpha1.ValidatorParticipationResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetValidatorParticipation indicates an expected call of GetValidatorParticipation
func (mr *MockBeaconChainClientMockRecorder) GetValidatorParticipation(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetValidatorParticipation", reflect.TypeOf((*MockBeaconChainClient)(nil).GetValidatorParticipation), varargs...)
}

// GetValidatorPerformance mocks base method
func (m *MockBeaconChainClient) GetValidatorPerformance(arg0 context.Context, arg1 *v1alpha1.ValidatorPerformanceRequest, arg2 ...grpc.CallOption) (*v1alpha1.ValidatorPerformanceResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetValidatorPerformance", varargs...)
	ret0, _ := ret[0].(*v1alpha1.ValidatorPerformanceResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetValidatorPerformance indicates an expected call of GetValidatorPerformance
func (mr *MockBeaconChainClientMockRecorder) GetValidatorPerformance(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetValidatorPerformance", reflect.TypeOf((*MockBeaconChainClient)(nil).GetValidatorPerformance), varargs...)
}

// GetValidatorQueue mocks base method
func (m *MockBeaconChainClient) GetValidatorQueue(arg0 context.Context, arg1 *empty.Empty, arg2 ...grpc.CallOption) (*v1alpha1.ValidatorQueue, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetValidatorQueue", varargs...)
	ret0, _ := ret[0].(*v1alpha1.ValidatorQueue)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetValidatorQueue indicates an expected call of GetValidatorQueue
func (mr *MockBeaconChainClientMockRecorder) GetValidatorQueue(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetValidatorQueue", reflect.TypeOf((*MockBeaconChainClient)(nil).GetValidatorQueue), varargs...)
}

// ListAttestations mocks base method
func (m *MockBeaconChainClient) ListAttestations(arg0 context.Context, arg1 *v1alpha1.ListAttestationsRequest, arg2 ...grpc.CallOption) (*v1alpha1.ListAttestationsResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListAttestations", varargs...)
	ret0, _ := ret[0].(*v1alpha1.ListAttestationsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListAttestations indicates an expected call of ListAttestations
func (mr *MockBeaconChainClientMockRecorder) ListAttestations(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListAttestations", reflect.TypeOf((*MockBeaconChainClient)(nil).ListAttestations), varargs...)
}

// ListBeaconCommittees mocks base method
func (m *MockBeaconChainClient) ListBeaconCommittees(arg0 context.Context, arg1 *v1alpha1.ListCommitteesRequest, arg2 ...grpc.CallOption) (*v1alpha1.BeaconCommittees, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListBeaconCommittees", varargs...)
	ret0, _ := ret[0].(*v1alpha1.BeaconCommittees)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListBeaconCommittees indicates an expected call of ListBeaconCommittees
func (mr *MockBeaconChainClientMockRecorder) ListBeaconCommittees(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBeaconCommittees", reflect.TypeOf((*MockBeaconChainClient)(nil).ListBeaconCommittees), varargs...)
}

// ListBlocks mocks base method
func (m *MockBeaconChainClient) ListBlocks(arg0 context.Context, arg1 *v1alpha1.ListBlocksRequest, arg2 ...grpc.CallOption) (*v1alpha1.ListBlocksResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListBlocks", varargs...)
	ret0, _ := ret[0].(*v1alpha1.ListBlocksResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListBlocks indicates an expected call of ListBlocks
func (mr *MockBeaconChainClientMockRecorder) ListBlocks(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBlocks", reflect.TypeOf((*MockBeaconChainClient)(nil).ListBlocks), varargs...)
}

// ListValidatorAssignments mocks base method
func (m *MockBeaconChainClient) ListValidatorAssignments(arg0 context.Context, arg1 *v1alpha1.ListValidatorAssignmentsRequest, arg2 ...grpc.CallOption) (*v1alpha1.ValidatorAssignments, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListValidatorAssignments", varargs...)
	ret0, _ := ret[0].(*v1alpha1.ValidatorAssignments)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListValidatorAssignments indicates an expected call of ListValidatorAssignments
func (mr *MockBeaconChainClientMockRecorder) ListValidatorAssignments(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListValidatorAssignments", reflect.TypeOf((*MockBeaconChainClient)(nil).ListValidatorAssignments), varargs...)
}

// ListValidatorBalances mocks base method
func (m *MockBeaconChainClient) ListValidatorBalances(arg0 context.Context, arg1 *v1alpha1.ListValidatorBalancesRequest, arg2 ...grpc.CallOption) (*v1alpha1.ValidatorBalances, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListValidatorBalances", varargs...)
	ret0, _ := ret[0].(*v1alpha1.ValidatorBalances)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListValidatorBalances indicates an expected call of ListValidatorBalances
func (mr *MockBeaconChainClientMockRecorder) ListValidatorBalances(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListValidatorBalances", reflect.TypeOf((*MockBeaconChainClient)(nil).ListValidatorBalances), varargs...)
}

// ListValidators mocks base method
func (m *MockBeaconChainClient) ListValidators(arg0 context.Context, arg1 *v1alpha1.ListValidatorsRequest, arg2 ...grpc.CallOption) (*v1alpha1.Validators, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ListValidators", varargs...)
	ret0, _ := ret[0].(*v1alpha1.Validators)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListValidators indicates an expected call of ListValidators
func (mr *MockBeaconChainClientMockRecorder) ListValidators(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListValidators", reflect.TypeOf((*MockBeaconChainClient)(nil).ListValidators), varargs...)
}

// StreamAttestations mocks base method
func (m *MockBeaconChainClient) StreamAttestations(arg0 context.Context, arg1 *empty.Empty, arg2 ...grpc.CallOption) (v1alpha1.BeaconChain_StreamAttestationsClient, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "StreamAttestations", varargs...)
	ret0, _ := ret[0].(v1alpha1.BeaconChain_StreamAttestationsClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StreamAttestations indicates an expected call of StreamAttestations
func (mr *MockBeaconChainClientMockRecorder) StreamAttestations(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StreamAttestations", reflect.TypeOf((*MockBeaconChainClient)(nil).StreamAttestations), varargs...)
}

// StreamBlocks mocks base method
func (m *MockBeaconChainClient) StreamBlocks(arg0 context.Context, arg1 *empty.Empty, arg2 ...grpc.CallOption) (v1alpha1.BeaconChain_StreamBlocksClient, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "StreamBlocks", varargs...)
	ret0, _ := ret[0].(v1alpha1.BeaconChain_StreamBlocksClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StreamBlocks indicates an expected call of StreamBlocks
func (mr *MockBeaconChainClientMockRecorder) StreamBlocks(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StreamBlocks", reflect.TypeOf((*MockBeaconChainClient)(nil).StreamBlocks), varargs...)
}

// StreamChainHead mocks base method
func (m *MockBeaconChainClient) StreamChainHead(arg0 context.Context, arg1 *empty.Empty, arg2 ...grpc.CallOption) (v1alpha1.BeaconChain_StreamChainHeadClient, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "StreamChainHead", varargs...)
	ret0, _ := ret[0].(v1alpha1.BeaconChain_StreamChainHeadClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StreamChainHead indicates an expected call of StreamChainHead
func (mr *MockBeaconChainClientMockRecorder) StreamChainHead(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StreamChainHead", reflect.TypeOf((*MockBeaconChainClient)(nil).StreamChainHead), varargs...)
}

// StreamValidatorsInfo mocks base method
func (m *MockBeaconChainClient) StreamValidatorsInfo(arg0 context.Context, arg1 ...grpc.CallOption) (v1alpha1.BeaconChain_StreamValidatorsInfoClient, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "StreamValidatorsInfo", varargs...)
	ret0, _ := ret[0].(v1alpha1.BeaconChain_StreamValidatorsInfoClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StreamValidatorsInfo indicates an expected call of StreamValidatorsInfo
func (mr *MockBeaconChainClientMockRecorder) StreamValidatorsInfo(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StreamValidatorsInfo", reflect.TypeOf((*MockBeaconChainClient)(nil).StreamValidatorsInfo), varargs...)
}

// MockBeaconChain_StreamChainHeadClient is a mock of BeaconChain_StreamChainHeadClient interface
type MockBeaconChain_StreamChainHeadClient struct {
	ctrl     *gomock.Controller
	recorder *MockBeaconChain_StreamChainHeadClientMockRecorder
}

// MockBeaconChain_StreamChainHeadClientMockRecorder is the mock recorder for MockBeaconChain_StreamChainHeadClient
type MockBeaconChain_StreamChainHeadClientMockRecorder struct {
	mock *MockBeaconChain_StreamChainHeadClient
}

// NewMockBeaconChain_StreamChainHeadClient creates a new mock instance
func NewMockBeaconChain_StreamChainHeadClient(ctrl *gomock.Controller) *MockBeaconChain_StreamChainHeadClient {
	mock := &MockBeaconChain_StreamChainHeadClient{ctrl: ctrl}
	mock.recorder = &MockBeaconChain_StreamChainHeadClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBeaconChain_StreamChainHeadClient) EXPECT() *MockBeaconChain_StreamChainHeadClientMockRecorder {
	return m.recorder
}

// CloseSend mocks base method
func (m *MockBeaconChain_StreamChainHeadClient) CloseSend() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseSend")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseSend indicates an expected call of CloseSend
func (mr *MockBeaconChain_StreamChainHeadClientMockRecorder) CloseSend() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseSend", reflect.TypeOf((*MockBeaconChain_StreamChainHeadClient)(nil).CloseSend))
}

// ctx mocks base method
func (m *MockBeaconChain_StreamChainHeadClient) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ctx")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// ctx indicates an expected call of ctx
func (mr *MockBeaconChain_StreamChainHeadClientMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ctx", reflect.TypeOf((*MockBeaconChain_StreamChainHeadClient)(nil).Context))
}

// Header mocks base method
func (m *MockBeaconChain_StreamChainHeadClient) Header() (metadata.MD, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Header")
	ret0, _ := ret[0].(metadata.MD)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Header indicates an expected call of Header
func (mr *MockBeaconChain_StreamChainHeadClientMockRecorder) Header() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Header", reflect.TypeOf((*MockBeaconChain_StreamChainHeadClient)(nil).Header))
}

// Recv mocks base method
func (m *MockBeaconChain_StreamChainHeadClient) Recv() (*v1alpha1.ChainHead, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Recv")
	ret0, _ := ret[0].(*v1alpha1.ChainHead)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Recv indicates an expected call of Recv
func (mr *MockBeaconChain_StreamChainHeadClientMockRecorder) Recv() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Recv", reflect.TypeOf((*MockBeaconChain_StreamChainHeadClient)(nil).Recv))
}

// RecvMsg mocks base method
func (m *MockBeaconChain_StreamChainHeadClient) RecvMsg(arg0 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecvMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg
func (mr *MockBeaconChain_StreamChainHeadClientMockRecorder) RecvMsg(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockBeaconChain_StreamChainHeadClient)(nil).RecvMsg), arg0)
}

// SendMsg mocks base method
func (m *MockBeaconChain_StreamChainHeadClient) SendMsg(arg0 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg
func (mr *MockBeaconChain_StreamChainHeadClientMockRecorder) SendMsg(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockBeaconChain_StreamChainHeadClient)(nil).SendMsg), arg0)
}

// Trailer mocks base method
func (m *MockBeaconChain_StreamChainHeadClient) Trailer() metadata.MD {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Trailer")
	ret0, _ := ret[0].(metadata.MD)
	return ret0
}

// Trailer indicates an expected call of Trailer
func (mr *MockBeaconChain_StreamChainHeadClientMockRecorder) Trailer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Trailer", reflect.TypeOf((*MockBeaconChain_StreamChainHeadClient)(nil).Trailer))
}

// MockBeaconChain_StreamBlocksClient is a mock of BeaconChain_StreamBlocksClient interface
type MockBeaconChain_StreamBlocksClient struct {
	ctrl     *gomock.Controller
	recorder *MockBeaconChain_StreamBlocksClientMockRecorder
}

// MockBeaconChain_StreamBlocksClientMockRecorder is the mock recorder for MockBeaconChain_StreamBlocksClient
type MockBeaconChain_StreamBlocksClientMockRecorder struct {
	mock *MockBeaconChain_StreamBlocksClient
}

// NewMockBeaconChain_StreamBlocksClient creates a new mock instance
func NewMockBeaconChain_StreamBlocksClient(ctrl *gomock.Controller) *MockBeaconChain_StreamBlocksClient {
	mock := &MockBeaconChain_StreamBlocksClient{ctrl: ctrl}
	mock.recorder = &MockBeaconChain_StreamBlocksClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBeaconChain_StreamBlocksClient) EXPECT() *MockBeaconChain_StreamBlocksClientMockRecorder {
	return m.recorder
}

// CloseSend mocks base method
func (m *MockBeaconChain_StreamBlocksClient) CloseSend() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseSend")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseSend indicates an expected call of CloseSend
func (mr *MockBeaconChain_StreamBlocksClientMockRecorder) CloseSend() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseSend", reflect.TypeOf((*MockBeaconChain_StreamBlocksClient)(nil).CloseSend))
}

// ctx mocks base method
func (m *MockBeaconChain_StreamBlocksClient) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ctx")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// ctx indicates an expected call of ctx
func (mr *MockBeaconChain_StreamBlocksClientMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ctx", reflect.TypeOf((*MockBeaconChain_StreamBlocksClient)(nil).Context))
}

// Header mocks base method
func (m *MockBeaconChain_StreamBlocksClient) Header() (metadata.MD, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Header")
	ret0, _ := ret[0].(metadata.MD)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Header indicates an expected call of Header
func (mr *MockBeaconChain_StreamBlocksClientMockRecorder) Header() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Header", reflect.TypeOf((*MockBeaconChain_StreamBlocksClient)(nil).Header))
}

// Recv mocks base method
func (m *MockBeaconChain_StreamBlocksClient) Recv() (*v1alpha1.SignedBeaconBlock, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Recv")
	ret0, _ := ret[0].(*v1alpha1.SignedBeaconBlock)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Recv indicates an expected call of Recv
func (mr *MockBeaconChain_StreamBlocksClientMockRecorder) Recv() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Recv", reflect.TypeOf((*MockBeaconChain_StreamBlocksClient)(nil).Recv))
}

// RecvMsg mocks base method
func (m *MockBeaconChain_StreamBlocksClient) RecvMsg(arg0 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecvMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg
func (mr *MockBeaconChain_StreamBlocksClientMockRecorder) RecvMsg(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockBeaconChain_StreamBlocksClient)(nil).RecvMsg), arg0)
}

// SendMsg mocks base method
func (m *MockBeaconChain_StreamBlocksClient) SendMsg(arg0 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg
func (mr *MockBeaconChain_StreamBlocksClientMockRecorder) SendMsg(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockBeaconChain_StreamBlocksClient)(nil).SendMsg), arg0)
}

// Trailer mocks base method
func (m *MockBeaconChain_StreamBlocksClient) Trailer() metadata.MD {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Trailer")
	ret0, _ := ret[0].(metadata.MD)
	return ret0
}

// Trailer indicates an expected call of Trailer
func (mr *MockBeaconChain_StreamBlocksClientMockRecorder) Trailer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Trailer", reflect.TypeOf((*MockBeaconChain_StreamBlocksClient)(nil).Trailer))
}

// MockBeaconChain_StreamAttestationsClient is a mock of BeaconChain_StreamAttestationsClient interface
type MockBeaconChain_StreamAttestationsClient struct {
	ctrl     *gomock.Controller
	recorder *MockBeaconChain_StreamAttestationsClientMockRecorder
}

// MockBeaconChain_StreamAttestationsClientMockRecorder is the mock recorder for MockBeaconChain_StreamAttestationsClient
type MockBeaconChain_StreamAttestationsClientMockRecorder struct {
	mock *MockBeaconChain_StreamAttestationsClient
}

// NewMockBeaconChain_StreamAttestationsClient creates a new mock instance
func NewMockBeaconChain_StreamAttestationsClient(ctrl *gomock.Controller) *MockBeaconChain_StreamAttestationsClient {
	mock := &MockBeaconChain_StreamAttestationsClient{ctrl: ctrl}
	mock.recorder = &MockBeaconChain_StreamAttestationsClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBeaconChain_StreamAttestationsClient) EXPECT() *MockBeaconChain_StreamAttestationsClientMockRecorder {
	return m.recorder
}

// CloseSend mocks base method
func (m *MockBeaconChain_StreamAttestationsClient) CloseSend() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseSend")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseSend indicates an expected call of CloseSend
func (mr *MockBeaconChain_StreamAttestationsClientMockRecorder) CloseSend() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseSend", reflect.TypeOf((*MockBeaconChain_StreamAttestationsClient)(nil).CloseSend))
}

// ctx mocks base method
func (m *MockBeaconChain_StreamAttestationsClient) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ctx")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// ctx indicates an expected call of ctx
func (mr *MockBeaconChain_StreamAttestationsClientMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ctx", reflect.TypeOf((*MockBeaconChain_StreamAttestationsClient)(nil).Context))
}

// Header mocks base method
func (m *MockBeaconChain_StreamAttestationsClient) Header() (metadata.MD, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Header")
	ret0, _ := ret[0].(metadata.MD)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Header indicates an expected call of Header
func (mr *MockBeaconChain_StreamAttestationsClientMockRecorder) Header() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Header", reflect.TypeOf((*MockBeaconChain_StreamAttestationsClient)(nil).Header))
}

// Recv mocks base method
func (m *MockBeaconChain_StreamAttestationsClient) Recv() (*v1alpha1.Attestation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Recv")
	ret0, _ := ret[0].(*v1alpha1.Attestation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Recv indicates an expected call of Recv
func (mr *MockBeaconChain_StreamAttestationsClientMockRecorder) Recv() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Recv", reflect.TypeOf((*MockBeaconChain_StreamAttestationsClient)(nil).Recv))
}

// RecvMsg mocks base method
func (m *MockBeaconChain_StreamAttestationsClient) RecvMsg(arg0 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecvMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg
func (mr *MockBeaconChain_StreamAttestationsClientMockRecorder) RecvMsg(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockBeaconChain_StreamAttestationsClient)(nil).RecvMsg), arg0)
}

// SendMsg mocks base method
func (m *MockBeaconChain_StreamAttestationsClient) SendMsg(arg0 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg
func (mr *MockBeaconChain_StreamAttestationsClientMockRecorder) SendMsg(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockBeaconChain_StreamAttestationsClient)(nil).SendMsg), arg0)
}

// Trailer mocks base method
func (m *MockBeaconChain_StreamAttestationsClient) Trailer() metadata.MD {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Trailer")
	ret0, _ := ret[0].(metadata.MD)
	return ret0
}

// Trailer indicates an expected call of Trailer
func (mr *MockBeaconChain_StreamAttestationsClientMockRecorder) Trailer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Trailer", reflect.TypeOf((*MockBeaconChain_StreamAttestationsClient)(nil).Trailer))
}

// MockBeaconChain_StreamValidatorsInfoClient is a mock of BeaconChain_StreamValidatorsInfoClient interface
type MockBeaconChain_StreamValidatorsInfoClient struct {
	ctrl     *gomock.Controller
	recorder *MockBeaconChain_StreamValidatorsInfoClientMockRecorder
}

// MockBeaconChain_StreamValidatorsInfoClientMockRecorder is the mock recorder for MockBeaconChain_StreamValidatorsInfoClient
type MockBeaconChain_StreamValidatorsInfoClientMockRecorder struct {
	mock *MockBeaconChain_StreamValidatorsInfoClient
}

// NewMockBeaconChain_StreamValidatorsInfoClient creates a new mock instance
func NewMockBeaconChain_StreamValidatorsInfoClient(ctrl *gomock.Controller) *MockBeaconChain_StreamValidatorsInfoClient {
	mock := &MockBeaconChain_StreamValidatorsInfoClient{ctrl: ctrl}
	mock.recorder = &MockBeaconChain_StreamValidatorsInfoClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBeaconChain_StreamValidatorsInfoClient) EXPECT() *MockBeaconChain_StreamValidatorsInfoClientMockRecorder {
	return m.recorder
}

// CloseSend mocks base method
func (m *MockBeaconChain_StreamValidatorsInfoClient) CloseSend() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseSend")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseSend indicates an expected call of CloseSend
func (mr *MockBeaconChain_StreamValidatorsInfoClientMockRecorder) CloseSend() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseSend", reflect.TypeOf((*MockBeaconChain_StreamValidatorsInfoClient)(nil).CloseSend))
}

// ctx mocks base method
func (m *MockBeaconChain_StreamValidatorsInfoClient) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ctx")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// ctx indicates an expected call of ctx
func (mr *MockBeaconChain_StreamValidatorsInfoClientMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ctx", reflect.TypeOf((*MockBeaconChain_StreamValidatorsInfoClient)(nil).Context))
}

// Header mocks base method
func (m *MockBeaconChain_StreamValidatorsInfoClient) Header() (metadata.MD, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Header")
	ret0, _ := ret[0].(metadata.MD)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Header indicates an expected call of Header
func (mr *MockBeaconChain_StreamValidatorsInfoClientMockRecorder) Header() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Header", reflect.TypeOf((*MockBeaconChain_StreamValidatorsInfoClient)(nil).Header))
}

// Recv mocks base method
func (m *MockBeaconChain_StreamValidatorsInfoClient) Recv() (*v1alpha1.ValidatorInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Recv")
	ret0, _ := ret[0].(*v1alpha1.ValidatorInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Recv indicates an expected call of Recv
func (mr *MockBeaconChain_StreamValidatorsInfoClientMockRecorder) Recv() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Recv", reflect.TypeOf((*MockBeaconChain_StreamValidatorsInfoClient)(nil).Recv))
}

// RecvMsg mocks base method
func (m *MockBeaconChain_StreamValidatorsInfoClient) RecvMsg(arg0 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecvMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg
func (mr *MockBeaconChain_StreamValidatorsInfoClientMockRecorder) RecvMsg(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockBeaconChain_StreamValidatorsInfoClient)(nil).RecvMsg), arg0)
}

// Send mocks base method
func (m *MockBeaconChain_StreamValidatorsInfoClient) Send(arg0 *v1alpha1.ValidatorChangeSet) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send
func (mr *MockBeaconChain_StreamValidatorsInfoClientMockRecorder) Send(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockBeaconChain_StreamValidatorsInfoClient)(nil).Send), arg0)
}

// SendMsg mocks base method
func (m *MockBeaconChain_StreamValidatorsInfoClient) SendMsg(arg0 interface{}) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg
func (mr *MockBeaconChain_StreamValidatorsInfoClientMockRecorder) SendMsg(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockBeaconChain_StreamValidatorsInfoClient)(nil).SendMsg), arg0)
}

// Trailer mocks base method
func (m *MockBeaconChain_StreamValidatorsInfoClient) Trailer() metadata.MD {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Trailer")
	ret0, _ := ret[0].(metadata.MD)
	return ret0
}

// Trailer indicates an expected call of Trailer
func (mr *MockBeaconChain_StreamValidatorsInfoClientMockRecorder) Trailer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Trailer", reflect.TypeOf((*MockBeaconChain_StreamValidatorsInfoClient)(nil).Trailer))
}

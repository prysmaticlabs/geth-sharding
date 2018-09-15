// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1 (interfaces: BeaconServiceServer,BeaconService_LatestBeaconBlockServer,BeaconService_LatestCrystallizedStateServer,BeaconService_LatestAttestationServer)

package internal

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	empty "github.com/golang/protobuf/ptypes/empty"
	v1 "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	v10 "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	metadata "google.golang.org/grpc/metadata"
	reflect "reflect"
)

// MockBeaconServiceServer is a mock of BeaconServiceServer interface
type MockBeaconServiceServer struct {
	ctrl     *gomock.Controller
	recorder *MockBeaconServiceServerMockRecorder
}

// MockBeaconServiceServerMockRecorder is the mock recorder for MockBeaconServiceServer
type MockBeaconServiceServerMockRecorder struct {
	mock *MockBeaconServiceServer
}

// NewMockBeaconServiceServer creates a new mock instance
func NewMockBeaconServiceServer(ctrl *gomock.Controller) *MockBeaconServiceServer {
	mock := &MockBeaconServiceServer{ctrl: ctrl}
	mock.recorder = &MockBeaconServiceServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBeaconServiceServer) EXPECT() *MockBeaconServiceServerMockRecorder {
	return m.recorder
}

// FetchShuffledValidatorIndices mocks base method
func (m *MockBeaconServiceServer) FetchShuffledValidatorIndices(arg0 context.Context, arg1 *v10.ShuffleRequest) (*v10.ShuffleResponse, error) {
	ret := m.ctrl.Call(m, "FetchShuffledValidatorIndices", arg0, arg1)
	ret0, _ := ret[0].(*v10.ShuffleResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchShuffledValidatorIndices indicates an expected call of FetchShuffledValidatorIndices
func (mr *MockBeaconServiceServerMockRecorder) FetchShuffledValidatorIndices(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchShuffledValidatorIndices", reflect.TypeOf((*MockBeaconServiceServer)(nil).FetchShuffledValidatorIndices), arg0, arg1)
}

// LatestAttestation mocks base method
func (m *MockBeaconServiceServer) LatestAttestation(arg0 *empty.Empty, arg1 v10.BeaconService_LatestAttestationServer) error {
	ret := m.ctrl.Call(m, "LatestAttestation", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// LatestAttestation indicates an expected call of LatestAttestation
func (mr *MockBeaconServiceServerMockRecorder) LatestAttestation(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LatestAttestation", reflect.TypeOf((*MockBeaconServiceServer)(nil).LatestAttestation), arg0, arg1)
}

// LatestBeaconBlock mocks base method
func (m *MockBeaconServiceServer) LatestBeaconBlock(arg0 *empty.Empty, arg1 v10.BeaconService_LatestBeaconBlockServer) error {
	ret := m.ctrl.Call(m, "LatestBeaconBlock", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// LatestBeaconBlock indicates an expected call of LatestBeaconBlock
func (mr *MockBeaconServiceServerMockRecorder) LatestBeaconBlock(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LatestBeaconBlock", reflect.TypeOf((*MockBeaconServiceServer)(nil).LatestBeaconBlock), arg0, arg1)
}

// LatestCrystallizedState mocks base method
func (m *MockBeaconServiceServer) LatestCrystallizedState(arg0 *empty.Empty, arg1 v10.BeaconService_LatestCrystallizedStateServer) error {
	ret := m.ctrl.Call(m, "LatestCrystallizedState", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// LatestCrystallizedState indicates an expected call of LatestCrystallizedState
func (mr *MockBeaconServiceServerMockRecorder) LatestCrystallizedState(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LatestCrystallizedState", reflect.TypeOf((*MockBeaconServiceServer)(nil).LatestCrystallizedState), arg0, arg1)
}

// MockBeaconService_LatestBeaconBlockServer is a mock of BeaconService_LatestBeaconBlockServer interface
type MockBeaconService_LatestBeaconBlockServer struct {
	ctrl     *gomock.Controller
	recorder *MockBeaconService_LatestBeaconBlockServerMockRecorder
}

// MockBeaconService_LatestBeaconBlockServerMockRecorder is the mock recorder for MockBeaconService_LatestBeaconBlockServer
type MockBeaconService_LatestBeaconBlockServerMockRecorder struct {
	mock *MockBeaconService_LatestBeaconBlockServer
}

// NewMockBeaconService_LatestBeaconBlockServer creates a new mock instance
func NewMockBeaconService_LatestBeaconBlockServer(ctrl *gomock.Controller) *MockBeaconService_LatestBeaconBlockServer {
	mock := &MockBeaconService_LatestBeaconBlockServer{ctrl: ctrl}
	mock.recorder = &MockBeaconService_LatestBeaconBlockServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBeaconService_LatestBeaconBlockServer) EXPECT() *MockBeaconService_LatestBeaconBlockServerMockRecorder {
	return m.recorder
}

// Context mocks base method
func (m *MockBeaconService_LatestBeaconBlockServer) Context() context.Context {
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context
func (mr *MockBeaconService_LatestBeaconBlockServerMockRecorder) Context() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockBeaconService_LatestBeaconBlockServer)(nil).Context))
}

// RecvMsg mocks base method
func (m *MockBeaconService_LatestBeaconBlockServer) RecvMsg(arg0 interface{}) error {
	ret := m.ctrl.Call(m, "RecvMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg
func (mr *MockBeaconService_LatestBeaconBlockServerMockRecorder) RecvMsg(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockBeaconService_LatestBeaconBlockServer)(nil).RecvMsg), arg0)
}

// Send mocks base method
func (m *MockBeaconService_LatestBeaconBlockServer) Send(arg0 *v1.BeaconBlock) error {
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send
func (mr *MockBeaconService_LatestBeaconBlockServerMockRecorder) Send(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockBeaconService_LatestBeaconBlockServer)(nil).Send), arg0)
}

// SendHeader mocks base method
func (m *MockBeaconService_LatestBeaconBlockServer) SendHeader(arg0 metadata.MD) error {
	ret := m.ctrl.Call(m, "SendHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendHeader indicates an expected call of SendHeader
func (mr *MockBeaconService_LatestBeaconBlockServerMockRecorder) SendHeader(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendHeader", reflect.TypeOf((*MockBeaconService_LatestBeaconBlockServer)(nil).SendHeader), arg0)
}

// SendMsg mocks base method
func (m *MockBeaconService_LatestBeaconBlockServer) SendMsg(arg0 interface{}) error {
	ret := m.ctrl.Call(m, "SendMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg
func (mr *MockBeaconService_LatestBeaconBlockServerMockRecorder) SendMsg(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockBeaconService_LatestBeaconBlockServer)(nil).SendMsg), arg0)
}

// SetHeader mocks base method
func (m *MockBeaconService_LatestBeaconBlockServer) SetHeader(arg0 metadata.MD) error {
	ret := m.ctrl.Call(m, "SetHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetHeader indicates an expected call of SetHeader
func (mr *MockBeaconService_LatestBeaconBlockServerMockRecorder) SetHeader(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetHeader", reflect.TypeOf((*MockBeaconService_LatestBeaconBlockServer)(nil).SetHeader), arg0)
}

// SetTrailer mocks base method
func (m *MockBeaconService_LatestBeaconBlockServer) SetTrailer(arg0 metadata.MD) {
	m.ctrl.Call(m, "SetTrailer", arg0)
}

// SetTrailer indicates an expected call of SetTrailer
func (mr *MockBeaconService_LatestBeaconBlockServerMockRecorder) SetTrailer(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTrailer", reflect.TypeOf((*MockBeaconService_LatestBeaconBlockServer)(nil).SetTrailer), arg0)
}

// MockBeaconService_LatestCrystallizedStateServer is a mock of BeaconService_LatestCrystallizedStateServer interface
type MockBeaconService_LatestCrystallizedStateServer struct {
	ctrl     *gomock.Controller
	recorder *MockBeaconService_LatestCrystallizedStateServerMockRecorder
}

// MockBeaconService_LatestCrystallizedStateServerMockRecorder is the mock recorder for MockBeaconService_LatestCrystallizedStateServer
type MockBeaconService_LatestCrystallizedStateServerMockRecorder struct {
	mock *MockBeaconService_LatestCrystallizedStateServer
}

// NewMockBeaconService_LatestCrystallizedStateServer creates a new mock instance
func NewMockBeaconService_LatestCrystallizedStateServer(ctrl *gomock.Controller) *MockBeaconService_LatestCrystallizedStateServer {
	mock := &MockBeaconService_LatestCrystallizedStateServer{ctrl: ctrl}
	mock.recorder = &MockBeaconService_LatestCrystallizedStateServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBeaconService_LatestCrystallizedStateServer) EXPECT() *MockBeaconService_LatestCrystallizedStateServerMockRecorder {
	return m.recorder
}

// Context mocks base method
func (m *MockBeaconService_LatestCrystallizedStateServer) Context() context.Context {
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context
func (mr *MockBeaconService_LatestCrystallizedStateServerMockRecorder) Context() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockBeaconService_LatestCrystallizedStateServer)(nil).Context))
}

// RecvMsg mocks base method
func (m *MockBeaconService_LatestCrystallizedStateServer) RecvMsg(arg0 interface{}) error {
	ret := m.ctrl.Call(m, "RecvMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg
func (mr *MockBeaconService_LatestCrystallizedStateServerMockRecorder) RecvMsg(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockBeaconService_LatestCrystallizedStateServer)(nil).RecvMsg), arg0)
}

// Send mocks base method
func (m *MockBeaconService_LatestCrystallizedStateServer) Send(arg0 *v1.CrystallizedState) error {
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send
func (mr *MockBeaconService_LatestCrystallizedStateServerMockRecorder) Send(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockBeaconService_LatestCrystallizedStateServer)(nil).Send), arg0)
}

// SendHeader mocks base method
func (m *MockBeaconService_LatestCrystallizedStateServer) SendHeader(arg0 metadata.MD) error {
	ret := m.ctrl.Call(m, "SendHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendHeader indicates an expected call of SendHeader
func (mr *MockBeaconService_LatestCrystallizedStateServerMockRecorder) SendHeader(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendHeader", reflect.TypeOf((*MockBeaconService_LatestCrystallizedStateServer)(nil).SendHeader), arg0)
}

// SendMsg mocks base method
func (m *MockBeaconService_LatestCrystallizedStateServer) SendMsg(arg0 interface{}) error {
	ret := m.ctrl.Call(m, "SendMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg
func (mr *MockBeaconService_LatestCrystallizedStateServerMockRecorder) SendMsg(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockBeaconService_LatestCrystallizedStateServer)(nil).SendMsg), arg0)
}

// SetHeader mocks base method
func (m *MockBeaconService_LatestCrystallizedStateServer) SetHeader(arg0 metadata.MD) error {
	ret := m.ctrl.Call(m, "SetHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetHeader indicates an expected call of SetHeader
func (mr *MockBeaconService_LatestCrystallizedStateServerMockRecorder) SetHeader(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetHeader", reflect.TypeOf((*MockBeaconService_LatestCrystallizedStateServer)(nil).SetHeader), arg0)
}

// SetTrailer mocks base method
func (m *MockBeaconService_LatestCrystallizedStateServer) SetTrailer(arg0 metadata.MD) {
	m.ctrl.Call(m, "SetTrailer", arg0)
}

// SetTrailer indicates an expected call of SetTrailer
func (mr *MockBeaconService_LatestCrystallizedStateServerMockRecorder) SetTrailer(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTrailer", reflect.TypeOf((*MockBeaconService_LatestCrystallizedStateServer)(nil).SetTrailer), arg0)
}

// MockBeaconService_LatestAttestationServer is a mock of BeaconService_LatestAttestationServer interface
type MockBeaconService_LatestAttestationServer struct {
	ctrl     *gomock.Controller
	recorder *MockBeaconService_LatestAttestationServerMockRecorder
}

// MockBeaconService_LatestAttestationServerMockRecorder is the mock recorder for MockBeaconService_LatestAttestationServer
type MockBeaconService_LatestAttestationServerMockRecorder struct {
	mock *MockBeaconService_LatestAttestationServer
}

// NewMockBeaconService_LatestAttestationServer creates a new mock instance
func NewMockBeaconService_LatestAttestationServer(ctrl *gomock.Controller) *MockBeaconService_LatestAttestationServer {
	mock := &MockBeaconService_LatestAttestationServer{ctrl: ctrl}
	mock.recorder = &MockBeaconService_LatestAttestationServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockBeaconService_LatestAttestationServer) EXPECT() *MockBeaconService_LatestAttestationServerMockRecorder {
	return m.recorder
}

// Context mocks base method
func (m *MockBeaconService_LatestAttestationServer) Context() context.Context {
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context
func (mr *MockBeaconService_LatestAttestationServerMockRecorder) Context() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockBeaconService_LatestAttestationServer)(nil).Context))
}

// RecvMsg mocks base method
func (m *MockBeaconService_LatestAttestationServer) RecvMsg(arg0 interface{}) error {
	ret := m.ctrl.Call(m, "RecvMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg
func (mr *MockBeaconService_LatestAttestationServerMockRecorder) RecvMsg(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockBeaconService_LatestAttestationServer)(nil).RecvMsg), arg0)
}

// Send mocks base method
func (m *MockBeaconService_LatestAttestationServer) Send(arg0 *v1.AggregatedAttestation) error {
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send
func (mr *MockBeaconService_LatestAttestationServerMockRecorder) Send(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockBeaconService_LatestAttestationServer)(nil).Send), arg0)
}

// SendHeader mocks base method
func (m *MockBeaconService_LatestAttestationServer) SendHeader(arg0 metadata.MD) error {
	ret := m.ctrl.Call(m, "SendHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendHeader indicates an expected call of SendHeader
func (mr *MockBeaconService_LatestAttestationServerMockRecorder) SendHeader(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendHeader", reflect.TypeOf((*MockBeaconService_LatestAttestationServer)(nil).SendHeader), arg0)
}

// SendMsg mocks base method
func (m *MockBeaconService_LatestAttestationServer) SendMsg(arg0 interface{}) error {
	ret := m.ctrl.Call(m, "SendMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg
func (mr *MockBeaconService_LatestAttestationServerMockRecorder) SendMsg(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockBeaconService_LatestAttestationServer)(nil).SendMsg), arg0)
}

// SetHeader mocks base method
func (m *MockBeaconService_LatestAttestationServer) SetHeader(arg0 metadata.MD) error {
	ret := m.ctrl.Call(m, "SetHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetHeader indicates an expected call of SetHeader
func (mr *MockBeaconService_LatestAttestationServerMockRecorder) SetHeader(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetHeader", reflect.TypeOf((*MockBeaconService_LatestAttestationServer)(nil).SetHeader), arg0)
}

// SetTrailer mocks base method
func (m *MockBeaconService_LatestAttestationServer) SetTrailer(arg0 metadata.MD) {
	m.ctrl.Call(m, "SetTrailer", arg0)
}

// SetTrailer indicates an expected call of SetTrailer
func (mr *MockBeaconService_LatestAttestationServerMockRecorder) SetTrailer(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTrailer", reflect.TypeOf((*MockBeaconService_LatestAttestationServer)(nil).SetTrailer), arg0)
}

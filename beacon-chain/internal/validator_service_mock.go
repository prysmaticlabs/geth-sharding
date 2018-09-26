// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1 (interfaces: ValidatorServiceServer,ValidatorService_ValidatorAssignmentServer)

package internal

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	empty "github.com/golang/protobuf/ptypes/empty"
	v1 "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	metadata "google.golang.org/grpc/metadata"
)

// MockValidatorServiceServer is a mock of ValidatorServiceServer interface
type MockValidatorServiceServer struct {
	ctrl     *gomock.Controller
	recorder *MockValidatorServiceServerMockRecorder
}

// MockValidatorServiceServerMockRecorder is the mock recorder for MockValidatorServiceServer
type MockValidatorServiceServerMockRecorder struct {
	mock *MockValidatorServiceServer
}

// NewMockValidatorServiceServer creates a new mock instance
func NewMockValidatorServiceServer(ctrl *gomock.Controller) *MockValidatorServiceServer {
	mock := &MockValidatorServiceServer{ctrl: ctrl}
	mock.recorder = &MockValidatorServiceServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockValidatorServiceServer) EXPECT() *MockValidatorServiceServerMockRecorder {
	return m.recorder
}

// CurrentAssignmentsAndGenesisTime mocks base method
func (m *MockValidatorServiceServer) CurrentAssignmentsAndGenesisTime(arg0 context.Context, arg1 *empty.Empty) (*v1.CurrentAssignmentsResponse, error) {
	ret := m.ctrl.Call(m, "CurrentAssignmentsAndGenesisTime", arg0, arg1)
	ret0, _ := ret[0].(*v1.CurrentAssignmentsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CurrentAssignmentsAndGenesisTime indicates an expected call of CurrentAssignmentsAndGenesisTime
func (mr *MockValidatorServiceServerMockRecorder) CurrentAssignmentsAndGenesisTime(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CurrentAssignmentsAndGenesisTime", reflect.TypeOf((*MockValidatorServiceServer)(nil).CurrentAssignmentsAndGenesisTime), arg0, arg1)
}

// ValidatorAssignment mocks base method
func (m *MockValidatorServiceServer) ValidatorAssignment(arg0 *v1.ValidatorAssignmentRequest, arg1 v1.ValidatorService_ValidatorAssignmentServer) error {
	ret := m.ctrl.Call(m, "ValidatorAssignment", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// ValidatorAssignment indicates an expected call of ValidatorAssignment
func (mr *MockValidatorServiceServerMockRecorder) ValidatorAssignment(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidatorAssignment", reflect.TypeOf((*MockValidatorServiceServer)(nil).ValidatorAssignment), arg0, arg1)
}

// ValidatorIndex mocks base method
func (m *MockValidatorServiceServer) ValidatorIndex(arg0 context.Context, arg1 *v1.PublicKey) (*v1.IndexResponse, error) {
	ret := m.ctrl.Call(m, "ValidatorIndex", arg0, arg1)
	ret0, _ := ret[0].(*v1.IndexResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ValidatorIndex indicates an expected call of ValidatorIndex
func (mr *MockValidatorServiceServerMockRecorder) ValidatorIndex(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidatorIndex", reflect.TypeOf((*MockValidatorServiceServer)(nil).ValidatorIndex), arg0, arg1)
}

// ValidatorShardID mocks base method
func (m *MockValidatorServiceServer) ValidatorShardID(arg0 context.Context, arg1 *v1.PublicKey) (*v1.ShardIDResponse, error) {
	ret := m.ctrl.Call(m, "ValidatorShardID", arg0, arg1)
	ret0, _ := ret[0].(*v1.ShardIDResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ValidatorShardID indicates an expected call of ValidatorShardID
func (mr *MockValidatorServiceServerMockRecorder) ValidatorShardID(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidatorShardID", reflect.TypeOf((*MockValidatorServiceServer)(nil).ValidatorShardID), arg0, arg1)
}

// ValidatorSlotAndResponsibility mocks base method
func (m *MockValidatorServiceServer) ValidatorSlotAndResponsibility(arg0 context.Context, arg1 *v1.PublicKey) (*v1.SlotResponsibilityResponse, error) {
	ret := m.ctrl.Call(m, "ValidatorSlotAndResponsibility", arg0, arg1)
	ret0, _ := ret[0].(*v1.SlotResponsibilityResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ValidatorSlotAndResponsibility indicates an expected call of ValidatorSlotAndResponsibility
func (mr *MockValidatorServiceServerMockRecorder) ValidatorSlotAndResponsibility(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidatorSlotAndResponsibility", reflect.TypeOf((*MockValidatorServiceServer)(nil).ValidatorSlotAndResponsibility), arg0, arg1)
}

// MockValidatorService_ValidatorAssignmentServer is a mock of ValidatorService_ValidatorAssignmentServer interface
type MockValidatorService_ValidatorAssignmentServer struct {
	ctrl     *gomock.Controller
	recorder *MockValidatorService_ValidatorAssignmentServerMockRecorder
}

// MockValidatorService_ValidatorAssignmentServerMockRecorder is the mock recorder for MockValidatorService_ValidatorAssignmentServer
type MockValidatorService_ValidatorAssignmentServerMockRecorder struct {
	mock *MockValidatorService_ValidatorAssignmentServer
}

// NewMockValidatorService_ValidatorAssignmentServer creates a new mock instance
func NewMockValidatorService_ValidatorAssignmentServer(ctrl *gomock.Controller) *MockValidatorService_ValidatorAssignmentServer {
	mock := &MockValidatorService_ValidatorAssignmentServer{ctrl: ctrl}
	mock.recorder = &MockValidatorService_ValidatorAssignmentServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockValidatorService_ValidatorAssignmentServer) EXPECT() *MockValidatorService_ValidatorAssignmentServerMockRecorder {
	return m.recorder
}

// Context mocks base method
func (m *MockValidatorService_ValidatorAssignmentServer) Context() context.Context {
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context
func (mr *MockValidatorService_ValidatorAssignmentServerMockRecorder) Context() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockValidatorService_ValidatorAssignmentServer)(nil).Context))
}

// RecvMsg mocks base method
func (m *MockValidatorService_ValidatorAssignmentServer) RecvMsg(arg0 interface{}) error {
	ret := m.ctrl.Call(m, "RecvMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg
func (mr *MockValidatorService_ValidatorAssignmentServerMockRecorder) RecvMsg(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockValidatorService_ValidatorAssignmentServer)(nil).RecvMsg), arg0)
}

// Send mocks base method
func (m *MockValidatorService_ValidatorAssignmentServer) Send(arg0 *v1.ValidatorAssignmentResponse) error {
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send
func (mr *MockValidatorService_ValidatorAssignmentServerMockRecorder) Send(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockValidatorService_ValidatorAssignmentServer)(nil).Send), arg0)
}

// SendHeader mocks base method
func (m *MockValidatorService_ValidatorAssignmentServer) SendHeader(arg0 metadata.MD) error {
	ret := m.ctrl.Call(m, "SendHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendHeader indicates an expected call of SendHeader
func (mr *MockValidatorService_ValidatorAssignmentServerMockRecorder) SendHeader(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendHeader", reflect.TypeOf((*MockValidatorService_ValidatorAssignmentServer)(nil).SendHeader), arg0)
}

// SendMsg mocks base method
func (m *MockValidatorService_ValidatorAssignmentServer) SendMsg(arg0 interface{}) error {
	ret := m.ctrl.Call(m, "SendMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg
func (mr *MockValidatorService_ValidatorAssignmentServerMockRecorder) SendMsg(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockValidatorService_ValidatorAssignmentServer)(nil).SendMsg), arg0)
}

// SetHeader mocks base method
func (m *MockValidatorService_ValidatorAssignmentServer) SetHeader(arg0 metadata.MD) error {
	ret := m.ctrl.Call(m, "SetHeader", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetHeader indicates an expected call of SetHeader
func (mr *MockValidatorService_ValidatorAssignmentServerMockRecorder) SetHeader(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetHeader", reflect.TypeOf((*MockValidatorService_ValidatorAssignmentServer)(nil).SetHeader), arg0)
}

// SetTrailer mocks base method
func (m *MockValidatorService_ValidatorAssignmentServer) SetTrailer(arg0 metadata.MD) {
	m.ctrl.Call(m, "SetTrailer", arg0)
}

// SetTrailer indicates an expected call of SetTrailer
func (mr *MockValidatorService_ValidatorAssignmentServerMockRecorder) SetTrailer(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTrailer", reflect.TypeOf((*MockValidatorService_ValidatorAssignmentServer)(nil).SetTrailer), arg0)
}

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1 (interfaces: ValidatorServiceClient)

// Package internal is a generated GoMock package.
package internal

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	v1 "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	grpc "google.golang.org/grpc"
)

// MockValidatorServiceClient is a mock of ValidatorServiceClient interface
type MockValidatorServiceClient struct {
	ctrl     *gomock.Controller
	recorder *MockValidatorServiceClientMockRecorder
}

// MockValidatorServiceClientMockRecorder is the mock recorder for MockValidatorServiceClient
type MockValidatorServiceClientMockRecorder struct {
	mock *MockValidatorServiceClient
}

// NewMockValidatorServiceClient creates a new mock instance
func NewMockValidatorServiceClient(ctrl *gomock.Controller) *MockValidatorServiceClient {
	mock := &MockValidatorServiceClient{ctrl: ctrl}
	mock.recorder = &MockValidatorServiceClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockValidatorServiceClient) EXPECT() *MockValidatorServiceClientMockRecorder {
	return m.recorder
}

// CommitteeAssignment mocks base method
func (m *MockValidatorServiceClient) CommitteeAssignment(arg0 context.Context, arg1 *v1.ValidatorEpochAssignmentsRequest, arg2 ...grpc.CallOption) (*v1.CommitteeAssignmentResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CommitteeAssignment", varargs...)
	ret0, _ := ret[0].(*v1.CommitteeAssignmentResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CommitteeAssignment indicates an expected call of CommitteeAssignment
func (mr *MockValidatorServiceClientMockRecorder) CommitteeAssignment(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitteeAssignment", reflect.TypeOf((*MockValidatorServiceClient)(nil).CommitteeAssignment), varargs...)
}

// ValidatorIndex mocks base method
func (m *MockValidatorServiceClient) ValidatorIndex(arg0 context.Context, arg1 *v1.ValidatorIndexRequest, arg2 ...grpc.CallOption) (*v1.ValidatorIndexResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ValidatorIndex", varargs...)
	ret0, _ := ret[0].(*v1.ValidatorIndexResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ValidatorIndex indicates an expected call of ValidatorIndex
func (mr *MockValidatorServiceClientMockRecorder) ValidatorIndex(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidatorIndex", reflect.TypeOf((*MockValidatorServiceClient)(nil).ValidatorIndex), varargs...)
}

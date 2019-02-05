// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1 (interfaces: AttesterServiceClient)

package internal

import (
	context "context"
	gomock "github.com/golang/mock/gomock"
	v1 "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	grpc "google.golang.org/grpc"
	reflect "reflect"
)

// MockAttesterServiceClient is a mock of AttesterServiceClient interface
type MockAttesterServiceClient struct {
	ctrl     *gomock.Controller
	recorder *MockAttesterServiceClientMockRecorder
}

// MockAttesterServiceClientMockRecorder is the mock recorder for MockAttesterServiceClient
type MockAttesterServiceClientMockRecorder struct {
	mock *MockAttesterServiceClient
}

// NewMockAttesterServiceClient creates a new mock instance
func NewMockAttesterServiceClient(ctrl *gomock.Controller) *MockAttesterServiceClient {
	mock := &MockAttesterServiceClient{ctrl: ctrl}
	mock.recorder = &MockAttesterServiceClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockAttesterServiceClient) EXPECT() *MockAttesterServiceClientMockRecorder {
	return m.recorder
}

// AttestHead mocks base method
func (m *MockAttesterServiceClient) AttestHead(arg0 context.Context, arg1 *v1.AttestRequest, arg2 ...grpc.CallOption) (*v1.AttestResponse, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "AttestHead", varargs...)
	ret0, _ := ret[0].(*v1.AttestResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AttestHead indicates an expected call of AttestHead
func (mr *MockAttesterServiceClientMockRecorder) AttestHead(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AttestHead", reflect.TypeOf((*MockAttesterServiceClient)(nil).AttestHead), varargs...)
}

// AttestationInfoAtSlot mocks base method
func (m *MockAttesterServiceClient) AttestationInfoAtSlot(arg0 context.Context, arg1 *v1.AttestationInfoRequest, arg2 ...grpc.CallOption) (*v1.AttestationInfoResponse, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "AttestationInfoAtSlot", varargs...)
	ret0, _ := ret[0].(*v1.AttestationInfoResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AttestationInfoAtSlot indicates an expected call of AttestationInfoAtSlot
func (mr *MockAttesterServiceClientMockRecorder) AttestationInfoAtSlot(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AttestationInfoAtSlot", reflect.TypeOf((*MockAttesterServiceClient)(nil).AttestationInfoAtSlot), varargs...)
}

// CrosslinkCommitteesAtSlot mocks base method
func (m *MockAttesterServiceClient) CrosslinkCommitteesAtSlot(arg0 context.Context, arg1 *v1.CrosslinkCommitteeRequest, arg2 ...grpc.CallOption) (*v1.CrosslinkCommitteeResponse, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "CrosslinkCommitteesAtSlot", varargs...)
	ret0, _ := ret[0].(*v1.CrosslinkCommitteeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CrosslinkCommitteesAtSlot indicates an expected call of CrosslinkCommitteesAtSlot
func (mr *MockAttesterServiceClientMockRecorder) CrosslinkCommitteesAtSlot(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CrosslinkCommitteesAtSlot", reflect.TypeOf((*MockAttesterServiceClient)(nil).CrosslinkCommitteesAtSlot), varargs...)
}

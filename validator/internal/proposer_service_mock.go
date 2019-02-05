// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1 (interfaces: ProposerServiceClient)

// Package internal is a generated GoMock package.
package internal

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	v1 "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	v10 "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	grpc "google.golang.org/grpc"
)

// MockProposerServiceClient is a mock of ProposerServiceClient interface
type MockProposerServiceClient struct {
	ctrl     *gomock.Controller
	recorder *MockProposerServiceClientMockRecorder
}

// MockProposerServiceClientMockRecorder is the mock recorder for MockProposerServiceClient
type MockProposerServiceClientMockRecorder struct {
	mock *MockProposerServiceClient
}

// NewMockProposerServiceClient creates a new mock instance
func NewMockProposerServiceClient(ctrl *gomock.Controller) *MockProposerServiceClient {
	mock := &MockProposerServiceClient{ctrl: ctrl}
	mock.recorder = &MockProposerServiceClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockProposerServiceClient) EXPECT() *MockProposerServiceClientMockRecorder {
	return m.recorder
}

// ComputeStateRoot mocks base method
func (m *MockProposerServiceClient) ComputeStateRoot(arg0 context.Context, arg1 *v1.BeaconBlock, arg2 ...grpc.CallOption) (*v10.StateRootResponse, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ComputeStateRoot", varargs...)
	ret0, _ := ret[0].(*v10.StateRootResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ComputeStateRoot indicates an expected call of ComputeStateRoot
func (mr *MockProposerServiceClientMockRecorder) ComputeStateRoot(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ComputeStateRoot", reflect.TypeOf((*MockProposerServiceClient)(nil).ComputeStateRoot), varargs...)
}

// ProposeBlock mocks base method
func (m *MockProposerServiceClient) ProposeBlock(arg0 context.Context, arg1 *v1.BeaconBlock, arg2 ...grpc.CallOption) (*v10.ProposeResponse, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ProposeBlock", varargs...)
	ret0, _ := ret[0].(*v10.ProposeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ProposeBlock indicates an expected call of ProposeBlock
func (mr *MockProposerServiceClientMockRecorder) ProposeBlock(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ProposeBlock", reflect.TypeOf((*MockProposerServiceClient)(nil).ProposeBlock), varargs...)
}

// ProposerIndex mocks base method
func (m *MockProposerServiceClient) ProposerIndex(arg0 context.Context, arg1 *v10.ProposerIndexRequest, arg2 ...grpc.CallOption) (*v10.ProposerIndexResponse, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ProposerIndex", varargs...)
	ret0, _ := ret[0].(*v10.ProposerIndexResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ProposerIndex indicates an expected call of ProposerIndex
func (mr *MockProposerServiceClientMockRecorder) ProposerIndex(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ProposerIndex", reflect.TypeOf((*MockProposerServiceClient)(nil).ProposerIndex), varargs...)
}

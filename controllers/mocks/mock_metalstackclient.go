// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/metal-stack/cluster-api-provider-metalstack/controllers (interfaces: MetalStackClient)

// Package mocks is a generated GoMock package.
package mocks

import (
	gomock "github.com/golang/mock/gomock"
	metalgo "github.com/metal-stack/metal-go"
	reflect "reflect"
)

// MockMetalStackClient is a mock of MetalStackClient interface
type MockMetalStackClient struct {
	ctrl     *gomock.Controller
	recorder *MockMetalStackClientMockRecorder
}

// MockMetalStackClientMockRecorder is the mock recorder for MockMetalStackClient
type MockMetalStackClientMockRecorder struct {
	mock *MockMetalStackClient
}

// NewMockMetalStackClient creates a new mock instance
func NewMockMetalStackClient(ctrl *gomock.Controller) *MockMetalStackClient {
	mock := &MockMetalStackClient{ctrl: ctrl}
	mock.recorder = &MockMetalStackClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockMetalStackClient) EXPECT() *MockMetalStackClientMockRecorder {
	return m.recorder
}

// FirewallCreate mocks base method
func (m *MockMetalStackClient) FirewallCreate(arg0 *metalgo.FirewallCreateRequest) (*metalgo.FirewallCreateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FirewallCreate", arg0)
	ret0, _ := ret[0].(*metalgo.FirewallCreateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FirewallCreate indicates an expected call of FirewallCreate
func (mr *MockMetalStackClientMockRecorder) FirewallCreate(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FirewallCreate", reflect.TypeOf((*MockMetalStackClient)(nil).FirewallCreate), arg0)
}

// IPAllocate mocks base method
func (m *MockMetalStackClient) IPAllocate(arg0 *metalgo.IPAllocateRequest) (*metalgo.IPDetailResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IPAllocate", arg0)
	ret0, _ := ret[0].(*metalgo.IPDetailResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IPAllocate indicates an expected call of IPAllocate
func (mr *MockMetalStackClientMockRecorder) IPAllocate(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IPAllocate", reflect.TypeOf((*MockMetalStackClient)(nil).IPAllocate), arg0)
}

// MachineCreate mocks base method
func (m *MockMetalStackClient) MachineCreate(arg0 *metalgo.MachineCreateRequest) (*metalgo.MachineCreateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MachineCreate", arg0)
	ret0, _ := ret[0].(*metalgo.MachineCreateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MachineCreate indicates an expected call of MachineCreate
func (mr *MockMetalStackClientMockRecorder) MachineCreate(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MachineCreate", reflect.TypeOf((*MockMetalStackClient)(nil).MachineCreate), arg0)
}

// MachineDelete mocks base method
func (m *MockMetalStackClient) MachineDelete(arg0 string) (*metalgo.MachineDeleteResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MachineDelete", arg0)
	ret0, _ := ret[0].(*metalgo.MachineDeleteResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MachineDelete indicates an expected call of MachineDelete
func (mr *MockMetalStackClientMockRecorder) MachineDelete(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MachineDelete", reflect.TypeOf((*MockMetalStackClient)(nil).MachineDelete), arg0)
}

// MachineFind mocks base method
func (m *MockMetalStackClient) MachineFind(arg0 *metalgo.MachineFindRequest) (*metalgo.MachineListResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MachineFind", arg0)
	ret0, _ := ret[0].(*metalgo.MachineListResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MachineFind indicates an expected call of MachineFind
func (mr *MockMetalStackClientMockRecorder) MachineFind(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MachineFind", reflect.TypeOf((*MockMetalStackClient)(nil).MachineFind), arg0)
}

// MachineGet mocks base method
func (m *MockMetalStackClient) MachineGet(arg0 string) (*metalgo.MachineGetResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MachineGet", arg0)
	ret0, _ := ret[0].(*metalgo.MachineGetResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MachineGet indicates an expected call of MachineGet
func (mr *MockMetalStackClientMockRecorder) MachineGet(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MachineGet", reflect.TypeOf((*MockMetalStackClient)(nil).MachineGet), arg0)
}

// NetworkAllocate mocks base method
func (m *MockMetalStackClient) NetworkAllocate(arg0 *metalgo.NetworkAllocateRequest) (*metalgo.NetworkDetailResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NetworkAllocate", arg0)
	ret0, _ := ret[0].(*metalgo.NetworkDetailResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NetworkAllocate indicates an expected call of NetworkAllocate
func (mr *MockMetalStackClientMockRecorder) NetworkAllocate(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NetworkAllocate", reflect.TypeOf((*MockMetalStackClient)(nil).NetworkAllocate), arg0)
}

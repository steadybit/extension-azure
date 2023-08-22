package extvm

import (
	"context"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAzureVirtualMachineStateAction_Prepare(t *testing.T) {
	action := virtualMachineStateAction{}

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError error
		wantedState *VirtualMachineStateChangeState
	}{
		{
			name: "Should return config",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "power-off",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"azure-vm.vm.name":             {"my-vm"},
						"azure-vm.subscription.id":     {"42"},
						"azure-vm.resource-group.name": {"rg0815"},
					},
				}),
			}),

			wantedState: &VirtualMachineStateChangeState{
				VmName:            "my-vm",
				Action:            "power-off",
				SubscriptionId:    "42",
				ResourceGroupName: "rg0815",
			},
		},
		{
			name: "Should return error if subscription is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "power-off",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"azure-vm.vm.name":             {"my-vm"},
						"azure-vm.resource-group.name": {"rg0815"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Target is missing the 'azure-vm.subscription.id' attribute.", nil),
		},
		{
			name: "Should return error if vm name is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "power-off",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"azure-vm.subscription.id":     {"42"},
						"azure-vm.resource-group.name": {"rg0815"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Target is missing the 'azure-vm.vm.name' attribute.", nil),
		},
		{
			name: "Should return error if resource-group is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "power-off",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"azure-vm.vm.name":         {"my-vm"},
						"azure-vm.subscription.id": {"42"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Target is missing the 'azure-vm.resource-group.name' attribute.", nil),
		},
		{
			name: "Should return error if action is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"azure-vm.vm.name":             {"my-vm"},
						"azure-vm.subscription.id":     {"42"},
						"azure-vm.resource-group.name": {"rg0815"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Missing attack action parameter.", nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//Given
			state := action.NewEmptyState()
			request := tt.requestBody
			//When
			_, err := action.Prepare(context.Background(), &state, request)

			//Then
			if tt.wantedError != nil {
				assert.EqualError(t, err, tt.wantedError.Error())
			}
			if tt.wantedState != nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantedState.ResourceGroupName, state.ResourceGroupName)
				assert.Equal(t, tt.wantedState.VmName, state.VmName)
				assert.Equal(t, tt.wantedState.SubscriptionId, state.SubscriptionId)
				assert.EqualValues(t, tt.wantedState.Action, state.Action)
			}
		})
	}
}

type ec2ClientApiMock struct {
	mock.Mock
}

func (m *ec2ClientApiMock) BeginStart(ctx context.Context, resourceGroupName string, vmName string, _ *armcompute.VirtualMachinesClientBeginStartOptions) (*runtime.Poller[armcompute.VirtualMachinesClientStartResponse], error) {
	args := m.Called(ctx, resourceGroupName, vmName)
	return nil, args.Error(1)
}

func (m *ec2ClientApiMock) BeginRestart(ctx context.Context, resourceGroupName string, vmName string, _ *armcompute.VirtualMachinesClientBeginRestartOptions) (*runtime.Poller[armcompute.VirtualMachinesClientRestartResponse], error) {
	args := m.Called(ctx, resourceGroupName, vmName)
	return nil, args.Error(1)
}

func (m *ec2ClientApiMock) BeginDelete(ctx context.Context, resourceGroupName string, vmName string, _ *armcompute.VirtualMachinesClientBeginDeleteOptions) (*runtime.Poller[armcompute.VirtualMachinesClientDeleteResponse], error) {
	args := m.Called(ctx, resourceGroupName, vmName)
	return nil, args.Error(1)
}

func (m *ec2ClientApiMock) BeginPowerOff(ctx context.Context, resourceGroupName string, vmName string, _ *armcompute.VirtualMachinesClientBeginPowerOffOptions) (*runtime.Poller[armcompute.VirtualMachinesClientPowerOffResponse], error) {
	args := m.Called(ctx, resourceGroupName, vmName)
	return nil, args.Error(1)
}
func (m *ec2ClientApiMock) BeginDeallocate(ctx context.Context, resourceGroupName string, vmName string, _ *armcompute.VirtualMachinesClientBeginDeallocateOptions) (*runtime.Poller[armcompute.VirtualMachinesClientDeallocateResponse], error) {
	args := m.Called(ctx, resourceGroupName, vmName)
	return nil, args.Error(1)
}

func TestAzureVirtualMachineStateAction_Start(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginStart", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmName string) bool {
		require.Equal(t, "my-vm", vmName)
		return true
	})).Return(nil, nil)

	action := virtualMachineStateAction{clientProvider: func(account string) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		SubscriptionId:    "42",
		VmName:            "my-vm",
		ResourceGroupName: "rg-42",
		Action:            "start",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestAzureVirtualMachineStateAction_ReStart(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginRestart", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmName string) bool {
		require.Equal(t, "my-vm", vmName)
		return true
	})).Return(nil, nil)

	action := virtualMachineStateAction{clientProvider: func(account string) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		SubscriptionId:    "42",
		VmName:            "my-vm",
		ResourceGroupName: "rg-42",
		Action:            "restart",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestAzureVirtualMachineStateAction_Delete(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginDelete", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmName string) bool {
		require.Equal(t, "my-vm", vmName)
		return true
	})).Return(nil, nil)

	action := virtualMachineStateAction{clientProvider: func(account string) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		SubscriptionId:    "42",
		VmName:            "my-vm",
		ResourceGroupName: "rg-42",
		Action:            "delete",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestAzureVirtualMachineStateAction_PowerOff(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginPowerOff", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmName string) bool {
		require.Equal(t, "my-vm", vmName)
		return true
	})).Return(nil, nil)

	action := virtualMachineStateAction{clientProvider: func(account string) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		SubscriptionId:    "42",
		VmName:            "my-vm",
		ResourceGroupName: "rg-42",
		Action:            "power-off",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestAzureVirtualMachineStateAction_Deallocate(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginDeallocate", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmName string) bool {
		require.Equal(t, "my-vm", vmName)
		return true
	})).Return(nil, nil)

	action := virtualMachineStateAction{clientProvider: func(account string) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		SubscriptionId:    "42",
		VmName:            "my-vm",
		ResourceGroupName: "rg-42",
		Action:            "deallocate",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestStartVirtualMachineStateChangeForwardsError(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginStart", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmName string) bool {
    require.Equal(t, "my-vm", vmName)
    return true
  })).Return(nil, errors.New("expected"))
	action := virtualMachineStateAction{clientProvider: func(account string) (virtualMachineStateChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &VirtualMachineStateChangeState{
		SubscriptionId:    "42",
		VmName:            "my-vm",
		ResourceGroupName: "rg-42",
		Action:            "start",
	})

	// Then
	assert.Error(t, err, "Failed to execute state change attack")
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

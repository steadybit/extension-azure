package extscalesetinstance

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

func TestAzureScaleSetInstanceAction_Prepare(t *testing.T) {
	action := scaleSetInstanceAction{}

	tests := []struct {
		name        string
		requestBody action_kit_api.PrepareActionRequestBody
		wantedError error
		wantedState *ScaleSetInstanceChangeState
	}{
		{
			name: "Should return config",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "power-off",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"azure-scaleset-instance.vm.name": {"my-vm"},
						"azure.subscription.id":           {"42"},
						"azure.resource-group.name":       {"rg0815"},
						"azure-scaleset-instance.id":      {"InstanceID0815"},
					},
				}),
			}),

			wantedState: &ScaleSetInstanceChangeState{
				InstanceName:      "my-vm",
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
						"azure-scaleset-instance.vm.name": {"my-vm"},
						"azure-scaleset-instance.id":      {"InstanceID0815"},
						"azure.resource-group.name":       {"rg0815"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Target is missing the 'azure.subscription.id' attribute.", nil),
		},
		{
			name: "Should return error if instanceId is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "power-off",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"azure-scaleset-instance.vm.name": {"my-vm"},
						"azure.resource-group.name":       {"rg0815"},
						"azure.subscription.id":           {"42"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Target is missing the 'azure-scaleset-instance.id' attribute.", nil),
		},
		{
			name: "Should return error if vm name is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "power-off",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"azure.subscription.id":      {"42"},
						"azure-scaleset-instance.id": {"InstanceID0815"},
						"azure.resource-group.name":  {"rg0815"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Target is missing the 'azure-scaleset-instance.vm.name' attribute.", nil),
		},
		{
			name: "Should return error if resource-group is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{
					"action": "power-off",
				},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"azure-scaleset-instance.vm.name": {"my-vm"},
						"azure-scaleset-instance.id":      {"InstanceID0815"},
						"azure.subscription.id":           {"42"},
					},
				}),
			}),
			wantedError: extension_kit.ToError("Target is missing the 'azure.resource-group.name' attribute.", nil),
		},
		{
			name: "Should return error if action is missing",
			requestBody: extutil.JsonMangle(action_kit_api.PrepareActionRequestBody{
				Config: map[string]interface{}{},
				Target: extutil.Ptr(action_kit_api.Target{
					Attributes: map[string][]string{
						"azure-scaleset-instance.vm.name": {"my-vm"},
						"azure-scaleset-instance.id":      {"InstanceID0815"},
						"azure.subscription.id":           {"42"},
						"azure.resource-group.name":       {"rg0815"},
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
				assert.Equal(t, tt.wantedState.InstanceName, state.InstanceName)
				assert.Equal(t, tt.wantedState.SubscriptionId, state.SubscriptionId)
				assert.EqualValues(t, tt.wantedState.Action, state.Action)
			}
		})
	}
}

type ec2ClientApiMock struct {
	mock.Mock
}

func (m *ec2ClientApiMock) BeginRestart(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, _ *armcompute.VirtualMachineScaleSetVMsClientBeginRestartOptions) (*runtime.Poller[armcompute.VirtualMachineScaleSetVMsClientRestartResponse], error) {
	args := m.Called(ctx, resourceGroupName, vmScaleSetName, instanceID)
	return nil, args.Error(1)
}

func (m *ec2ClientApiMock) BeginDelete(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, _ *armcompute.VirtualMachineScaleSetVMsClientBeginDeleteOptions) (*runtime.Poller[armcompute.VirtualMachineScaleSetVMsClientDeleteResponse], error) {
	args := m.Called(ctx, resourceGroupName, vmScaleSetName, instanceID)
	return nil, args.Error(1)
}

func (m *ec2ClientApiMock) BeginPowerOff(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, _ *armcompute.VirtualMachineScaleSetVMsClientBeginPowerOffOptions) (*runtime.Poller[armcompute.VirtualMachineScaleSetVMsClientPowerOffResponse], error) {
	args := m.Called(ctx, resourceGroupName, vmScaleSetName, instanceID)
	return nil, args.Error(1)
}
func (m *ec2ClientApiMock) BeginDeallocate(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, _ *armcompute.VirtualMachineScaleSetVMsClientBeginDeallocateOptions) (*runtime.Poller[armcompute.VirtualMachineScaleSetVMsClientDeallocateResponse], error) {
	args := m.Called(ctx, resourceGroupName, vmScaleSetName, instanceID)
	return nil, args.Error(1)
}

func TestAzureScaleSetInstanceAction_ReStart(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginRestart", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmScaleSetName string) bool {
		require.Equal(t, "my-vm", vmScaleSetName)
		return true
	}), mock.MatchedBy(func(instanceId string) bool {
		require.Equal(t, "InstanceID0815", instanceId)
		return true
	})).Return(nil, nil)

	action := scaleSetInstanceAction{clientProvider: func(account string) (scaleSetInstanceChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &ScaleSetInstanceChangeState{
		SubscriptionId:    "42",
		InstanceName:      "my-vm",
		InstanceID:        "InstanceID0815",
		ResourceGroupName: "rg-42",
		Action:            "restart",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestAzureScaleSetInstanceAction_Delete(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginDelete", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmScaleSetName string) bool {
		require.Equal(t, "my-vm", vmScaleSetName)
		return true
	}), mock.MatchedBy(func(instanceId string) bool {
		require.Equal(t, "InstanceID0815", instanceId)
		return true
	})).Return(nil, nil)

	action := scaleSetInstanceAction{clientProvider: func(account string) (scaleSetInstanceChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &ScaleSetInstanceChangeState{
		SubscriptionId:    "42",
		InstanceName:      "my-vm",
		InstanceID:        "InstanceID0815",
		ResourceGroupName: "rg-42",
		Action:            "delete",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestAzureScaleSetInstanceAction_PowerOff(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginPowerOff", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmScaleSetName string) bool {
		require.Equal(t, "my-vm", vmScaleSetName)
		return true
	}), mock.MatchedBy(func(instanceId string) bool {
		require.Equal(t, "InstanceID0815", instanceId)
		return true
	})).Return(nil, nil)

	action := scaleSetInstanceAction{clientProvider: func(account string) (scaleSetInstanceChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &ScaleSetInstanceChangeState{
		SubscriptionId:    "42",
		InstanceName:      "my-vm",
		InstanceID:        "InstanceID0815",
		ResourceGroupName: "rg-42",
		Action:            "power-off",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestAzureScaleSetInstanceAction_Deallocate(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginDeallocate", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmScaleSetName string) bool {
		require.Equal(t, "my-vm", vmScaleSetName)
		return true
	}), mock.MatchedBy(func(instanceId string) bool {
		require.Equal(t, "InstanceID0815", instanceId)
		return true
	})).Return(nil, nil)

	action := scaleSetInstanceAction{clientProvider: func(account string) (scaleSetInstanceChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &ScaleSetInstanceChangeState{
		SubscriptionId:    "42",
		InstanceName:      "my-vm",
		InstanceID:        "InstanceID0815",
		ResourceGroupName: "rg-42",
		Action:            "deallocate",
	})

	// Then
	assert.NoError(t, err)
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

func TestStartScaleSetInstanceChangeForwardsError(t *testing.T) {
	// Given
	api := new(ec2ClientApiMock)
	api.On("BeginRestart", mock.Anything, mock.MatchedBy(func(resourceGroupName string) bool {
		require.Equal(t, "rg-42", resourceGroupName)
		return true
	}), mock.MatchedBy(func(vmScaleSetName string) bool {
		require.Equal(t, "my-vm", vmScaleSetName)
		return true
	}), mock.MatchedBy(func(instanceId string) bool {
		require.Equal(t, "InstanceID0815", instanceId)
		return true
	})).Return(nil, errors.New("expected"))
	action := scaleSetInstanceAction{clientProvider: func(account string) (scaleSetInstanceChangeApi, error) {
		return api, nil
	}}

	// When
	result, err := action.Start(context.Background(), &ScaleSetInstanceChangeState{
		SubscriptionId:    "42",
		InstanceName:      "my-vm",
		InstanceID:        "InstanceID0815",
		ResourceGroupName: "rg-42",
		Action:            "restart",
	})

	// Then
	assert.Error(t, err, "Failed to execute state change attack")
	assert.Nil(t, result)

	api.AssertExpectations(t)
}

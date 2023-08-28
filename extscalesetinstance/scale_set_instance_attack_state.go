/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extscalesetinstance

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type scaleSetInstanceAction struct {
	clientProvider func(account string) (scaleSetInstanceChangeApi, error)
}

// Make sure lambdaAction implements all required interfaces
var _ action_kit_sdk.Action[ScaleSetInstanceChangeState] = (*scaleSetInstanceAction)(nil)

type ScaleSetInstanceChangeState struct {
	SubscriptionId string
  VmScaleSetName string
	InstanceID     string
	ResourceGroupName string
	Action            string
}

type scaleSetInstanceChangeApi interface {
  BeginRestart(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, options *armcompute.VirtualMachineScaleSetVMsClientBeginRestartOptions) (*runtime.Poller[armcompute.VirtualMachineScaleSetVMsClientRestartResponse], error)
  BeginDelete(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, options *armcompute.VirtualMachineScaleSetVMsClientBeginDeleteOptions) (*runtime.Poller[armcompute.VirtualMachineScaleSetVMsClientDeleteResponse], error)
  BeginPowerOff(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, options *armcompute.VirtualMachineScaleSetVMsClientBeginPowerOffOptions) (*runtime.Poller[armcompute.VirtualMachineScaleSetVMsClientPowerOffResponse], error)
  BeginDeallocate(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, options *armcompute.VirtualMachineScaleSetVMsClientBeginDeallocateOptions) (*runtime.Poller[armcompute.VirtualMachineScaleSetVMsClientDeallocateResponse], error)
}

func NewScaleSetInstanceStateAction() action_kit_sdk.Action[ScaleSetInstanceChangeState] {
	return &scaleSetInstanceAction{defaultClientProvider}
}

func (e *scaleSetInstanceAction) NewEmptyState() ScaleSetInstanceChangeState {
	return ScaleSetInstanceChangeState{}
}

func (e *scaleSetInstanceAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          ScaleSetInstanceStateActionId,
		Label:       "Change Virtual Machine State",
		Description: "Restart, start, stop, deallocate or delete Azure scale set instances",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDScaleSetInstance,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by cluster name",
					Description: extutil.Ptr("Find azure scale set instance by cluster name"),
					Query:       "azure-containerservice-managed-cluster.name=\"\"",
				},
        {
					Label:       "by instance name",
					Description: extutil.Ptr("Find azure scale set instance by name"),
					Query:       "azure-scale-set-instance.name=\"\"",
				},
        {
          Label:       "by scaleset name",
          Description: extutil.Ptr("Find azure scale set instance by scale set name"),
          Query:       "azure-scale-set.name=\"\"",
        },
			}),
		}),
		Category:    extutil.Ptr("state"),
		TimeControl: action_kit_api.TimeControlInstantaneous,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:        "action",
				Label:       "Action",
				Description: extutil.Ptr("The kind of state change operation to execute for the azure scale set instances"),
				Required:    extutil.Ptr(true),
				Type:        action_kit_api.String,
				Options: extutil.Ptr([]action_kit_api.ParameterOption{
					action_kit_api.ExplicitParameterOption{
						Label: "Restart",
						Value: "restart",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Power Off",
						Value: "power-off",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Delete",
						Value: "delete",
					},
					action_kit_api.ExplicitParameterOption{
						Label: "Deallocate",
						Value: "deallocate",
					},
				}),
			},
		},
	}
}

func (e *scaleSetInstanceAction) Prepare(_ context.Context, state *ScaleSetInstanceChangeState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	vmScaleSetName := request.Target.Attributes["azure-scale-set.name"]
	if len(vmScaleSetName) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'azure-scaleset.name' attribute.", nil)
	}

  instanceId := request.Target.Attributes["azure-scale-set-instance.id"]
	if len(instanceId) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'azure-scaleset-instance.id' attribute.", nil)
	}

	subscriptionId := request.Target.Attributes["azure.subscription.id"]
	if len(subscriptionId) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'azure.subscription.id' attribute.", nil)
	}

	resourceGroupName := request.Target.Attributes["azure.resource-group.name"]
	if len(resourceGroupName) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'azure.resource-group.name' attribute.", nil)
	}

	action := request.Config["action"]
	if action == nil {
		return nil, extension_kit.ToError("Missing attack action parameter.", nil)
	}

	state.SubscriptionId = subscriptionId[0]
	state.VmScaleSetName = vmScaleSetName[0]
	state.InstanceID = instanceId[0]
	state.ResourceGroupName = resourceGroupName[0]
	state.Action = action.(string)
	return nil, nil
}

func (e *scaleSetInstanceAction) Start(ctx context.Context, state *ScaleSetInstanceChangeState) (*action_kit_api.StartResult, error) {
	client, err := e.clientProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize azure client for subscription %s", state.SubscriptionId), err)
	}

	if state.Action == "restart" {
		_, err = client.BeginRestart(ctx, state.ResourceGroupName, state.VmScaleSetName,state.InstanceID, nil)
	} else if state.Action == "power-off" {
		_, err = client.BeginPowerOff(ctx, state.ResourceGroupName, state.VmScaleSetName,state.InstanceID, nil)
	} else if state.Action == "delete" {
		_, err = client.BeginDelete(ctx, state.ResourceGroupName, state.VmScaleSetName, state.InstanceID,nil)
	} else if state.Action == "deallocate" {
		_, err = client.BeginDeallocate(ctx, state.ResourceGroupName, state.VmScaleSetName,state.InstanceID, nil)
	} else {
		return nil, extension_kit.ToError(fmt.Sprintf("Unknown state change attack '%s'", state.Action), nil)
	}

	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to execute state change attack '%s' on vm '%s'", state.Action, state.VmScaleSetName), err)
	}

	return nil, nil
}

func defaultClientProvider(subscriptionId string) (scaleSetInstanceChangeApi, error) {
	return common.GetVirtualMachineScaleSetVMsClient(subscriptionId)
}

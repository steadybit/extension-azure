/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extvm

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/utils"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

type virtualMachineStateAction struct {
	clientProvider func(account string) (virtualMachineStateChangeApi, error)
}

// Make sure lambdaAction implements all required interfaces
var _ action_kit_sdk.Action[VirtualMachineStateChangeState] = (*virtualMachineStateAction)(nil)

type VirtualMachineStateChangeState struct {
	SubscriptionId    string
	VmName            string
	ResourceGroupName string
	Action            string
}

type virtualMachineStateChangeApi interface {
	BeginStart(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientBeginStartOptions) (*runtime.Poller[armcompute.VirtualMachinesClientStartResponse], error)
	BeginRestart(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientBeginRestartOptions) (*runtime.Poller[armcompute.VirtualMachinesClientRestartResponse], error)
	BeginDelete(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientBeginDeleteOptions) (*runtime.Poller[armcompute.VirtualMachinesClientDeleteResponse], error)
	BeginPowerOff(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientBeginPowerOffOptions) (*runtime.Poller[armcompute.VirtualMachinesClientPowerOffResponse], error)
	BeginDeallocate(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientBeginDeallocateOptions) (*runtime.Poller[armcompute.VirtualMachinesClientDeallocateResponse], error)
}

func NewVirtualMachineStateAction() action_kit_sdk.Action[VirtualMachineStateChangeState] {
	return &virtualMachineStateAction{defaultClientProvider}
}

func (e *virtualMachineStateAction) NewEmptyState() VirtualMachineStateChangeState {
	return VirtualMachineStateChangeState{}
}

func (e *virtualMachineStateAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          VirtualMachineStateActionId,
		Label:       "Change Virtual Machine State",
		Description: "Restart, start, stop, deallocate or delete Azure virtual machines",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDVM,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by vm-id",
					Description: extutil.Ptr("Find azure virtual machine by vm-id"),
					Query:       "azure-vm.vm.id=\"\"",
				},
				{
					Label:       "by vm-name",
					Description: extutil.Ptr("Find azure virtual machine by name"),
					Query:       "azure-vm.vm.name=\"\"",
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
				Description: extutil.Ptr("The kind of state change operation to execute for the azure virtual machines"),
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
						Label: "Start",
						Value: "start",
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

func (e *virtualMachineStateAction) Prepare(_ context.Context, state *VirtualMachineStateChangeState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	vmName := request.Target.Attributes["azure-vm.vm.name"]
	if len(vmName) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'azure-vm.vm.name' attribute.", nil)
	}

	subscriptionId := request.Target.Attributes["azure-vm.subscription.id"]
	if len(subscriptionId) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'azure-vm.subscription.id' attribute.", nil)
	}

	resourceGroupName := request.Target.Attributes["azure-vm.resource-group.name"]
	if len(resourceGroupName) == 0 {
		return nil, extension_kit.ToError("Target is missing the 'azure-vm.resource-group.name' attribute.", nil)
	}

	action := request.Config["action"]
	if action == nil {
		return nil, extension_kit.ToError("Missing attack action parameter.", nil)
	}

	state.SubscriptionId = subscriptionId[0]
	state.VmName = vmName[0]
	state.ResourceGroupName = resourceGroupName[0]
	state.Action = action.(string)
	return nil, nil
}

func (e *virtualMachineStateAction) Start(ctx context.Context, state *VirtualMachineStateChangeState) (*action_kit_api.StartResult, error) {
	client, err := e.clientProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize azure client for subscription %s", state.SubscriptionId), err)
	}

	if state.Action == "restart" {
		_, err = client.BeginRestart(ctx, state.ResourceGroupName, state.VmName, nil)
	} else if state.Action == "power-off" {
		_, err = client.BeginPowerOff(ctx, state.ResourceGroupName, state.VmName, nil)
	} else if state.Action == "start" {
		_, err = client.BeginStart(ctx, state.ResourceGroupName, state.VmName, nil)
	} else if state.Action == "delete" {
		_, err = client.BeginDelete(ctx, state.ResourceGroupName, state.VmName, nil)
	} else if state.Action == "deallocate" {
		_, err = client.BeginDeallocate(ctx, state.ResourceGroupName, state.VmName, nil)
	} else {
		return nil, extension_kit.ToError(fmt.Sprintf("Unknown state change attack '%s'", state.Action), nil)
	}

	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to execute state change attack '%s' on vm '%s'", state.Action, state.VmName), err)
	}

	return nil, nil
}

func defaultClientProvider(subscriptionId string) (virtualMachineStateChangeApi, error) {
	return utils.GetVirtualMachinesClient(subscriptionId)
}

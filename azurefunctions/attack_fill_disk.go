package azurefunctions

import (
	"fmt"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

func NewFillDiskAction() action_kit_sdk.Action[AzureFunctionActionState] {
	return &azureFunctionAction{
		description:    getInjectFillDiskDescription(),
		configProvider: injectFillDisk,
	}
}

func getInjectFillDiskDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.fill_disk", TargetIDAzureFunction),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Inject Fill Disk",
		Description:     "Fills disk by a certain amount of memory for each function execution.",
		Icon:            extutil.Ptr(string(targetIcon)),
		TargetSelection: &azureFunctionTargetSelection,
		Technology:      extutil.Ptr("Azure"),
		Category:        extutil.Ptr("Azure Functions"),
		Kind:            action_kit_api.Attack,
		TimeControl:     action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Name:         "duration",
				Type:         action_kit_api.ActionParameterTypeDuration,
				Description:  extutil.Ptr("The duration of the attack."),
				Advanced:     extutil.Ptr(false),
				Required:     extutil.Ptr(true),
				DefaultValue: extutil.Ptr("30s"),
				Order:        extutil.Ptr(0),
			},
			{
				Name:         "rate",
				Label:        "Rate",
				Description:  extutil.Ptr("The rate of invocations to affect."),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: extutil.Ptr("100"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(1),
			},
			{
				Name:         "megabytes",
				Label:        "Megabytes To Fill",
				Description:  extutil.Ptr("Megabytes to fill the disk with for each function execution."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: extutil.Ptr("10"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func injectFillDisk(request action_kit_api.PrepareActionRequestBody) (*FaultInjectionConfig, error) {
	return &FaultInjectionConfig{
		Injection: "FillDisk",
		Rate:      int(request.Config["rate"].(float64)),
		DiskSpace: extutil.Ptr(int(request.Config["megabytes"].(float64))),
		Enabled:   true,
	}, nil
}

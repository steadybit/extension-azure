package azurefunctions

import (
	"errors"
	"fmt"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/appconfig"
	"github.com/steadybit/extension-kit/extbuild"
)

func NewAzureFunctionFillDiskAction() action_kit_sdk.Action[appconfig.AppConfigurationActionState] {
	return &appconfig.AppConfigurationAction{
		Description:    getInjectFillDiskDescription(),
		ConfigProvider: injectFillDisk,
	}
}

func getInjectFillDiskDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.fill_disk", TargetIDAzureFunction),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Fill Diskspace",
		Description:     "Fills tmp diskspace of the function.",
		Icon:            new(string(targetIcon)),
		TargetSelection: &azureFunctionTargetSelection,
		Technology:      new("Azure"),
		Category:        new("Azure Function"),
		Kind:            action_kit_api.Attack,
		TimeControl:     action_kit_api.TimeControlExternal,
		Parameters: []action_kit_api.ActionParameter{
			{
				Label:        "Duration",
				Name:         "duration",
				Type:         action_kit_api.ActionParameterTypeDuration,
				Description:  new("The duration of the attack."),
				Advanced:     new(false),
				Required:     new(true),
				DefaultValue: new("30s"),
				Order:        new(0),
			},
			{
				Name:         "rate",
				Label:        "Rate",
				Description:  new("The rate of invocations to affect."),
				Type:         action_kit_api.ActionParameterTypePercentage,
				DefaultValue: new("100"),
				Required:     new(true),
				Order:        new(1),
			},
			{
				Name:         "megabytes",
				Label:        "Megabytes To Fill",
				Description:  new("Megabytes to fill the disk with for each function execution."),
				Type:         action_kit_api.ActionParameterTypeInteger,
				DefaultValue: new("10"),
				Required:     new(true),
				Order:        new(2),
			},
		},
		Stop: new(action_kit_api.MutatingEndpointReference{}),
	}
}

func injectFillDisk(request action_kit_api.PrepareActionRequestBody) (*appconfig.FaultInjectionConfig, error) {
	appConfigurationEndpoints := request.Target.Attributes["azure-function.app-configuration.endpoint"]

	if len(appConfigurationEndpoints) == 0 {
		return nil, errors.New("no app configuration endpoint found, check if 'STEADYBIT_FAULT_INJECTION_ENDPOINT' environment variable is pointing to the correct app configuration and you are using the Steadybit .NET middleware (https://github.com/steadybit/failure-azure-functions-net)")
	}

	return &appconfig.FaultInjectionConfig{
		Injection:                "FillDisk",
		Rate:                     int(request.Config["rate"].(float64)),
		DiskSpace:                new(int(request.Config["megabytes"].(float64))),
		Enabled:                  true,
		AppConfigurationEndpoint: new(request.Target.Attributes["azure-function.app-configuration.endpoint"][0]),
		AppConfigurationSuffix:   new(request.Target.Attributes["steadybit.label"][0]),
	}, nil
}

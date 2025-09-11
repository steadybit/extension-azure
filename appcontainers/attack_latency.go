package appcontainers

import (
	"errors"
	"fmt"
	"time"

	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/appconfig"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

func NewAppContainerLatencyAction() action_kit_sdk.Action[appconfig.AppConfigurationActionState] {
	return &appconfig.AppConfigurationAction{
		Description:    getInjectLatencyDescription(),
		ConfigProvider: injectLatency,
	}
}

func getInjectLatencyDescription() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:              fmt.Sprintf("%s.latency", TargetIDContainerApp),
		Version:         extbuild.GetSemverVersionStringOrUnknown(),
		Label:           "Inject Latency",
		Description:     "Injects latency into the app container.",
		Icon:            extutil.Ptr(string(targetIcon)),
		TargetSelection: &azureFunctionTargetSelection,
		Technology:      extutil.Ptr("Azure"),
		Category:        extutil.Ptr("Azure App Container"),
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
				Name:         "minimumLatency",
				Label:        "Minimum Latency",
				Description:  extutil.Ptr("Minimum latency to inject into the function in milliseconds."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("1s"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(2),
			},
			{
				Name:         "maximumLatency",
				Label:        "Maximum Latency",
				Description:  extutil.Ptr("Maximum latency to inject into the function."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("2s"),
				Required:     extutil.Ptr(true),
				Order:        extutil.Ptr(3),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func injectLatency(request action_kit_api.PrepareActionRequestBody) (*appconfig.FaultInjectionConfig, error) {
	appConfigurationEndpoints := request.Target.Attributes["container-app.app-configuration.endpoint"]

	if len(appConfigurationEndpoints) == 0 {
		return nil, errors.New("no app configuration endpoint found, check if 'STEADYBIT_FAULT_INJECTION_ENDPOINT' environment variable is pointing to the correct app configuration and you are using the Steadybit .NET middleware (https://github.com/steadybit/failure-azure-functions-net).")
	}

	return &appconfig.FaultInjectionConfig{
		Injection:                "Delay",
		Rate:                     int(request.Config["rate"].(float64)),
		MinLatency:               extutil.Ptr(time.Duration(extutil.ToInt64(request.Config["minimumLatency"])) * time.Millisecond),
		MaxLatency:               extutil.Ptr(time.Duration(extutil.ToInt64(request.Config["maximumLatency"])) * time.Millisecond),
		Enabled:                  true,
		AppConfigurationEndpoint: extutil.Ptr(request.Target.Attributes["container-app.app-configuration.endpoint"][0]),
		AppConfigurationSuffix:   extutil.Ptr(request.Target.Attributes["steadybit.label"][0]),
	}, nil
}

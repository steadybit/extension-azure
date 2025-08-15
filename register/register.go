package register

import (
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/appconfig"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-azure/extscalesetinstance"
	"github.com/steadybit/extension-azure/extvm"
	"github.com/steadybit/extension-azure/nsg"
)

func RegisterHandlers() error {
	configSpec := config.Config

	if configSpec.DiscoveryEnableVirtualMachines {
		discovery_kit_sdk.Register(extvm.NewVirtualMachineDiscovery())
		action_kit_sdk.RegisterAction(extvm.NewVirtualMachineStateAction())
	}

	if configSpec.DiscoveryEnableScaleInstances {
		discovery_kit_sdk.Register(extscalesetinstance.NewScaleSetInstanceDiscovery())
		action_kit_sdk.RegisterAction(extscalesetinstance.NewScaleSetInstanceStateAction())
	}

	if configSpec.DiscoveryEnableAppConfigurations {
		discovery_kit_sdk.Register(appconfig.NewAppConfigurationDiscovery())
		action_kit_sdk.RegisterAction(appconfig.NewExceptionAction())
		action_kit_sdk.RegisterAction(appconfig.NewStatusCodeAction())
		action_kit_sdk.RegisterAction(appconfig.NewLatencyAction())
		action_kit_sdk.RegisterAction(appconfig.NewFillDiskAction())
	}

	if configSpec.DiscoveryEnableNetworkSecurityGroups {
		discovery_kit_sdk.Register(nsg.NewNsgDiscovery())
		action_kit_sdk.RegisterAction(nsg.NewBlockAction())
	}

	return nil
}

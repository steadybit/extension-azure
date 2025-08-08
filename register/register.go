package register

import (
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/azurefunctions"
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

	if configSpec.DiscoveryEnableAzureFunctions {
		discovery_kit_sdk.Register(azurefunctions.NewAzureFunctionDiscovery())
		action_kit_sdk.RegisterAction(azurefunctions.NewExceptionAction())
		action_kit_sdk.RegisterAction(azurefunctions.NewStatusCodeAction())
		action_kit_sdk.RegisterAction(azurefunctions.NewLatencyAction())
		action_kit_sdk.RegisterAction(azurefunctions.NewFillDiskAction())
	}

	if configSpec.DiscoveryEnableNetworkSecurityGroups {
		discovery_kit_sdk.Register(nsg.NewNsgDiscovery())
		action_kit_sdk.RegisterAction(nsg.NewBlockAction())
	}

	return nil
}

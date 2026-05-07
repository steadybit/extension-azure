package register

import (
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/appcontainers"
	"github.com/steadybit/extension-azure/azurefunctions"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-azure/extaks"
	"github.com/steadybit/extension-azure/extapim"
	"github.com/steadybit/extension-azure/extappgateway"
	"github.com/steadybit/extension-azure/extcosmosdb"
	"github.com/steadybit/extension-azure/extdisk"
	"github.com/steadybit/extension-azure/exteventgrid"
	"github.com/steadybit/extension-azure/extloadbalancer"
	"github.com/steadybit/extension-azure/extnatgateway"
	"github.com/steadybit/extension-azure/extscalesetinstance"
	"github.com/steadybit/extension-azure/extservicebus"
	"github.com/steadybit/extension-azure/extstoragequeue"
	"github.com/steadybit/extension-azure/extvm"
	"github.com/steadybit/extension-azure/extvmss"
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

	if configSpec.DiscoveryEnableNetworkSecurityGroups {
		discovery_kit_sdk.Register(nsg.NewNsgDiscovery())
		action_kit_sdk.RegisterAction(nsg.NewBlockAction())
	}

	if configSpec.DiscoveryEnableAzureFunctions {
		discovery_kit_sdk.Register(azurefunctions.NewAzureFunctionDiscovery())
		action_kit_sdk.RegisterAction(azurefunctions.NewAzureFunctionExceptionAction())
		action_kit_sdk.RegisterAction(azurefunctions.NewAzureFunctionStatusCodeAction())
		action_kit_sdk.RegisterAction(azurefunctions.NewAzureFunctionLatencyAction())
		action_kit_sdk.RegisterAction(azurefunctions.NewAzureFunctionFillDiskAction())
	}

	if configSpec.DiscoveryEnableContainerApps {
		discovery_kit_sdk.Register(appcontainers.NewAppContainerDiscovery())
		action_kit_sdk.RegisterAction(appcontainers.NewAppContainerExceptionAction())
		action_kit_sdk.RegisterAction(appcontainers.NewAppContainerStatusCodeAction())
		action_kit_sdk.RegisterAction(appcontainers.NewAppContainerLatencyAction())
		action_kit_sdk.RegisterAction(appcontainers.NewAppContainerFillDiskAction())
	}

	if configSpec.DiscoveryEnableAksCluster {
		discovery_kit_sdk.Register(extaks.NewClusterDiscovery())
	}
	if configSpec.DiscoveryEnableAksNodePool {
		discovery_kit_sdk.Register(extaks.NewNodePoolDiscovery())
		action_kit_sdk.RegisterAction(extaks.NewNodePoolTerminateInstancesAction())
	}
	if configSpec.DiscoveryEnableScaleSet {
		discovery_kit_sdk.Register(extvmss.NewScaleSetDiscovery())
	}
	if configSpec.DiscoveryEnableManagedDisk {
		discovery_kit_sdk.Register(extdisk.NewDiskDiscovery())
	}
	if configSpec.DiscoveryEnableNatGateway {
		discovery_kit_sdk.Register(extnatgateway.NewNatGatewayDiscovery())
	}
	if configSpec.DiscoveryEnableCosmosDb {
		discovery_kit_sdk.Register(extcosmosdb.NewAccountDiscovery())
	}
	if configSpec.DiscoveryEnableEventGrid {
		discovery_kit_sdk.Register(exteventgrid.NewTopicDiscovery())
		discovery_kit_sdk.Register(exteventgrid.NewSubscriptionDiscovery())
	}
	if configSpec.DiscoveryEnableServiceBus {
		discovery_kit_sdk.Register(extservicebus.NewNamespaceDiscovery())
	}
	if configSpec.DiscoveryEnableStorageQueue {
		discovery_kit_sdk.Register(extstoragequeue.NewStorageAccountDiscovery())
	}
	if configSpec.DiscoveryEnableLoadBalancer {
		discovery_kit_sdk.Register(extloadbalancer.NewLoadBalancerDiscovery())
	}
	if configSpec.DiscoveryEnableApplicationGateway {
		discovery_kit_sdk.Register(extappgateway.NewAppGatewayDiscovery())
	}
	if configSpec.DiscoveryEnableApiManagement {
		discovery_kit_sdk.Register(extapim.NewApiManagementDiscovery())
	}

	return nil
}

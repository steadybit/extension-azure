/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extvm

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-azure/config"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const discoveryBasePath = "/" + TargetIDVM + "/discovery"

func RegisterDiscoveryHandlers() {
	exthttp.RegisterHttpHandler(discoveryBasePath, exthttp.GetterAsHandler(getDiscoveryDescription))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/target-description", exthttp.GetterAsHandler(getTargetDescription))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/attribute-descriptions", exthttp.GetterAsHandler(getAttributeDescriptions))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/discovered-targets", getDiscoveredVMs)
	exthttp.RegisterHttpHandler(discoveryBasePath+"/rules/azure-vm-to-container", exthttp.GetterAsHandler(getToContainerEnrichmentRule))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/rules/azure-vm-to-host", exthttp.GetterAsHandler(getToHostEnrichmentRule))
}

var (
	virtualMachinesClient *armcompute.VirtualMachinesClient
)

func GetDiscoveryList() discovery_kit_api.DiscoveryList {
	return discovery_kit_api.DiscoveryList{
		Discoveries: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath,
			},
		},
		TargetTypes: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath + "/target-description",
			},
		},
		TargetAttributes: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath + "/attribute-descriptions",
			},
		},
		TargetEnrichmentRules: []discovery_kit_api.DescribingEndpointReference{
			{
				Method: "GET",
				Path:   discoveryBasePath + "/rules/azure-vm-to-host",
			},
			{
				Method: "GET",
				Path:   discoveryBasePath + "/rules/azure-vm-to-container",
			},
		},
	}
}

func getDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         TargetIDVM,
		RestrictTo: extutil.Ptr(discovery_kit_api.LEADER),
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			Method:       "GET",
			Path:         discoveryBasePath + "/discovered-targets",
			CallInterval: extutil.Ptr("1m"),
		},
	}
}

func getTargetDescription() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:      TargetIDVM,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),

		// Labels used in the UI
		Label: discovery_kit_api.PluralLabel{One: "Azure Virtual Machine", Other: "Azure Virtual Machines"},

		// Category for the targets to appear in
		Category: extutil.Ptr("cloud"),

		// Specify attributes shown in table columns and to be used for sorting
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure-vm.power.state"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{
				{
					Attribute: "steadybit.label",
					Direction: "ASC",
				},
			},
		},
	}
}

func getAttributeDescriptions() discovery_kit_api.AttributeDescriptions {
	return discovery_kit_api.AttributeDescriptions{
		Attributes: []discovery_kit_api.AttributeDescription{
			{
				Attribute: "azure-vm.hostname",
				Label: discovery_kit_api.PluralLabel{
					One:   "Host name",
					Other: "Host names",
				},
			},
			{
				Attribute: "azure.location",
				Label: discovery_kit_api.PluralLabel{
					One:   "Location",
					Other: "Locations",
				},
			},
			{
				Attribute: "azure-vm.network.id",
				Label: discovery_kit_api.PluralLabel{
					One:   "Network ID",
					Other: "Network IDs",
				},
			},
			{
				Attribute: "azure-vm.os.name",
				Label: discovery_kit_api.PluralLabel{
					One:   "OS name",
					Other: "OS names",
				},
			},
			{
				Attribute: "azure-vm.os.type",
				Label: discovery_kit_api.PluralLabel{
					One:   "OS type",
					Other: "OS types",
				},
			},
			{
				Attribute: "azure-vm.os.version",
				Label: discovery_kit_api.PluralLabel{
					One:   "OS version",
					Other: "OS versions",
				},
			},
			{
				Attribute: "azure-vm.power.state",
				Label: discovery_kit_api.PluralLabel{
					One:   "Power state",
					Other: "Power states",
				},
			},
			{
				Attribute: "azure-vm.tags",
				Label: discovery_kit_api.PluralLabel{
					One:   "Tags",
					Other: "Tags",
				},
			},
			{
				Attribute: "azure-vm.vm.id",
				Label: discovery_kit_api.PluralLabel{
					One:   "VM ID",
					Other: "VM IDs",
				},
			},
			{
				Attribute: "azure-vm.vm.size",
				Label: discovery_kit_api.PluralLabel{
					One:   "VM size",
					Other: "VM sizes",
				},
			},
			{
				Attribute: "azure-vm.vm.name",
				Label: discovery_kit_api.PluralLabel{
					One:   "VM name",
					Other: "VM names",
				},
			},
			{
				Attribute: "azure.subscription.id",
				Label: discovery_kit_api.PluralLabel{
					One:   "Subscription ID",
					Other: "Subscription IDs",
				},
			},
			{
				Attribute: "azure.resource-group.name",
				Label: discovery_kit_api.PluralLabel{
					One:   "Resource group name",
					Other: "Resource group names",
				},
			},
		},
	}
}

func getDiscoveredVMs(w http.ResponseWriter, _ *http.Request, _ []byte) {
	ctx := context.Background()
	client, err := common.GetClientByCredentials()
	if err != nil {
		log.Error().Msgf("failed to get client: %v", err)
		return
	}
	targets, err := GetAllVirtualMachines(ctx, client)
	if err != nil {
		log.Error().Msgf("failed to get all virtual machines: %v", err)
		exthttp.WriteError(w, extension_kit.ToError("Failed to collect azure virtual machines information", err))
		return
	}

	exthttp.WriteBody(w, discovery_kit_api.DiscoveryData{Targets: &targets})
}

func GetAllVirtualMachines(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {

	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{to.Ptr(subscriptionId)}
	}
	results, err := client.Resources(ctx,
		armresourcegraph.QueryRequest{
			Query: to.Ptr("Resources | where type =~ 'Microsoft.Compute/virtualMachines' | project name, type, resourceGroup, location, tags, properties, subscriptionId"),
			Options: &armresourcegraph.QueryRequestOptions{
				ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
			},
			Subscriptions: subscriptions,
		},
		nil)
	if err != nil {
		log.Error().Msgf("failed to get results: %v", err)
		return nil, err
	} else {
		// Print the obtained query results
		log.Debug().Msgf("Virtual Machines found: " + strconv.FormatInt(*results.TotalRecords, 10))
		targets := make([]discovery_kit_api.Target, 0)
		if m, ok := results.Data.([]interface{}); ok {
			for _, r := range m {
				items := r.(map[string]interface{})
				attributes := make(map[string][]string)

				properties := common.GetMapValue(items, "properties")
				extended := common.GetMapValue(properties, "extended")
				networkProfile := common.GetMapValue(properties, "networkProfile")
				networkInterfaces := common.GetMapValue(networkProfile, "networkInterfaces")
				instanceView := common.GetMapValue(extended, "instanceView")
				hardwareProfile := common.GetMapValue(properties, "hardwareProfile")
				powerState := common.GetMapValue(instanceView, "powerState")
				storageProfile := common.GetMapValue(properties, "storageProfile")
				osDisk := common.GetMapValue(storageProfile, "osDisk")

				attributes["azure-vm.vm.name"] = []string{items["name"].(string)}
				attributes["azure.subscription.id"] = []string{items["subscriptionId"].(string)}
				attributes["azure-vm.vm.id"] = []string{getPropertyValue(properties, "vmId")}
				attributes["azure-vm.vm.size"] = []string{getPropertyValue(hardwareProfile, "vmSize")}
				attributes["azure-vm.os.name"] = []string{getPropertyValue(instanceView, "osName")}
				attributes["azure-vm.hostname"] = []string{getPropertyValue(instanceView, "computerName")}
				attributes["azure-vm.os.version"] = []string{getPropertyValue(instanceView, "osVersion")}
				attributes["azure-vm.os.type"] = []string{getPropertyValue(osDisk, "osType")}
				attributes["azure-vm.power.state"] = []string{getPropertyValue(powerState, "code")}
				attributes["azure-vm.network.id"] = []string{getPropertyValue(networkInterfaces, "id")}
				attributes["azure.location"] = []string{getPropertyValue(items, "location")}
				attributes["azure.resource-group.name"] = []string{getPropertyValue(items, "resourceGroup")}

				for k, v := range common.GetMapValue(items, "tags") {
					attributes[fmt.Sprintf("azure-vm.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
				}

				targets = append(targets, discovery_kit_api.Target{
					Id:         properties["vmId"].(string),
					TargetType: TargetIDVM,
					Label:      items["name"].(string),
					Attributes: attributes,
				})
			}
		}
		return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesVM), nil
	}
}

func getToHostEnrichmentRule() discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      "com.steadybit.extension_azure.azure-vm-to-host",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDVM,
			Selector: map[string]string{
				"azure-vm.hostname": "${dest.host.hostname}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: "com.steadybit.extension_host.host",
			Selector: map[string]string{
				"host.hostname": "${src.azure-vm.hostname}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.subscription.id",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.vm.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.vm.size",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.os.name",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.os.version",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.os.type",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.network.id",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.location",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.resource-group.name",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "azure-vm.label.",
			},
		},
	}
}

func getToContainerEnrichmentRule() discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      "com.steadybit.extension_azure.azure-vm-to-container",
		Version: extbuild.GetSemverVersionStringOrUnknown(),

		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDVM,
			Selector: map[string]string{
				"azure-vm.hostname": "${dest.container.host}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: "com.steadybit.extension_container.container",
			Selector: map[string]string{
				"container.host": "${src.azure-vm.hostname}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.subscription.id",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.vm.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.vm.size",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.os.name",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.os.version",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.os.type",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-vm.network.id",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.location",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.resource-group.name",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "azure-vm.label.",
			},
		},
	}
}

func getPropertyValue(properties map[string]interface{}, key string) string {
	if value, ok := properties[key]; ok {
		return value.(string)
	}
	return ""
}

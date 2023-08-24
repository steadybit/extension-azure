/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extscalesetinstance

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/extension-azure/common"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/exthttp"
	"github.com/steadybit/extension-kit/extutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const discoveryBasePath = "/" + TargetIDScaleSetInstance + "/discovery"

func RegisterDiscoveryHandlers() {
	exthttp.RegisterHttpHandler(discoveryBasePath, exthttp.GetterAsHandler(getDiscoveryDescription))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/target-description", exthttp.GetterAsHandler(getTargetDescription))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/attribute-descriptions", exthttp.GetterAsHandler(getAttributeDescriptions))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/discovered-targets", getDiscoveredInstances)
	exthttp.RegisterHttpHandler(discoveryBasePath+"/rules/azure-scale_set-vm-to-container", exthttp.GetterAsHandler(getToContainerEnrichmentRule))
	exthttp.RegisterHttpHandler(discoveryBasePath+"/rules/azure-scale_set-vm-to-host", exthttp.GetterAsHandler(getToHostEnrichmentRule))
}

var (
	instancesClient *armcompute.VirtualMachineScaleSetVMsClient
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
				Path:   discoveryBasePath + "/rules/azure-scale_set-vm-to-host",
			},
			{
				Method: "GET",
				Path:   discoveryBasePath + "/rules/azure-scale_set-vm-to-container",
			},
		},
	}
}

func getDiscoveryDescription() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:         TargetIDScaleSetInstance,
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
		Id:      TargetIDScaleSetInstance,
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),

		// Labels used in the UI
		Label: discovery_kit_api.PluralLabel{One: "Azure Scale Set Instance", Other: "Azure Scale Set Instances"},

		// Category for the targets to appear in
		Category: extutil.Ptr("cloud"),

		// Specify attributes shown in table columns and to be used for sorting
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
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
				Attribute: "azure.location",
				Label: discovery_kit_api.PluralLabel{
					One:   "Location",
					Other: "Locations",
				},
			},
		},
	}
}

func getDiscoveredInstances(w http.ResponseWriter, _ *http.Request, _ []byte) {
	ctx := context.Background()
	client, err := common.GetClientByCredentials()
	if err != nil {
		log.Error().Msgf("failed to get client: %v", err)
		return
	}
	scaleSets, err := GetAllScaleSets(ctx, client)
	if err != nil {
		log.Error().Msgf("failed to get all scale-sets: %v", err)
		exthttp.WriteError(w, extension_kit.ToError("Failed to collect azure scale set instances information", err))
		return
	}

	scaleSetMap := make(map[string][]ScaleSet)
	for _, scaleSet := range scaleSets {
		if scaleSetMap[scaleSet.SubscriptionId] != nil {
			scaleSetMap[scaleSet.SubscriptionId] = append(scaleSetMap[scaleSet.SubscriptionId], scaleSet)
		} else {
			scaleSetMap[scaleSet.SubscriptionId] = []ScaleSet{scaleSet}
		}
	}

	targets := make([]discovery_kit_api.Target, 0)
	for subscriptionId, scaleSetList := range scaleSetMap {
		scaleSetVMsClient, err := common.GetVirtualMachineScaleSetVMsClient(subscriptionId)
		if err != nil {
			log.Error().Msgf("failed to get client: %v", err)
			continue
		}

		for _, scaleSet := range scaleSetList {
			newTargets, err := GetAllScaleSetInstances(ctx, scaleSetVMsClient, scaleSet)
			if err != nil {
				log.Error().Msgf("failed to get all scale-sets instances: %v", err)
				exthttp.WriteError(w, extension_kit.ToError("Failed to collect azure scale set instances information", err))
				return
			}
			targets = append(targets, newTargets...)
		}
	}

	exthttp.WriteBody(w, discovery_kit_api.DiscoveryData{Targets: &targets})
}

func GetAllScaleSetInstances(ctx context.Context, scaleSetVMsClient *armcompute.VirtualMachineScaleSetVMsClient, scaleSet ScaleSet) ([]discovery_kit_api.Target, error) {
	pager := scaleSetVMsClient.NewListPager(scaleSet.ResourceGroupName, scaleSet.Name, nil)
	targets := make([]discovery_kit_api.Target, 0)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Printf("Error getting next page: %v\n", err)
		}

		for _, instance := range page.Value {
			attributes := make(map[string][]string)


			attributes["azure-scaleset.name"] = []string{scaleSet.Name}
			attributes["azure-scaleset-instance.vm.name"] = []string{*instance.Name}
			attributes["azure.subscription.id"] = []string{scaleSet.SubscriptionId}
			attributes["azure-scaleset-instance.resource.id"] = []string{*instance.ID}
			attributes["azure-scaleset-instance.id"] = []string{*instance.InstanceID}

			if instance.Properties != nil {
				if instance.Properties.OSProfile != nil {
					attributes["azure-scaleset-instance.hostname"] = []string{getStringValue(instance.Properties.OSProfile.ComputerName)}
				}
				if instance.Properties.HardwareProfile != nil {
					if instance.Properties.HardwareProfile.VMSize != nil {
						attributes["azure-scaleset-instance.vm.size"] = []string{fmt.Sprint(*instance.Properties.HardwareProfile.VMSize)}
					}
				}
				if instance.Properties.InstanceView != nil {
					attributes["azure-scaleset-instance.os.name"] = []string{getStringValue(instance.Properties.InstanceView.OSName)}
					attributes["azure-scaleset-instance.os.version"] = []string{getStringValue(instance.Properties.InstanceView.OSVersion)}
				}
				if instance.Properties.StorageProfile != nil && instance.Properties.StorageProfile.OSDisk != nil && instance.Properties.StorageProfile.OSDisk.OSType != nil {
					attributes["azure-scaleset-instance.os.type"] = []string{fmt.Sprint(*instance.Properties.StorageProfile.OSDisk.OSType)}
				}
				if instance.Properties.InstanceView != nil && instance.Properties.InstanceView.Statuses != nil {
					for _, status := range instance.Properties.InstanceView.Statuses {
						log.Info().Msgf("Status: %v", status)
					}
					//attributes["azure-scaleset-instance.power.state"] = []string{instance.Properties.InstanceView.Statuses}
				}
				attributes["azure-scaleset-instance.provisioning.state"] = []string{getStringValue(instance.Properties.ProvisioningState)}
			}

			attributes["azure.location"] = []string{scaleSet.Location}
			zones := ""
			if instance.Zones != nil {
				for _, zone := range instance.Zones {
					zones += getStringValue(zone) + ","
				}
			}
			attributes["azure.zone"] = []string{zones}
			attributes["azure.resource-group.name"] = []string{scaleSet.ResourceGroupName}

			for k, v := range instance.Tags {
				attributes[fmt.Sprintf("azure-scaleset-instance.label.%s", strings.ToLower(k))] = []string{getStringValue(v)}
			}
			//scaleSet.Attributes
			for k, v := range scaleSet.Attributes {
				attributes[k] = v
			}

			targets = append(targets, discovery_kit_api.Target{
				Id:         getStringValue(instance.ID),
				TargetType: TargetIDScaleSetInstance,
				Label:      getStringValue(instance.Name),
				Attributes: attributes,
			})
		}
	}
	return targets, nil
}

type ScaleSet struct {
	Id                string
	Name              string
	ResourceGroupName string
	Location          string
	SubscriptionId    string
	Attributes        map[string][]string
}

func GetAllScaleSets(ctx context.Context, client common.ArmResourceGraphApi) ([]ScaleSet, error) {

	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{to.Ptr(subscriptionId)}
	}
	results, err := client.Resources(ctx,
		armresourcegraph.QueryRequest{
			Query: to.Ptr("Resources | where type =~ 'microsoft.compute/virtualmachinescalesets' | project name, type, ['id'], resourceGroup, location, tags, properties, subscriptionId"),
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
		log.Debug().Msgf("ScaleSets found: " + strconv.FormatInt(*results.TotalRecords, 10))
		scaleSets := make([]ScaleSet, 0)
		if m, ok := results.Data.([]interface{}); ok {
			for _, r := range m {
				items := r.(map[string]interface{})
				attributes := make(map[string][]string)

				for k, v := range getMapValue(items, "tags") {
					attributes[fmt.Sprintf("azure-scaleset.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
				}

				scaleSets = append(scaleSets, ScaleSet{
					Id:                items["id"].(string),
					Name:              items["name"].(string),
					Location:          items["location"].(string),
					ResourceGroupName: items["resourceGroup"].(string),
					SubscriptionId:    items["subscriptionId"].(string),
					Attributes:        attributes,
				})
			}
		}
		return scaleSets, nil
	}
}

func getToHostEnrichmentRule() discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      "com.steadybit.extension_azure.azure-scaleset-instance-to-host",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDScaleSetInstance,
			Selector: map[string]string{
				"azure-scaleset-instance.hostname": "${dest.host.hostname}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: "com.steadybit.extension_host.host",
			Selector: map[string]string{
				"host.hostname": "${src.azure-scaleset-instance.hostname}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.subscription.id",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.vm.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.vm.size",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.os.name",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.os.version",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.os.type",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.network.id",
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
				Name:    "azure-scaleset-instance.label.",
			},
		},
	}
}

func getToContainerEnrichmentRule() discovery_kit_api.TargetEnrichmentRule {
	return discovery_kit_api.TargetEnrichmentRule{
		Id:      "com.steadybit.extension_azure.azure-scaleset-instance-to-container",
		Version: extbuild.GetSemverVersionStringOrUnknown(),

		Src: discovery_kit_api.SourceOrDestination{
			Type: TargetIDScaleSetInstance,
			Selector: map[string]string{
				"azure-scaleset-instance.hostname": "${dest.container.host}",
			},
		},
		Dest: discovery_kit_api.SourceOrDestination{
			Type: "com.steadybit.extension_container.container",
			Selector: map[string]string{
				"container.host": "${src.azure-scaleset-instance.hostname}",
			},
		},
		Attributes: []discovery_kit_api.Attribute{
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.subscription.id",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.vm.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.vm.size",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.os.name",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.os.version",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.os.type",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scaleset-instance.network.id",
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
				Name:    "azure-scaleset-instance.label.",
			},
		},
	}
}

func getStringValue(val *string) string {
	if val != nil {
		return *val
	}
	return ""
}

func getPropertyValue(properties map[string]interface{}, key string) string {
	if value, ok := properties[key]; ok {
		return value.(string)
	}
	return ""
}

func getMapValue(properties map[string]interface{}, key string) map[string]interface{} {
	if value, ok := properties[key]; ok {
		if m, ok := value.(map[string]interface{}); ok {
			return m
		} else if n, ok := value.([]interface{}); ok {
			if len(n) > 0 {
				if o, ok := n[0].(map[string]interface{}); ok {
					return o
				}
			}
		}
	}
	return make(map[string]interface{})
}

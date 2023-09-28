/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package extscalesetinstance

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
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
				{Attribute: "azure-scale-set-instance.provisioning.state"},
				{Attribute: "azure.location"},
				{Attribute: "azure-containerservice-managed-cluster.name"},
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
				Attribute: "azure-containerservice-managed-cluster.name",
				Label: discovery_kit_api.PluralLabel{
					One:   "Cluster name",
					Other: "Cluster names",
				},
			},
			{
				Attribute: "azure-scale-set-instance.hostname",
				Label: discovery_kit_api.PluralLabel{
					One:   "Host name",
					Other: "Host names",
				},
			},
			{
				Attribute: "azure-scale-set-instance.os.name",
				Label: discovery_kit_api.PluralLabel{
					One:   "OS name",
					Other: "OS names",
				},
			},
			{
				Attribute: "azure-scale-set-instance.os.type",
				Label: discovery_kit_api.PluralLabel{
					One:   "OS type",
					Other: "OS types",
				},
			},
			{
				Attribute: "azure-scale-set-instance.os.version",
				Label: discovery_kit_api.PluralLabel{
					One:   "OS version",
					Other: "OS versions",
				},
			},
			{
				Attribute: "azure-scale-set-instance.id",
				Label: discovery_kit_api.PluralLabel{
					One:   "VM ID",
					Other: "VM IDs",
				},
			},
			{
				Attribute: "azure-scale-set-instance.vm.size",
				Label: discovery_kit_api.PluralLabel{
					One:   "VM size",
					Other: "VM sizes",
				},
			},
			{
				Attribute: "azure-scale-set-instance.name",
				Label: discovery_kit_api.PluralLabel{
					One:   "VM name",
					Other: "VM names",
				},
			},
			{
				Attribute: "azure-scale-set-instance.provisioning.state",
				Label: discovery_kit_api.PluralLabel{
					One:   "Provisioning state",
					Other: "Provisioning states",
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
		appendKubernetesServiceAttributes(ctx, client, scaleSet)
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

func appendKubernetesServiceAttributes(ctx context.Context, client *armresourcegraph.Client, scaleSet ScaleSet) {
	clusters, err := getKubernetesManagedClusters(ctx, client, scaleSet.ResourceGroupName)
	if err != nil {
		log.Error().Msgf("failed to get kubernetes managed clusters: %v", err)
		return
	}
	for _, cluster := range clusters {
		common.AddAttribute(scaleSet.Attributes, "azure-containerservice-managed-cluster.name", cluster.Name)
		common.AddAttribute(scaleSet.Attributes, "azure-containerservice-managed-cluster.location", cluster.Location)
		common.AddAttribute(scaleSet.Attributes, "azure-containerservice-managed-cluster.resource-group.name", cluster.ResourceGroupName)
		common.AddAttribute(scaleSet.Attributes, "azure-containerservice-managed-cluster.subscription.id", cluster.SubscriptionId)
		for k, v := range cluster.Attributes {
			common.AddAttribute(scaleSet.Attributes, k, v[0])
		}
	}
}

type AzureVirtualMachineScaleSetVMsClient interface {
	NewListPager(resourceGroupName string, virtualMachineScaleSetName string, options *armcompute.VirtualMachineScaleSetVMsClientListOptions) *runtime.Pager[armcompute.VirtualMachineScaleSetVMsClientListResponse]
}

func GetAllScaleSetInstances(ctx context.Context, scaleSetVMsClient AzureVirtualMachineScaleSetVMsClient, scaleSet ScaleSet) ([]discovery_kit_api.Target, error) {
	pager := scaleSetVMsClient.NewListPager(scaleSet.ResourceGroupName, scaleSet.Name, nil)
	targets := make([]discovery_kit_api.Target, 0)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Printf("Error getting next page: %v\n", err)
		}

		for _, instance := range page.Value {
			attributes := make(map[string][]string)

			attributes["azure-scale-set.name"] = []string{scaleSet.Name}
			attributes["azure-scale-set-instance.name"] = []string{*instance.Name}
			attributes["azure.subscription.id"] = []string{scaleSet.SubscriptionId}
			attributes["azure-scale-set-instance.resource.id"] = []string{*instance.ID}
			attributes["azure-scale-set-instance.id"] = []string{*instance.InstanceID}
			attributes["azure.location"] = []string{scaleSet.Location}
			attributes["azure.resource-group.name"] = []string{scaleSet.ResourceGroupName}

			zones := ""
			if instance.Zones != nil {
				for _, zone := range instance.Zones {
					zones += common.GetStringValue(zone) + ","
				}
			}
			attributes["azure.zone"] = []string{zones}

			if instance.Properties != nil {
				if instance.Properties.OSProfile != nil {
					attributes["azure-scale-set-instance.hostname"] = []string{common.GetStringValue(instance.Properties.OSProfile.ComputerName)}
				}
				if instance.Properties.HardwareProfile != nil {
					if instance.Properties.HardwareProfile.VMSize != nil {
						attributes["azure-scale-set-instance.vm.size"] = []string{fmt.Sprint(*instance.Properties.HardwareProfile.VMSize)}
					}
				}
				if instance.Properties.InstanceView != nil {
					attributes["azure-scale-set-instance.os.name"] = []string{common.GetStringValue(instance.Properties.InstanceView.OSName)}
					attributes["azure-scale-set-instance.os.version"] = []string{common.GetStringValue(instance.Properties.InstanceView.OSVersion)}
				}
				if instance.Properties.StorageProfile != nil && instance.Properties.StorageProfile.OSDisk != nil && instance.Properties.StorageProfile.OSDisk.OSType != nil {
					attributes["azure-scale-set-instance.os.type"] = []string{fmt.Sprint(*instance.Properties.StorageProfile.OSDisk.OSType)}
				}
				attributes["azure-scale-set-instance.provisioning.state"] = []string{common.GetStringValue(instance.Properties.ProvisioningState)}
			}

			for k, v := range instance.Tags {
				attributes[fmt.Sprintf("azure-scale-set-instance.label.%s", strings.ToLower(k))] = []string{common.GetStringValue(v)}
			}
			//scaleSet.Attributes
			for k, v := range scaleSet.Attributes {
				attributes[k] = v
			}

			targets = append(targets, discovery_kit_api.Target{
				Id:         common.GetStringValue(instance.ID),
				TargetType: TargetIDScaleSetInstance,
				Label:      common.GetStringValue(instance.Name),
				Attributes: attributes,
			})
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesScaleSetInstance), nil
}

type ScaleSet struct {
	Id                string
	Name              string
	ResourceGroupName string
	Location          string
	SubscriptionId    string
	Attributes        map[string][]string
}

type KubernetesService struct {
	Name              string
	ResourceGroupName string
	Location          string
	SubscriptionId    string
	Attributes        map[string][]string
}

func getKubernetesManagedClusters(ctx context.Context, client common.ArmResourceGraphApi, nodeResourceGroup string) ([]KubernetesService, error) {

	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{to.Ptr(subscriptionId)}
	}
	results, err := client.Resources(ctx,
		armresourcegraph.QueryRequest{
			Query: to.Ptr("resources | where type =~ 'microsoft.containerservice/managedclusters' and tolower(properties.nodeResourceGroup) == \"" + nodeResourceGroup + "\" | project name, type, resourceGroup, location, tags, properties, subscriptionId"),
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
		log.Debug().Msgf("Kubernetes Services found: " + strconv.FormatInt(*results.TotalRecords, 10))
		kubernetesServices := make([]KubernetesService, 0)
		if m, ok := results.Data.([]interface{}); ok {
			for _, r := range m {
				items := r.(map[string]interface{})
				attributes := make(map[string][]string)

				for k, v := range common.GetMapValue(items, "tags") {
					attributes[fmt.Sprintf("azure-containerservice-managed-cluster.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
				}

				kubernetesServices = append(kubernetesServices, KubernetesService{
					Name:              items["name"].(string),
					Location:          items["location"].(string),
					ResourceGroupName: items["resourceGroup"].(string),
					SubscriptionId:    items["subscriptionId"].(string),
					Attributes:        attributes,
				})
			}
		}
		return kubernetesServices, nil
	}
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
		log.Debug().Msgf("ScaleSets found: " + strconv.FormatInt(*results.TotalRecords, 10))
		scaleSets := make([]ScaleSet, 0)
		if m, ok := results.Data.([]interface{}); ok {
			for _, r := range m {
				items := r.(map[string]interface{})
				attributes := make(map[string][]string)

				for k, v := range common.GetMapValue(items, "tags") {
					attributes[fmt.Sprintf("azure-scale-set.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
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
				"azure-scale-set-instance.hostname": "${dest.host.hostname}",
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
				Name:    "azure-scale-set-instance.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set-instance.vm.size",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set-instance.os.name",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set-instance.os.version",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set-instance.os.type",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.location",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.resource-group.name",
			}, {
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set.name",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set-instance.provisioning.state",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "azure-scale-set-instance.label.",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "azure-scale-set.label.",
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
				"azure-scale-set-instance.hostname": "${dest.container.host}",
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
				Name:    "azure-scale-set-instance.id",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set-instance.vm.size",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set-instance.os.name",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set-instance.os.version",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set-instance.os.type",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.location",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure.resource-group.name",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set-instance.provisioning.state",
			},
			{
				Matcher: discovery_kit_api.Equals,
				Name:    "azure-scale-set.name",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "azure-scale-set-instance.label.",
			},
			{
				Matcher: discovery_kit_api.StartsWith,
				Name:    "azure-scale-set.label.",
			},
		},
	}
}

/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extaks

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
	"os"
)

type nodePoolDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*nodePoolDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*nodePoolDiscovery)(nil)
)

func NewNodePoolDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&nodePoolDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *nodePoolDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id: TargetIDNodePool,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("60s"),
		},
	}
}

func (d *nodePoolDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDNodePool,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure AKS node pool", Other: "Azure AKS node pools"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.aks.cluster.name"},
				{Attribute: "azure.aks.nodepool.mode"},
				{Attribute: "azure.aks.nodepool.scale-set-priority"},
				{Attribute: "azure.aks.nodepool.min-count"},
				{Attribute: "azure.aks.nodepool.max-count"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *nodePoolDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.aks.cluster.name", Label: discovery_kit_api.PluralLabel{One: "AKS cluster name", Other: "AKS cluster names"}},
		{Attribute: "azure.aks.nodepool.name", Label: discovery_kit_api.PluralLabel{One: "AKS node pool name", Other: "AKS node pool names"}},
		{Attribute: "azure.aks.nodepool.mode", Label: discovery_kit_api.PluralLabel{One: "AKS node pool mode", Other: "AKS node pool modes"}},
		{Attribute: "azure.aks.nodepool.scale-set-priority", Label: discovery_kit_api.PluralLabel{One: "AKS node pool scale-set priority", Other: "AKS node pool scale-set priorities"}},
		{Attribute: "azure.aks.nodepool.scale-set-eviction-policy", Label: discovery_kit_api.PluralLabel{One: "AKS node pool eviction policy", Other: "AKS node pool eviction policies"}},
		{Attribute: "azure.aks.nodepool.os-type", Label: discovery_kit_api.PluralLabel{One: "AKS node pool OS type", Other: "AKS node pool OS types"}},
		{Attribute: "azure.aks.nodepool.os-sku", Label: discovery_kit_api.PluralLabel{One: "AKS node pool OS SKU", Other: "AKS node pool OS SKUs"}},
		{Attribute: "azure.aks.nodepool.vm-size", Label: discovery_kit_api.PluralLabel{One: "AKS node pool VM size", Other: "AKS node pool VM sizes"}},
		{Attribute: "azure.aks.nodepool.orchestrator-version", Label: discovery_kit_api.PluralLabel{One: "AKS node pool Kubernetes version", Other: "AKS node pool Kubernetes versions"}},
		{Attribute: "azure.aks.nodepool.min-count", Label: discovery_kit_api.PluralLabel{One: "AKS node pool min count", Other: "AKS node pool min counts"}},
		{Attribute: "azure.aks.nodepool.max-count", Label: discovery_kit_api.PluralLabel{One: "AKS node pool max count", Other: "AKS node pool max counts"}},
		{Attribute: "azure.aks.nodepool.enable-auto-scaling", Label: discovery_kit_api.PluralLabel{One: "AKS node pool autoscaling", Other: "AKS node pool autoscaling"}},
		{Attribute: "azure.aks.nodepool.max-pods", Label: discovery_kit_api.PluralLabel{One: "AKS node pool max pods", Other: "AKS node pool max pods"}},
		{Attribute: "azure.aks.nodepool.availability-zones", Label: discovery_kit_api.PluralLabel{One: "AKS node pool availability zone", Other: "AKS node pool availability zones"}},
		{Attribute: "azure.aks.nodepool.taints", Label: discovery_kit_api.PluralLabel{One: "AKS node pool taint", Other: "AKS node pool taints"}},
		{Attribute: "azure.aks.nodepool.upgrade-settings.max-surge", Label: discovery_kit_api.PluralLabel{One: "AKS node pool upgrade max surge", Other: "AKS node pool upgrade max surges"}},
		{Attribute: "azure.aks.nodepool.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "AKS node pool provisioning state", Other: "AKS node pool provisioning states"}},
		{Attribute: "k8s.cluster-name", Label: discovery_kit_api.PluralLabel{One: "Kubernetes cluster name", Other: "Kubernetes cluster names"}},
	}
}

func (d *nodePoolDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllAksNodePools(ctx, client)
}

func getAllAksNodePools(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	// AKS node pools live as separate ARM resources of type Microsoft.ContainerService/managedClusters/agentPools.
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.ContainerService/managedClusters/agentPools' | project id, name, type, resourceGroup, location, properties, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get AKS node pool results")
		return nil, err
	}

	targets := make([]discovery_kit_api.Target, 0)
	rows, ok := results.Data.([]any)
	if !ok {
		return targets, nil
	}
	for _, r := range rows {
		items, ok := r.(map[string]any)
		if !ok {
			continue
		}
		targets = append(targets, toNodePoolTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesAksNodePool), nil
}

func toNodePoolTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	upgradeSettings := common.GetMapValue(properties, "upgradeSettings")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string) // ARM returns "<cluster>/<pool>" for child resources
	clusterName := name
	poolName := name
	if i := strings.Index(name, "/"); i >= 0 {
		clusterName = name[:i]
		poolName = name[i+1:]
	}
	label := fmt.Sprintf("%s/%s", clusterName, poolName)

	attributes := make(map[string][]string)
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}
	attributes["azure.aks.cluster.name"] = []string{clusterName}
	// Surface the cluster-wide k8s.cluster-name attribute so extension-kubernetes's targets can be joined
	// to this node pool via enrichment (matches the AKS cluster target's k8s.cluster-name).
	attributes["k8s.cluster-name"] = []string{clusterName}
	attributes["azure.aks.nodepool.name"] = []string{poolName}

	if v := stringFromMap(properties, "mode"); v != "" {
		attributes["azure.aks.nodepool.mode"] = []string{v}
	}
	if v := stringFromMap(properties, "scaleSetPriority"); v != "" {
		attributes["azure.aks.nodepool.scale-set-priority"] = []string{v}
	}
	if v := stringFromMap(properties, "scaleSetEvictionPolicy"); v != "" {
		attributes["azure.aks.nodepool.scale-set-eviction-policy"] = []string{v}
	}
	if v := stringFromMap(properties, "osType"); v != "" {
		attributes["azure.aks.nodepool.os-type"] = []string{v}
	}
	if v := stringFromMap(properties, "osSKU"); v != "" {
		attributes["azure.aks.nodepool.os-sku"] = []string{v}
	}
	if v := stringFromMap(properties, "vmSize"); v != "" {
		attributes["azure.aks.nodepool.vm-size"] = []string{v}
	}
	if v := stringFromMap(properties, "orchestratorVersion"); v != "" {
		attributes["azure.aks.nodepool.orchestrator-version"] = []string{v}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.aks.nodepool.provisioning-state"] = []string{v}
	}
	if v, ok := properties["minCount"].(float64); ok {
		attributes["azure.aks.nodepool.min-count"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := properties["maxCount"].(float64); ok {
		attributes["azure.aks.nodepool.max-count"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := properties["enableAutoScaling"].(bool); ok {
		attributes["azure.aks.nodepool.enable-auto-scaling"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["maxPods"].(float64); ok {
		attributes["azure.aks.nodepool.max-pods"] = []string{strconv.Itoa(int(v))}
	}
	if zones := stringSliceFromMap(properties, "availabilityZones"); len(zones) > 0 {
		sort.Strings(zones)
		attributes["azure.aks.nodepool.availability-zones"] = zones
	}
	if taints := stringSliceFromMap(properties, "nodeTaints"); len(taints) > 0 {
		sort.Strings(taints)
		attributes["azure.aks.nodepool.taints"] = taints
	}
	if v := stringFromMap(upgradeSettings, "maxSurge"); v != "" {
		attributes["azure.aks.nodepool.upgrade-settings.max-surge"] = []string{v}
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.aks.nodepool.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDNodePool,
		Label:      label,
		Attributes: attributes,
	}
}

/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extaks

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
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
		Icon:     extutil.Ptr(nodePoolIcon),
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
	rgClient, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get Resource Graph client: %w", err)
	}
	return getAllAksNodePools(ctx, rgClient, sdkAksAgentPoolLister(newAgentPoolsClient))
}

// aksClusterRef carries the addressing fields needed to issue per-cluster AgentPool listings.
type aksClusterRef struct {
	name           string
	resourceGroup  string
	subscriptionId string
	location       string
}

// aksAgentPoolLister returns all agent pools for one cluster. Indirection over the SDK pager so
// tests can swap a fake implementation.
type aksAgentPoolLister func(ctx context.Context, subscriptionId, resourceGroup, clusterName string) ([]*armcontainerservice.AgentPool, error)

// sdkAksAgentPoolLister wraps the SDK pager into a flat lister. Production use only.
func sdkAksAgentPoolLister(provider func(subscriptionId string) (*armcontainerservice.AgentPoolsClient, error)) aksAgentPoolLister {
	return func(ctx context.Context, subscriptionId, resourceGroup, clusterName string) ([]*armcontainerservice.AgentPool, error) {
		client, err := provider(subscriptionId)
		if err != nil {
			return nil, err
		}
		pager := client.NewListPager(resourceGroup, clusterName, nil)
		var pools []*armcontainerservice.AgentPool
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return pools, err
			}
			pools = append(pools, page.Value...)
		}
		return pools, nil
	}
}

// getAllAksNodePools lists AKS node pools via direct ARM. Resource Graph indexes the
// Microsoft.ContainerService/managedClusters/agentPools type with a multi-minute lag, which makes
// ad-hoc dev testing painful (newly-created clusters' pools don't appear for ~15-20 min). Direct
// ARM is real-time. Clusters themselves still come from Resource Graph because their cardinality
// is low and the lag matters less at the cluster level.
func getAllAksNodePools(ctx context.Context, rgClient common.ArmResourceGraphApi, lister aksAgentPoolLister) ([]discovery_kit_api.Target, error) {
	clusters, err := listAksClusterRefs(ctx, rgClient)
	if err != nil {
		return nil, err
	}

	targets := make([]discovery_kit_api.Target, 0)
	for _, c := range clusters {
		pools, err := lister(ctx, c.subscriptionId, c.resourceGroup, c.name)
		if err != nil {
			log.Warn().Err(err).Msgf("failed to list AKS node pools for cluster %s/%s; skipping", c.subscriptionId, c.name)
			continue
		}
		for _, p := range pools {
			if p == nil {
				continue
			}
			targets = append(targets, nodePoolTargetFromSDK(p, c))
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesAksNodePool), nil
}

func listAksClusterRefs(ctx context.Context, rgClient common.ArmResourceGraphApi) ([]aksClusterRef, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := rgClient.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.ContainerService/managedClusters' | project name, resourceGroup, location, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to list AKS clusters from Resource Graph")
		return nil, err
	}

	refs := make([]aksClusterRef, 0)
	rows, ok := results.Data.([]any)
	if !ok {
		return refs, nil
	}
	for _, r := range rows {
		items, ok := r.(map[string]any)
		if !ok {
			continue
		}
		refs = append(refs, aksClusterRef{
			name:           stringFromMap(items, "name"),
			resourceGroup:  stringFromMap(items, "resourceGroup"),
			subscriptionId: stringFromMap(items, "subscriptionId"),
			location:       stringFromMap(items, "location"),
		})
	}
	return refs, nil
}

func nodePoolTargetFromSDK(p *armcontainerservice.AgentPool, c aksClusterRef) discovery_kit_api.Target {
	poolName := ""
	if p.Name != nil {
		poolName = *p.Name
	}
	id := ""
	if p.ID != nil {
		id = *p.ID
	}

	attributes := map[string][]string{
		"azure.subscription.id":     {c.subscriptionId},
		"azure.resource-group.name": {c.resourceGroup},
		"azure.location":            {c.location},
		"azure.aks.cluster.name":    {c.name},
		// Surface the cluster-wide k8s.cluster-name attribute so extension-kubernetes's targets can be
		// joined to this node pool via enrichment (matches the AKS cluster target's k8s.cluster-name).
		"k8s.cluster-name":           {c.name},
		"azure.aks.nodepool.name":    {poolName},
	}

	if pp := p.Properties; pp != nil {
		if pp.Mode != nil {
			attributes["azure.aks.nodepool.mode"] = []string{string(*pp.Mode)}
		}
		if pp.ScaleSetPriority != nil {
			attributes["azure.aks.nodepool.scale-set-priority"] = []string{string(*pp.ScaleSetPriority)}
		}
		if pp.ScaleSetEvictionPolicy != nil {
			attributes["azure.aks.nodepool.scale-set-eviction-policy"] = []string{string(*pp.ScaleSetEvictionPolicy)}
		}
		if pp.OSType != nil {
			attributes["azure.aks.nodepool.os-type"] = []string{string(*pp.OSType)}
		}
		if pp.OSSKU != nil {
			attributes["azure.aks.nodepool.os-sku"] = []string{string(*pp.OSSKU)}
		}
		if pp.VMSize != nil {
			attributes["azure.aks.nodepool.vm-size"] = []string{*pp.VMSize}
		}
		if pp.OrchestratorVersion != nil {
			attributes["azure.aks.nodepool.orchestrator-version"] = []string{*pp.OrchestratorVersion}
		}
		if pp.ProvisioningState != nil {
			attributes["azure.aks.nodepool.provisioning-state"] = []string{*pp.ProvisioningState}
		}
		if pp.MinCount != nil {
			attributes["azure.aks.nodepool.min-count"] = []string{strconv.Itoa(int(*pp.MinCount))}
		}
		if pp.MaxCount != nil {
			attributes["azure.aks.nodepool.max-count"] = []string{strconv.Itoa(int(*pp.MaxCount))}
		}
		if pp.EnableAutoScaling != nil {
			attributes["azure.aks.nodepool.enable-auto-scaling"] = []string{strconv.FormatBool(*pp.EnableAutoScaling)}
		}
		if pp.MaxPods != nil {
			attributes["azure.aks.nodepool.max-pods"] = []string{strconv.Itoa(int(*pp.MaxPods))}
		}
		if len(pp.AvailabilityZones) > 0 {
			zones := make([]string, 0, len(pp.AvailabilityZones))
			for _, z := range pp.AvailabilityZones {
				if z != nil {
					zones = append(zones, *z)
				}
			}
			sort.Strings(zones)
			attributes["azure.aks.nodepool.availability-zones"] = zones
		}
		if len(pp.NodeTaints) > 0 {
			taints := make([]string, 0, len(pp.NodeTaints))
			for _, t := range pp.NodeTaints {
				if t != nil {
					taints = append(taints, *t)
				}
			}
			sort.Strings(taints)
			attributes["azure.aks.nodepool.taints"] = taints
		}
		if pp.UpgradeSettings != nil && pp.UpgradeSettings.MaxSurge != nil {
			attributes["azure.aks.nodepool.upgrade-settings.max-surge"] = []string{*pp.UpgradeSettings.MaxSurge}
		}
		for k, v := range pp.Tags {
			if v != nil {
				attributes[fmt.Sprintf("azure.aks.nodepool.label.%s", k)] = []string{*v}
			}
		}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDNodePool,
		Label:      fmt.Sprintf("%s/%s", c.name, poolName),
		Attributes: attributes,
	}
}


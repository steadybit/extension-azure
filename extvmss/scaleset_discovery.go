/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extvmss

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

type scaleSetDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*scaleSetDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*scaleSetDiscovery)(nil)
)

func NewScaleSetDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&scaleSetDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *scaleSetDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDScaleSet,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *scaleSetDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDScaleSet,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Virtual Machine Scale Set", Other: "Azure Virtual Machine Scale Sets"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.vmss.sku.capacity"},
				{Attribute: "azure.vmss.zones"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *scaleSetDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.vmss.name", Label: discovery_kit_api.PluralLabel{One: "VMSS name", Other: "VMSS names"}},
		{Attribute: "azure.vmss.sku.name", Label: discovery_kit_api.PluralLabel{One: "VMSS SKU name", Other: "VMSS SKU names"}},
		{Attribute: "azure.vmss.sku.tier", Label: discovery_kit_api.PluralLabel{One: "VMSS SKU tier", Other: "VMSS SKU tiers"}},
		{Attribute: "azure.vmss.sku.capacity", Label: discovery_kit_api.PluralLabel{One: "VMSS SKU capacity", Other: "VMSS SKU capacities"}},
		{Attribute: "azure.vmss.zones", Label: discovery_kit_api.PluralLabel{One: "VMSS availability zone", Other: "VMSS availability zones"}},
		{Attribute: "azure.vmss.upgrade-policy.mode", Label: discovery_kit_api.PluralLabel{One: "VMSS upgrade policy mode", Other: "VMSS upgrade policy modes"}},
		{Attribute: "azure.vmss.platform-fault-domain-count", Label: discovery_kit_api.PluralLabel{One: "VMSS platform fault domain count", Other: "VMSS platform fault domain counts"}},
		{Attribute: "azure.vmss.single-placement-group", Label: discovery_kit_api.PluralLabel{One: "VMSS single placement group", Other: "VMSS single placement group"}},
		{Attribute: "azure.vmss.zone-balance", Label: discovery_kit_api.PluralLabel{One: "VMSS zone balance", Other: "VMSS zone balance"}},
		{Attribute: "azure.vmss.orchestration-mode", Label: discovery_kit_api.PluralLabel{One: "VMSS orchestration mode", Other: "VMSS orchestration modes"}},
		{Attribute: "azure.vmss.spot.priority", Label: discovery_kit_api.PluralLabel{One: "VMSS spot priority", Other: "VMSS spot priorities"}},
		{Attribute: "azure.vmss.spot.eviction-policy", Label: discovery_kit_api.PluralLabel{One: "VMSS spot eviction policy", Other: "VMSS spot eviction policies"}},
		{Attribute: "azure.vmss.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "VMSS provisioning state", Other: "VMSS provisioning states"}},
	}
}

func (d *scaleSetDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllScaleSets(ctx, client)
}

func getAllScaleSets(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.Compute/virtualMachineScaleSets' | project id, name, type, resourceGroup, location, tags, properties, sku, zones, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get VMSS results")
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
		targets = append(targets, toScaleSetTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesScaleSet), nil
}

func toScaleSetTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	upgradePolicy := common.GetMapValue(properties, "upgradePolicy")
	vmProfile := common.GetMapValue(properties, "virtualMachineProfile")
	priorityProfile := common.GetMapValue(properties, "priorityMixPolicy")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.vmss.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(sku, "name"); v != "" {
		attributes["azure.vmss.sku.name"] = []string{v}
	}
	if v := stringFromMap(sku, "tier"); v != "" {
		attributes["azure.vmss.sku.tier"] = []string{v}
	}
	if v, ok := sku["capacity"].(float64); ok {
		attributes["azure.vmss.sku.capacity"] = []string{strconv.Itoa(int(v))}
	}
	if zones := topLevelStringSlice(items, "zones"); len(zones) > 0 {
		sort.Strings(zones)
		attributes["azure.vmss.zones"] = zones
	}
	if v := stringFromMap(upgradePolicy, "mode"); v != "" {
		attributes["azure.vmss.upgrade-policy.mode"] = []string{v}
	}
	if v, ok := properties["platformFaultDomainCount"].(float64); ok {
		attributes["azure.vmss.platform-fault-domain-count"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := properties["singlePlacementGroup"].(bool); ok {
		attributes["azure.vmss.single-placement-group"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["zoneBalance"].(bool); ok {
		attributes["azure.vmss.zone-balance"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(properties, "orchestrationMode"); v != "" {
		attributes["azure.vmss.orchestration-mode"] = []string{v}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.vmss.provisioning-state"] = []string{v}
	}
	if v := stringFromMap(vmProfile, "priority"); v != "" {
		attributes["azure.vmss.spot.priority"] = []string{v}
	}
	if v := stringFromMap(vmProfile, "evictionPolicy"); v != "" {
		attributes["azure.vmss.spot.eviction-policy"] = []string{v}
	}
	_ = priorityProfile // reserved; could expose ratios in a follow-up

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.vmss.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDScaleSet,
		Label:      name,
		Attributes: attributes,
	}
}

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func topLevelStringSlice(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

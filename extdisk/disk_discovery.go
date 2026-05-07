/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extdisk

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

const (
	TargetIDDisk = "com.steadybit.extension_azure.disk"
	targetIcon   = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNMTIgM2M0Ljk3IDAgOSAxLjM0IDkgM3YxMmMwIDEuNjYtNC4wMyAzLTkgM3MtOS0xLjM0LTktM1Y2YzAtMS42NiA0LjAzLTMgOS0zem0wIDJjLTMuOSAwLTcgLjg5LTcgMnMzLjEgMiA3IDIgNy0uODkgNy0yLTMuMS0yLTctMnoiIGZpbGw9ImN1cnJlbnRDb2xvciIvPjwvc3ZnPg=="
)

type diskDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*diskDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*diskDiscovery)(nil)
)

func NewDiskDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&diskDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *diskDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDDisk,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *diskDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDDisk,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Managed Disk", Other: "Azure Managed Disks"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.disk.sku-name"},
				{Attribute: "azure.disk.size-gb"},
				{Attribute: "azure.disk.encryption.type"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *diskDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.disk.name", Label: discovery_kit_api.PluralLabel{One: "Disk name", Other: "Disk names"}},
		{Attribute: "azure.disk.sku-name", Label: discovery_kit_api.PluralLabel{One: "Disk SKU", Other: "Disk SKUs"}},
		{Attribute: "azure.disk.size-gb", Label: discovery_kit_api.PluralLabel{One: "Disk size (GiB)", Other: "Disk sizes (GiB)"}},
		{Attribute: "azure.disk.iops-read-write", Label: discovery_kit_api.PluralLabel{One: "Disk IOPS (read/write)", Other: "Disk IOPS (read/write)"}},
		{Attribute: "azure.disk.mbps-read-write", Label: discovery_kit_api.PluralLabel{One: "Disk throughput (MB/s)", Other: "Disk throughput (MB/s)"}},
		{Attribute: "azure.disk.os-type", Label: discovery_kit_api.PluralLabel{One: "Disk OS type", Other: "Disk OS types"}},
		{Attribute: "azure.disk.disk-state", Label: discovery_kit_api.PluralLabel{One: "Disk state", Other: "Disk states"}},
		{Attribute: "azure.disk.encryption.type", Label: discovery_kit_api.PluralLabel{One: "Disk encryption type", Other: "Disk encryption types"}},
		{Attribute: "azure.disk.encryption.set-id", Label: discovery_kit_api.PluralLabel{One: "Disk encryption set ID", Other: "Disk encryption set IDs"}},
		{Attribute: "azure.disk.public-network-access", Label: discovery_kit_api.PluralLabel{One: "Disk public network access", Other: "Disk public network access"}},
		{Attribute: "azure.disk.network-access-policy", Label: discovery_kit_api.PluralLabel{One: "Disk network access policy", Other: "Disk network access policies"}},
		{Attribute: "azure.disk.zones", Label: discovery_kit_api.PluralLabel{One: "Disk zone", Other: "Disk zones"}},
		{Attribute: "azure.disk.max-shares", Label: discovery_kit_api.PluralLabel{One: "Disk max shares", Other: "Disk max shares"}},
		{Attribute: "azure.disk.managed-by", Label: discovery_kit_api.PluralLabel{One: "Disk attached to", Other: "Disk attached to"}},
		{Attribute: "azure.disk.bursting-enabled", Label: discovery_kit_api.PluralLabel{One: "Disk bursting enabled", Other: "Disk bursting enabled"}},
	}
}

func (d *diskDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllDisks(ctx, client)
}

func getAllDisks(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.Compute/disks' | project id, name, type, resourceGroup, location, tags, properties, sku, zones, managedBy, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get disk results")
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
		targets = append(targets, toDiskTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesManagedDisk), nil
}

func toDiskTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	encryption := common.GetMapValue(properties, "encryption")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.disk.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(sku, "name"); v != "" {
		attributes["azure.disk.sku-name"] = []string{v}
	}
	if v, ok := properties["diskSizeGB"].(float64); ok {
		attributes["azure.disk.size-gb"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := properties["diskIOPSReadWrite"].(float64); ok {
		attributes["azure.disk.iops-read-write"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := properties["diskMBpsReadWrite"].(float64); ok {
		attributes["azure.disk.mbps-read-write"] = []string{strconv.Itoa(int(v))}
	}
	if v := stringFromMap(properties, "osType"); v != "" {
		attributes["azure.disk.os-type"] = []string{v}
	}
	if v := stringFromMap(properties, "diskState"); v != "" {
		attributes["azure.disk.disk-state"] = []string{v}
	}
	if v := stringFromMap(encryption, "type"); v != "" {
		attributes["azure.disk.encryption.type"] = []string{v}
	}
	if v := stringFromMap(encryption, "diskEncryptionSetId"); v != "" {
		attributes["azure.disk.encryption.set-id"] = []string{v}
	}
	if v := stringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.disk.public-network-access"] = []string{v}
	}
	if v := stringFromMap(properties, "networkAccessPolicy"); v != "" {
		attributes["azure.disk.network-access-policy"] = []string{v}
	}
	if v, ok := properties["maxShares"].(float64); ok {
		attributes["azure.disk.max-shares"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := properties["burstingEnabled"].(bool); ok {
		attributes["azure.disk.bursting-enabled"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(items, "managedBy"); v != "" {
		attributes["azure.disk.managed-by"] = []string{v}
	}
	if zones := stringSliceFromMap(items, "zones"); len(zones) > 0 {
		sort.Strings(zones)
		attributes["azure.disk.zones"] = zones
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.disk.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDDisk,
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

func stringSliceFromMap(m map[string]any, key string) []string {
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

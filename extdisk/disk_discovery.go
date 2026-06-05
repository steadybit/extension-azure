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

	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	TargetIDDisk = "com.steadybit.extension_azure.disk"
	targetIcon   = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZmlsbC1ydWxlPSJldmVub2RkIiBjbGlwLXJ1bGU9ImV2ZW5vZGQiIGQ9Ik01LjU3OTY0IDEyLjUxMThDNy4yMzE1NSAxMy4yMzEzIDkuNTQxMzQgMTMuNjgxIDEyLjA0ODQgMTMuNjgxQzE0LjU2MzIgMTMuNjgxIDE2LjgzNDMgMTMuMjM4IDE4LjQ2NyAxMi41MzI1QzIwLjAxNTcgMTMuMjA0NCAyMC45NzQyIDE0LjExNTkgMjAuOTk3NCAxNS4xMjI1QzIwLjk5NzYgMTUuMTEwMyAyMSAxNS4wOTc5IDIxIDE1LjA4NTdWMTcuMzE4OUMyMC44MzIgMTkuMzQzNSAxNi44NzIgMjEgMTIgMjFDNy4xMjgwNiAyMSAzLjAwMDA2IDE5LjMwNjkgMy4wMDAwNSAxNy4yMjFWMTUuMDg1N0MyLjk5OTc3IDE1LjA5NzkgMy4wMDE4MSAxNS4xMTAzIDMuMDAxODEgMTUuMTIyNUMzLjAyNTE2IDE0LjEwNTggNC4wMDM1MiAxMy4xODU2IDUuNTc5NjQgMTIuNTExOFpNMTIuMjI3NyAxNC4yODg1QzEwLjU3OCAxNC4yODg2IDkuMjQwOTQgMTQuNjYxOSA5LjI0MDI3IDE1LjEyMjVDOS4yNDA0OSAxNS41ODMyIDEwLjU3NzcgMTUuOTU3NCAxMi4yMjc3IDE1Ljk1NzRDMTMuODc3OCAxNS45NTc0IDE1LjIxNTcgMTUuNTgzMyAxNS4yMTU5IDE1LjEyMjVDMTUuMjE1MyAxNC42NjE5IDEzLjg3NzUgMTQuMjg4NSAxMi4yMjc3IDE0LjI4ODVaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPHBhdGggZmlsbC1ydWxlPSJldmVub2RkIiBjbGlwLXJ1bGU9ImV2ZW5vZGQiIGQ9Ik0xMiAzQzE2LjkzMTkgMy4wMDAwMSAyMC45MzU0IDQuNjY1NjcgMjAuOTk3NCA2LjczMDQ5QzIwLjk5NzcgNi43MTgwNCAyMSA2LjcwNTIzIDIxIDYuNjkyNzVWOC45MjYwMUMyMC44MzIgMTAuOTUwNiAxNi44NzIgMTIuNjA3MSAxMiAxMi42MDcxQzcuMTI4MDYgMTIuNjA3MSAzLjAwMDA1IDEwLjkxNCAzLjAwMDA1IDguODI4MDVWNi42OTI3NUMyLjk5OTY1IDYuNzEwMzIgMy4wMDE2NCA2LjcyODI1IDMuMDAxODEgNi43NDU3N0MzLjA0NDM1IDQuNjczODggNy4wNTU5MyAzLjAwMDAxIDEyIDNaTTEyLjIyNzcgNS44OTU2QzEwLjU3NzYgNS44OTU2NiA5LjI0MDI3IDYuMjY5NzEgOS4yNDAyNyA2LjczMDQ5QzkuMjQxMjcgNy4xOTEwMyAxMC41NzgyIDcuNTY0NDIgMTIuMjI3NyA3LjU2NDQ4QzEzLjg3NzMgNy41NjQ0OCAxNS4yMTQ5IDcuMTkxMDYgMTUuMjE1OSA2LjczMDQ5QzE1LjIxNTkgNi4yNjk2OCAxMy44Nzc5IDUuODk1NiAxMi4yMjc3IDUuODk1NloiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8L3N2Zz4K"
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
	targets, err := common.DiscoverViaResourceGraph(ctx, client,
		"Resources | where type =~ 'Microsoft.Compute/disks' | project id, name, type, resourceGroup, location, tags, properties, sku, zones, managedBy, subscriptionId",
		toDiskTarget)
	if err != nil {
		log.Error().Err(err).Msg("failed to get disk results")
		return nil, err
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
	attributes["azure.subscription.id"] = []string{common.StringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{common.StringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{common.StringFromMap(items, "location")}

	if v := common.StringFromMap(sku, "name"); v != "" {
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
	if v := common.StringFromMap(properties, "osType"); v != "" {
		attributes["azure.disk.os-type"] = []string{v}
	}
	if v := common.StringFromMap(properties, "diskState"); v != "" {
		attributes["azure.disk.disk-state"] = []string{v}
	}
	if v := common.StringFromMap(encryption, "type"); v != "" {
		attributes["azure.disk.encryption.type"] = []string{v}
	}
	if v := common.StringFromMap(encryption, "diskEncryptionSetId"); v != "" {
		attributes["azure.disk.encryption.set-id"] = []string{v}
	}
	if v := common.StringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.disk.public-network-access"] = []string{v}
	}
	if v := common.StringFromMap(properties, "networkAccessPolicy"); v != "" {
		attributes["azure.disk.network-access-policy"] = []string{v}
	}
	if v, ok := properties["maxShares"].(float64); ok {
		attributes["azure.disk.max-shares"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := properties["burstingEnabled"].(bool); ok {
		attributes["azure.disk.bursting-enabled"] = []string{strconv.FormatBool(v)}
	}
	if v := common.StringFromMap(items, "managedBy"); v != "" {
		attributes["azure.disk.managed-by"] = []string{v}
	}
	if zones := common.StringSliceFromMap(items, "zones"); len(zones) > 0 {
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


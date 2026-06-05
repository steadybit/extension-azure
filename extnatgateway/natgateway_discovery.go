/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extnatgateway

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
	TargetIDNatGateway = "com.steadybit.extension_azure.nat-gateway"
	targetIcon         = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTExLjIwNjEgMTcuMDcyNEMxMS42MDAxIDE2LjY3NzYgMTIuMjQyOCAxNi42ODAzIDEyLjY0MTcgMTcuMDc4MkMxMy4wNDAzIDE3LjQ3NjMgMTMuMDQ0NCAxOC4xMTkxIDEyLjY1MDUgMTguNTEzOEMxMi4yNTY1IDE4LjkwODQgMTEuNjEzNyAxOC45MDU3IDExLjIxNDkgMTguNTA3OUMxMC44MTYxIDE4LjExIDEwLjgxMjEgMTcuNDY3MiAxMS4yMDYxIDE3LjA3MjRaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPHBhdGggZD0iTTE2LjkzMDggMTEuMzgxOEMxNy4zMjUgMTAuOTg3MSAxNy45NjY2IDEwLjk4OTggMTguMzY1NCAxMS4zODc3QzE4Ljc2NDMgMTEuNzg1NiAxOC43NjgzIDEyLjQyNzUgMTguMzc0MiAxMi44MjIzQzE3Ljk4MDMgMTMuMjE3IDE3LjMzODUgMTMuMjE0OSAxNi45Mzk2IDEyLjgxNzRDMTYuNTQwOCAxMi40MTk0IDE2LjUzNjkgMTEuNzc2NiAxNi45MzA4IDExLjM4MThaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPHBhdGggZD0iTTUuNjE2MTMgMTEuMzc5OUM2LjAxMDEgMTAuOTg1MiA2LjY1Mjg4IDEwLjk4OCA3LjA1MTcxIDExLjM4NTdDNy40NTA1NiAxMS43ODM3IDcuNDU0NTIgMTIuNDI2NSA3LjA2MDUgMTIuODIxM0M2LjY2NjQ3IDEzLjIxNiA2LjAyMzc1IDEzLjIxMzQgNS42MjQ5MiAxMi44MTU0QzUuMjI2MyAxMi40MTc2IDUuMjIyMTggMTEuNzc0NiA1LjYxNjEzIDExLjM3OTlaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPHBhdGggZmlsbC1ydWxlPSJldmVub2RkIiBjbGlwLXJ1bGU9ImV2ZW5vZGQiIGQ9Ik0xMS41Mjg0IDIuMTk1MDhDMTEuNzg4NiAxLjkzNDkgMTIuMjEwNiAxLjkzNDg4IDEyLjQ3MDggMi4xOTUwOEwyMS44MDUgMTEuNTI4M0MyMi4wNjUgMTEuNzg4NSAyMi4wNjUgMTIuMjEwNSAyMS44MDUgMTIuNDcwN0wxMi40NzA4IDIxLjgwNDlDMTIuMjEwNiAyMi4wNjQ5IDExLjc4ODUgMjIuMDY0OSAxMS41Mjg0IDIxLjgwNDlMMi4xOTUxNSAxMi40NzA3QzEuOTM0OTUgMTIuMjEwNSAxLjkzNDk1IDExLjc4ODUgMi4xOTUxNSAxMS41MjgzTDExLjUyODQgMi4xOTUwOFpNMTEuODYwNCA0Ljk4MTI3QzExLjg0IDQuOTg5ODcgMTEuODIxNCA1LjAwMjU5IDExLjgwNTcgNS4wMTgzOEw5LjU1MTc2IDcuMjcyMzRDOS41Mjc3NyA3LjI5NTk1IDkuNTEwNjkgNy4zMjYyNiA5LjUwMzkxIDcuMzU5MjZDOS40OTcyIDcuMzkyMTggOS41MDA4IDcuNDI2OCA5LjUxMzY4IDcuNDU3ODlDOS41MjY0OSA3LjQ4ODgzIDkuNTQ4NDMgNy41MTU1MiA5LjU3NjE4IDcuNTM0MDZDOS42MDQxNiA3LjU1MjU5IDkuNjM3MzcgNy41NjI1MiA5LjY3MDkxIDcuNTYyMzlIMTAuOTUwMkMxMC45ODIxIDcuNTYyNDYgMTEuMDEyNiA3LjU3NDk0IDExLjAzNTIgNy41OTc1NEMxMS4wNTc2IDcuNjIwMDQgMTEuMDcwMyA3LjY1MDY5IDExLjA3MDQgNy42ODI1MVYxNS43MjM3TDguMzA0NjcgMTIuOTU4SDkuNzcyNDdWMTEuMjQ0MUg3Ljg5NTQ4QzcuODE1NiAxMS4xMDAxIDcuNzE1MzQgMTAuOTYzOSA3LjU5Mjc0IDEwLjg0MThDNi44OTM1MiAxMC4xNDU1IDUuNzY0ODggMTAuMTQ1MSA1LjA3MjE3IDEwLjg0MDhDNC4zNzk4NCAxMS41MzYzIDQuMzg1IDEyLjY2NDIgNS4wODM4OSAxMy4zNjA0QzUuNTM5MTkgMTMuODEzNyA2LjE3NjMxIDEzLjk2OTkgNi43NTY3OCAxMy44MzNMMTAuMjE0OSAxNy4yOTExQzEwLjA0MTQgMTcuODk1NSAxMC4xOTUgMTguNTc1IDEwLjY3MzkgMTkuMDUxOUMxMS4zNzMxIDE5Ljc0OCAxMi41MDA3IDE5Ljc0ODQgMTMuMTkzNCAxOS4wNTI5QzEzLjY2NTIgMTguNTc4OSAxMy44MTI0IDE3LjkwNDkgMTMuNjM3OCAxNy4zMDI4TDE3LjEzNjkgMTMuODA0N0MxNy43NDc4IDEzLjk5MTYgMTguNDM3OSAxMy44NDQ3IDE4LjkxODIgMTMuMzYyM0MxOS42MTA0IDEyLjY2NjkgMTkuNjA1MSAxMS41Mzg5IDE4LjkwNjUgMTAuODQyN0MxOC4yMDc0IDEwLjE0NjUgMTcuMDc4NiAxMC4xNDYxIDE2LjM4NTkgMTAuODQxOEMxNi4yNjQyIDEwLjk2NCAxNi4xNjQ3IDExLjA5OTggMTYuMDg2MSAxMS4yNDQxSDE0LjExNjNWMTIuOTU4SDE1LjU2MDdMMTIuNzkwMSAxNS43Mjc2VjcuNjgyNTFDMTIuNzkgNy42NTE5NyAxMi44MDE2IDcuNjIyNzIgMTIuODIyMyA3LjYwMDQ3QzEyLjg0MzMgNy41NzgwNSAxMi44NzE5IDcuNTY0MzQgMTIuOTAyNCA3LjU2MjM5SDE0LjE3NjlDMTQuMjEwNyA3LjU2MzI2IDE0LjI0NSA3LjU1MzQgMTQuMjczNiA3LjUzNTA0QzE0LjMwMTggNy41MTY2NyAxNC4zMjM4IDcuNDg5OTYgMTQuMzM3IDcuNDU4ODdDMTQuMzUwMiA3LjQyNzczIDE0LjM1NDMgNy4zOTM0NCAxNC4zNDc4IDcuMzYwMjNDMTQuMzQxMSA3LjMyNzAzIDE0LjMyNDEgNy4yOTYxOSAxNC4yOTk5IDcuMjcyMzRMMTIuMDQ2IDUuMDE4MzhDMTIuMDMwNSA1LjAwMjY0IDEyLjAxMTYgNC45ODk4NyAxMS45OTEzIDQuOTgxMjdDMTEuOTcwNyA0Ljk3MjcgMTEuOTQ4MiA0Ljk2NzY0IDExLjkyNTggNC45Njc2QzExLjkwMzYgNC45Njc2IDExLjg4MSA0Ljk3MjY5IDExLjg2MDQgNC45ODEyN1oiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8L3N2Zz4K"
)

type natGatewayDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*natGatewayDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*natGatewayDiscovery)(nil)
)

func NewNatGatewayDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&natGatewayDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *natGatewayDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDNatGateway,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *natGatewayDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDNatGateway,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure NAT Gateway", Other: "Azure NAT Gateways"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.nat-gateway.zones"},
				{Attribute: "azure.nat-gateway.idle-timeout-in-minutes"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *natGatewayDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.nat-gateway.name", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway name", Other: "NAT Gateway names"}},
		{Attribute: "azure.nat-gateway.sku-name", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway SKU", Other: "NAT Gateway SKUs"}},
		{Attribute: "azure.nat-gateway.zones", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway zone", Other: "NAT Gateway zones"}},
		{Attribute: "azure.nat-gateway.idle-timeout-in-minutes", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway idle timeout (minutes)", Other: "NAT Gateway idle timeouts (minutes)"}},
		{Attribute: "azure.nat-gateway.public-ip-addresses", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway Public IP", Other: "NAT Gateway Public IPs"}},
		{Attribute: "azure.nat-gateway.public-ip-prefixes", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway Public IP prefix", Other: "NAT Gateway Public IP prefixes"}},
		{Attribute: "azure.nat-gateway.subnets", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway subnet", Other: "NAT Gateway subnets"}},
		{Attribute: "azure.nat-gateway.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "NAT Gateway provisioning state", Other: "NAT Gateway provisioning states"}},
	}
}

func (d *natGatewayDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllNatGateways(ctx, client)
}

func getAllNatGateways(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.Network/natGateways' | project id, name, type, resourceGroup, location, tags, properties, sku, zones, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get NAT Gateway results")
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
		targets = append(targets, toNatGatewayTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesNatGateway), nil
}

func toNatGatewayTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.nat-gateway.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(sku, "name"); v != "" {
		attributes["azure.nat-gateway.sku-name"] = []string{v}
	}
	if zones := stringSliceFromMap(items, "zones"); len(zones) > 0 {
		sort.Strings(zones)
		attributes["azure.nat-gateway.zones"] = zones
	}
	if v, ok := properties["idleTimeoutInMinutes"].(float64); ok {
		attributes["azure.nat-gateway.idle-timeout-in-minutes"] = []string{strconv.Itoa(int(v))}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.nat-gateway.provisioning-state"] = []string{v}
	}

	if ips := referenceIds(properties, "publicIpAddresses"); len(ips) > 0 {
		sort.Strings(ips)
		attributes["azure.nat-gateway.public-ip-addresses"] = ips
	}
	if prefixes := referenceIds(properties, "publicIpPrefixes"); len(prefixes) > 0 {
		sort.Strings(prefixes)
		attributes["azure.nat-gateway.public-ip-prefixes"] = prefixes
	}
	if subnets := referenceIds(properties, "subnets"); len(subnets) > 0 {
		sort.Strings(subnets)
		attributes["azure.nat-gateway.subnets"] = subnets
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.nat-gateway.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDNatGateway,
		Label:      name,
		Attributes: attributes,
	}
}

// referenceIds extracts the resource ARM IDs from a list of {id: ...} ARM references.
func referenceIds(m map[string]any, key string) []string {
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
		ref, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := ref["id"].(string); ok && id != "" {
			out = append(out, id)
		}
	}
	return out
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

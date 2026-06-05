/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extloadbalancer

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
	TargetIDLoadBalancer = "com.steadybit.extension_azure.load-balancer"
	targetIcon           = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTEyLjAxMDggMTAuMTg5NEMxMy4wMDQ3IDEwLjE4OTQgMTMuODEwNiAxMC45OTUzIDEzLjgxMDYgMTEuOTg5MkMxMy44MTA0IDEyLjk4MjkgMTMuMDA0NyAxMy43ODg5IDEyLjAxMDggMTMuNzg4OUMxMS4wMTcyIDEzLjc4ODggMTAuMjExMiAxMi45ODI4IDEwLjIxMTEgMTEuOTg5MkMxMC4yMTExIDEwLjk5NTMgMTEuMDE3MSAxMC4xODk2IDEyLjAxMDggMTAuMTg5NFoiIGZpbGw9ImN1cnJlbnRDb2xvciIvPgo8cGF0aCBmaWxsLXJ1bGU9ImV2ZW5vZGQiIGNsaXAtcnVsZT0iZXZlbm9kZCIgZD0iTTEyIDJDMTIuMDg5IDIuMDAwMDEgMTIuMTc3MiAyLjAxODUyIDEyLjI1OTMgMi4wNTMxNkMxMi4zNDExIDIuMDg3NzMgMTIuNDE1MyAyLjEzODE1IDEyLjQ3NzMgMi4yMDE3OEwyMS43OTgyIDExLjUyMjdDMjEuODYxOCAxMS41ODQ3IDIxLjkxMjMgMTEuNjU4OSAyMS45NDY4IDExLjc0MDdDMjEuOTgxNSAxMS44MjI4IDIyIDExLjkxMSAyMiAxMkMyMiAxMi4wODkxIDIxLjk4MTUgMTIuMTc3MiAyMS45NDY4IDEyLjI1OTNDMjEuOTEyMyAxMi4zNDExIDIxLjg2MTggMTIuNDE1MyAyMS43OTgyIDEyLjQ3NzNMMTIuNDY2NSAyMS44MDkxQzEyLjM0MTkgMjEuOTMxMiAxMi4xNzQ0IDIyIDEyIDIyQzExLjgyNTYgMjIgMTEuNjU4MSAyMS45MzExIDExLjUzMzUgMjEuODA5MUwyLjIwMTc4IDEyLjQ3NzNDMi4xMzgxNCAxMi40MTUzIDIuMDg3NzMgMTIuMzQxMSAyLjA1MzE2IDEyLjI1OTNDMi4wMTg1MiAxMi4xNzcyIDIgMTIuMDg5MSAyIDEyQzIgMTEuOTExIDIuMDE4NTIgMTEuODIyOCAyLjA1MzE2IDExLjc0MDdDMi4wODc3NCAxMS42NTg5IDIuMTM4MTQgMTEuNTg0NyAyLjIwMTc4IDExLjUyMjdMMTEuNTIyNyAyLjIwMTc4QzExLjU4NDcgMi4xMzgxNSAxMS42NTg5IDIuMDg3NzMgMTEuNzQwNyAyLjA1MzE2QzExLjgyMjggMi4wMTg1MiAxMS45MTA5IDIgMTIgMlpNMTIgNC4wNjc2OUMxMS45NjczIDQuMDY3NyAxMS45MzU1IDQuMDc5NTQgMTEuOTExIDQuMTAxMzJMOS41NTU4NyA2LjQ0NTY1QzkuNTM0MjggNi40NjAyMyA5LjUxODU5IDYuNDgyNDUgOS41MTEzOSA2LjUwNzQ5QzkuNTA0MjkgNi41MzI0OSA5LjUwNjI1IDYuNTU5NjYgOS41MTY4MSA2LjU4MzQyQzkuNTI3NCA2LjYwNjk0IDkuNTQ1NzYgNi42MjYyNiA5LjU2ODg5IDYuNjM3NjdDOS41OTIzNCA2LjY0OTEyIDkuNjE5NSA2LjY1MTU1IDkuNjQ0ODMgNi42NDUyNkgxMS4wMjI2QzExLjAzODYgNi42NDUyNyAxMS4wNTQ0IDYuNjQ4ODkgMTEuMDY5MiA2LjY1NTAyQzExLjA4NCA2LjY2MTE2IDExLjA5OCA2LjY2OTcyIDExLjEwOTQgNi42ODEwNkMxMS4xMjA3IDYuNjkyMzkgMTEuMTI5MyA2LjcwNjQgMTEuMTM1NCA2LjcyMTJDMTEuMTQxNSA2LjczNTk4IDExLjE0NTIgNi43NTE4NSAxMS4xNDUyIDYuNzY3ODVWOC45ODk1OUMxMS4xNDUyIDguOTkzNjYgMTEuMTQ2OSA4Ljk5NzUgMTEuMTQ3MyA5LjAwMTUyQzEwLjc4MzEgOS4xMDU3OCAxMC40MzkyIDkuMjc1ODkgMTAuMTM0MSA5LjUwNTk3QzkuNjcyNzYgOS44NTM5MSA5LjMxNzI3IDEwLjMyNDEgOS4xMDg5MiAxMC44NjMxQzkuMTAxOSAxMC44ODEzIDkuMDk2MDYgMTAuOTAwMiA5LjA4OTM5IDEwLjkxODRDOS4wODY5MSAxMC45MTYyIDkuMDg1NTYgMTAuOTExOCA5LjA4Mjg4IDEwLjkwOTdDOS4wNjg1MyAxMC44OTkgOS4wNTE3NSAxMC44OTIgOS4wMzQwNiAxMC44ODkxSDYuODEyMzJDNi43OTY5MyAxMC44OTA3IDYuNzgxNDcgMTAuODg4NSA2Ljc2Njc2IDEwLjg4MzdDNi43NTIwNSAxMC44Nzg4IDYuNzM4MTEgMTAuODcxMyA2LjcyNjYyIDEwLjg2MDlDNi43MTUxIDEwLjg1MDQgNi43MDU3NiAxMC44MzcyIDYuNjk5NSAxMC44MjNDNi42OTMzNyAxMC44MDg5IDYuNjg5NzIgMTAuNzkzOCA2LjY4OTc0IDEwLjc3ODVWOS40MzQzN0M2LjcwMTUyIDkuNDAxOTYgNi42OTk5OCA5LjM2NTgxIDYuNjg1NCA5LjMzNDU2QzYuNjcwODcgOS4zMDM0NSA2LjY0NDk0IDkuMjc5MTMgNi42MTI3MSA5LjI2NzNDNi41ODAzMSA5LjI1NTUyIDYuNTQ0MTUgOS4yNTcwNyA2LjUxMjkxIDkuMjcxNjRDNi40ODE2NiA5LjI4NjIzIDYuNDU3NDMgOS4zMTMgNi40NDU2NSA5LjM0NTQxTDQuMTEzMjYgMTEuNzExNEM0LjA5MjE3IDExLjczNDEgNC4wNzk2MyAxMS43NjQgNC4wNzk2MyAxMS43OTVDNC4wNzk3NCAxMS44MjU3IDQuMDkyMjggMTEuODU0OSA0LjExMzI2IDExLjg3NzRMNi40NDU2NSAxNC4yMjE3QzYuNDYyNzMgMTQuMjM5OSA2LjQ4NTI4IDE0LjI1MjQgNi41MDk2NSAxNC4yNTc1QzYuNTMzOTQgMTQuMjYyNiA2LjU1OTQ3IDE0LjI2MDcgNi41ODIzNCAxNC4yNTFDNi42MDUyNyAxNC4yNDE0IDYuNjI0MzQgMTQuMjI0MyA2LjYzNzY3IDE0LjIwMzNDNi42NTEgMTQuMTgyMyA2LjY1ODIzIDE0LjE1NzcgNi42NTcxOSAxNC4xMzI4VjEyLjc2N0M2LjY1NzE5IDEyLjczNDYgNi42NzAwOCAxMi43MDMxIDYuNjkyOTkgMTIuNjgwMkM2LjcxNTgzIDEyLjY1NzQgNi43NDY0NiAxMi42NDQ1IDYuNzc4NjkgMTIuNjQ0NEg4Ljk3NDRDOS4wOTE1OCAxMy4xNzg3IDkuMzQ2ODMgMTMuNjczOCA5LjcxNzUxIDE0LjA3NzVDMTAuMTA4MyAxNC41MDI5IDEwLjYxMDQgMTQuODEwNiAxMS4xNjY4IDE0Ljk2NTlWMTYuMDIxNUMxMC43ODg4IDE2LjIxNjkgMTAuNDg3NSAxNi41MzM2IDEwLjMxMDkgMTYuOTIwOEMxMC4xMzQ0IDE3LjMwODEgMTAuMDkyMiAxNy43NDM5IDEwLjE5MjcgMTguMTU3NUMxMC4yOTMyIDE4LjU3MTIgMTAuNTMwNSAxOC45MzkxIDEwLjg2NTMgMTkuMjAyMkMxMS4yIDE5LjQ2NTMgMTEuNjEzMyAxOS42MDkgMTIuMDM5MSAxOS42MDlDMTIuNDY0OCAxOS42MDkgMTIuODc4MiAxOS40NjUzIDEzLjIxMjggMTkuMjAyMkMxMy41NDc1IDE4LjkzOTEgMTMuNzgzOCAxOC41NzEyIDEzLjg4NDQgMTguMTU3NUMxMy45ODQ5IDE3Ljc0MzggMTMuOTQzOCAxNy4zMDgyIDEzLjc2NzIgMTYuOTIwOEMxMy41OTA2IDE2LjUzMzYgMTMuMjg5MyAxNi4yMTY5IDEyLjkxMTMgMTYuMDIxNVYxNC45MTA2QzEzLjUzODIgMTQuNzIxNSAxNC4wODg2IDE0LjMzNzIgMTQuNDgyMSAxMy44MTM4QzE0Ljc1MDggMTMuNDU2NSAxNC45MzMyIDEzLjA0NjIgMTUuMDI2NyAxMi42MTRDMTUuMDM1MyAxMi42MjE3IDE1LjA0MzIgMTIuNjI5OSAxNS4wNTM4IDEyLjYzNDZDMTUuMDY4MSAxMi42NDA5IDE1LjA4MzggMTIuNjQ0NSAxNS4wOTk0IDEyLjY0NDRIMTcuMzIxMUMxNy4zNTM1IDEyLjY0NDQgMTcuMzg1IDEyLjY1NzMgMTcuNDA3OSAxMi42ODAyQzE3LjQzMDYgMTIuNzAzMSAxNy40NDM3IDEyLjczNDcgMTcuNDQzNyAxMi43NjdWMTQuMTQzNkMxNy40NDY1IDE0LjE2NTQgMTcuNDU1MSAxNC4xODYgMTcuNDY4NiAxNC4yMDMzQzE3LjQ4MjEgMTQuMjIwNSAxNy40OTkzIDE0LjIzNDQgMTcuNTE5NiAxNC4yNDI0QzE3LjU0MDEgMTQuMjUwMyAxNy41NjMxIDE0LjI1MjUgMTcuNTg0NyAxNC4yNDg5QzE3LjYwNjMgMTQuMjQ1MiAxNy42MjY2IDE0LjIzNTggMTcuNjQzMyAxNC4yMjE3TDIwLjAwOTMgMTEuODY2NkMyMC4wMzA0IDExLjg0NCAyMC4wNDE5IDExLjgxMzkgMjAuMDQxOSAxMS43ODNDMjAuMDQxOCAxMS43NTI0IDIwLjAzMDIgMTEuNzIzMSAyMC4wMDkzIDExLjcwMDZMMTcuNjQzMyA5LjM0NTQxQzE3LjYyNTggOS4zMzA5MiAxNy42MDQgOS4zMjEzOCAxNy41ODE1IDkuMzE4MjlDMTcuNTU5MSA5LjMxNTI5IDE3LjUzNTkgOS4zMTg4OCAxNy41MTUzIDkuMzI4MDVDMTcuNDk0NyA5LjMzNzMgMTcuNDc3IDkuMzUxNzQgMTcuNDY0MyA5LjM3MDM2QzE3LjQ1MTUgOS4zODkxNSAxNy40NDQ3IDkuNDExNjUgMTcuNDQzNyA5LjQzNDM3VjEwLjgyM0MxNy40NDM3IDEwLjgzODMgMTcuNDQwMSAxMC44NTM0IDE3LjQzMzkgMTAuODY3NEMxNy40Mjc3IDEwLjg4MTcgMTcuNDE4MyAxMC44OTQ5IDE3LjQwNjggMTAuOTA1NEMxNy4zOTUzIDEwLjkxNTggMTcuMzgxNCAxMC45MjMzIDE3LjM2NjcgMTAuOTI4MkMxNy4zNTIgMTAuOTMzIDE3LjMzNjUgMTAuOTM1MSAxNy4zMjExIDEwLjkzMzZIMTUuMDk5NEMxNS4wODM4IDEwLjkzMzUgMTUuMDY4MSAxMC45MzcxIDE1LjA1MzggMTAuOTQzNEMxNS4wMzk5IDEwLjk0OTUgMTUuMDI3MiAxMC45NTgyIDE1LjAxNjkgMTAuOTY5NEMxNS4wMDY1IDEwLjk4MDkgMTQuOTk3OSAxMC45OTQ4IDE0Ljk5MzEgMTEuMDA5NUMxNC45ODgyIDExLjAyNDMgMTQuOTg3MiAxMS4wNDA3IDE0Ljk4ODcgMTEuMDU2MlYxMS4xNjI1QzE0LjkxMTkgMTAuODgyIDE0Ljc5NiAxMC42MTIzIDE0LjY0MjcgMTAuMzYxOUMxNC4zNDA4IDkuODY5MTMgMTMuOTA3MiA5LjQ3MDIzIDEzLjM5MDggOS4yMTA4OUMxMy4yMTc3IDkuMTI0MDEgMTMuMDM3MiA5LjA1NTQ0IDEyLjg1MjcgOS4wMDI2QzEyLjg1MzIgOC45OTgyMyAxMi44NTU5IDguOTk0MDMgMTIuODU1OSA4Ljk4OTU5VjYuNzY3ODVDMTIuODU1OSA2Ljc1MTg2IDEyLjg1ODUgNi43MzU5NyAxMi44NjQ2IDYuNzIxMkMxMi44NzA4IDYuNzA2MzcgMTIuODgwNCA2LjY5MjQxIDEyLjg5MTcgNi42ODEwNkMxMi45MDMgNi42Njk5NCAxMi45MTYyIDYuNjYxMDggMTIuOTMwOCA2LjY1NTAyQzEyLjk0NTYgNi42NDg5IDEyLjk2MTQgNi42NDUyOCAxMi45Nzc0IDYuNjQ1MjZIMTQuMzU1MkMxNC4zODA1IDYuNjUxNTcgMTQuNDA3NiA2LjY0OTEzIDE0LjQzMTEgNi42Mzc2N0MxNC40NTQzIDYuNjI2MjYgMTQuNDcyNiA2LjYwNjk1IDE0LjQ4MzIgNi41ODM0MkMxNC40OTM3IDYuNTU5NjYgMTQuNDk1NyA2LjUzMjQ5IDE0LjQ4ODYgNi41MDc0OUMxNC40ODE0IDYuNDgyNDMgMTQuNDY1NyA2LjQ2MDIzIDE0LjQ0NDEgNi40NDU2NUwxMi4wODkgNC4xMDEzMkMxMi4wNjQ1IDQuMDc5NTUgMTIuMDMyNyA0LjA2NzY5IDEyIDQuMDY3NjlaIiBmaWxsPSJjdXJyZW50Q29sb3IiLz4KPC9zdmc+Cg=="
)

type loadBalancerDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*loadBalancerDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*loadBalancerDiscovery)(nil)
)

func NewLoadBalancerDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&loadBalancerDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *loadBalancerDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDLoadBalancer,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *loadBalancerDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDLoadBalancer,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Load Balancer", Other: "Azure Load Balancers"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.load-balancer.sku-name"},
				{Attribute: "azure.load-balancer.frontend.public-exposed"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *loadBalancerDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.load-balancer.name", Label: discovery_kit_api.PluralLabel{One: "Load Balancer name", Other: "Load Balancer names"}},
		{Attribute: "azure.load-balancer.sku-name", Label: discovery_kit_api.PluralLabel{One: "Load Balancer SKU name", Other: "Load Balancer SKU names"}},
		{Attribute: "azure.load-balancer.sku-tier", Label: discovery_kit_api.PluralLabel{One: "Load Balancer SKU tier", Other: "Load Balancer SKU tiers"}},
		{Attribute: "azure.load-balancer.frontend.public-exposed", Label: discovery_kit_api.PluralLabel{One: "Load Balancer has public frontend", Other: "Load Balancer has public frontend"}},
		{Attribute: "azure.load-balancer.frontend.zones", Label: discovery_kit_api.PluralLabel{One: "Load Balancer frontend zone", Other: "Load Balancer frontend zones"}},
		{Attribute: "azure.load-balancer.backend-pool-count", Label: discovery_kit_api.PluralLabel{One: "Load Balancer backend pool count", Other: "Load Balancer backend pool counts"}},
		{Attribute: "azure.load-balancer.load-balancing-rule-count", Label: discovery_kit_api.PluralLabel{One: "Load Balancer rule count", Other: "Load Balancer rule counts"}},
		{Attribute: "azure.load-balancer.outbound-rule-count", Label: discovery_kit_api.PluralLabel{One: "Load Balancer outbound rule count", Other: "Load Balancer outbound rule counts"}},
		{Attribute: "azure.load-balancer.probe-count", Label: discovery_kit_api.PluralLabel{One: "Load Balancer probe count", Other: "Load Balancer probe counts"}},
		{Attribute: "azure.load-balancer.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "Load Balancer provisioning state", Other: "Load Balancer provisioning states"}},
	}
}

func (d *loadBalancerDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllLoadBalancers(ctx, client)
}

func getAllLoadBalancers(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	targets, err := common.DiscoverViaResourceGraph(ctx, client,
		"Resources | where type =~ 'Microsoft.Network/loadBalancers' | project id, name, type, resourceGroup, location, tags, properties, sku, subscriptionId",
		toLoadBalancerTarget)
	if err != nil {
		log.Error().Err(err).Msg("failed to get Load Balancer results")
		return nil, err
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesLoadBalancer), nil
}

func toLoadBalancerTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.load-balancer.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{common.StringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{common.StringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{common.StringFromMap(items, "location")}

	if v := common.StringFromMap(sku, "name"); v != "" {
		attributes["azure.load-balancer.sku-name"] = []string{v}
	}
	if v := common.StringFromMap(sku, "tier"); v != "" {
		attributes["azure.load-balancer.sku-tier"] = []string{v}
	}
	if v := common.StringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.load-balancer.provisioning-state"] = []string{v}
	}

	publicExposed, frontendZones := analyzeFrontendIps(properties)
	attributes["azure.load-balancer.frontend.public-exposed"] = []string{strconv.FormatBool(publicExposed)}
	if len(frontendZones) > 0 {
		sort.Strings(frontendZones)
		attributes["azure.load-balancer.frontend.zones"] = frontendZones
	}

	attributes["azure.load-balancer.backend-pool-count"] = []string{strconv.Itoa(arrayLen(properties, "backendAddressPools"))}
	attributes["azure.load-balancer.load-balancing-rule-count"] = []string{strconv.Itoa(arrayLen(properties, "loadBalancingRules"))}
	attributes["azure.load-balancer.outbound-rule-count"] = []string{strconv.Itoa(arrayLen(properties, "outboundRules"))}
	attributes["azure.load-balancer.probe-count"] = []string{strconv.Itoa(arrayLen(properties, "probes"))}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.load-balancer.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDLoadBalancer,
		Label:      name,
		Attributes: attributes,
	}
}

// analyzeFrontendIps walks frontendIPConfigurations to determine if any of them have a publicIPAddress reference,
// and aggregates the unique availability zones declared on those frontend configs.
func analyzeFrontendIps(properties map[string]any) (publicExposed bool, zones []string) {
	v, ok := properties["frontendIPConfigurations"]
	if !ok {
		return false, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return false, nil
	}
	zoneSet := make(map[string]struct{})
	for _, e := range arr {
		fc, ok := e.(map[string]any)
		if !ok {
			continue
		}
		fcProps := common.GetMapValue(fc, "properties")
		publicIP := common.GetMapValue(fcProps, "publicIPAddress")
		if id, ok := publicIP["id"].(string); ok && id != "" {
			publicExposed = true
		}
		if zArr, ok := fc["zones"].([]any); ok {
			for _, z := range zArr {
				if s, ok := z.(string); ok && s != "" {
					zoneSet[s] = struct{}{}
				}
			}
		}
	}
	for z := range zoneSet {
		zones = append(zones, z)
	}
	return publicExposed, zones
}

func arrayLen(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	arr, ok := v.([]any)
	if !ok {
		return 0
	}
	return len(arr)
}

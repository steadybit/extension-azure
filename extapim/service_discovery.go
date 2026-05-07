/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extapim

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
	TargetIDApiManagement = "com.steadybit.extension_azure.apim.service"
	targetIcon            = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNCIgaGVpZ2h0PSIyNCIgdmlld0JveD0iMCAwIDI0IDI0IiBmaWxsPSJub25lIj48cGF0aCBkPSJNNCA0aDE2djE2SDRWNHptMiAyaDR2NEg2VjZ6bTYgMGg2djRoLTZWNnptLTYgNmg2djZINnYtNnptOCAwaDR2Mmg0djRoLTRoLTRoLTRWMTJ6IiBmaWxsPSJjdXJyZW50Q29sb3IiLz48L3N2Zz4="
)

type apimDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*apimDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*apimDiscovery)(nil)
)

func NewApiManagementDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&apimDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *apimDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDApiManagement,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *apimDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDApiManagement,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure API Management service", Other: "Azure API Management services"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.apim.sku-name"},
				{Attribute: "azure.apim.zones"},
				{Attribute: "azure.apim.public-network-access"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *apimDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.apim.service.name", Label: discovery_kit_api.PluralLabel{One: "API Management service name", Other: "API Management service names"}},
		{Attribute: "azure.apim.sku-name", Label: discovery_kit_api.PluralLabel{One: "API Management SKU name", Other: "API Management SKU names"}},
		{Attribute: "azure.apim.sku-capacity", Label: discovery_kit_api.PluralLabel{One: "API Management SKU capacity", Other: "API Management SKU capacities"}},
		{Attribute: "azure.apim.zones", Label: discovery_kit_api.PluralLabel{One: "API Management zone", Other: "API Management zones"}},
		{Attribute: "azure.apim.gateway-url", Label: discovery_kit_api.PluralLabel{One: "API Management gateway URL", Other: "API Management gateway URLs"}},
		{Attribute: "azure.apim.developer-portal-url", Label: discovery_kit_api.PluralLabel{One: "API Management developer portal URL", Other: "API Management developer portal URLs"}},
		{Attribute: "azure.apim.virtual-network-type", Label: discovery_kit_api.PluralLabel{One: "API Management VNet type", Other: "API Management VNet types"}},
		{Attribute: "azure.apim.public-network-access", Label: discovery_kit_api.PluralLabel{One: "API Management public network access", Other: "API Management public network access"}},
		{Attribute: "azure.apim.disable-gateway", Label: discovery_kit_api.PluralLabel{One: "API Management gateway disabled", Other: "API Management gateway disabled"}},
		{Attribute: "azure.apim.additional-locations", Label: discovery_kit_api.PluralLabel{One: "API Management additional location", Other: "API Management additional locations"}},
		{Attribute: "azure.apim.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "API Management provisioning state", Other: "API Management provisioning states"}},
	}
}

func (d *apimDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllApimServices(ctx, client)
}

func getAllApimServices(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.ApiManagement/service' | project id, name, type, resourceGroup, location, tags, properties, sku, zones, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get API Management results")
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
		targets = append(targets, toApimTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesApiManagement), nil
}

func toApimTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	sku := common.GetMapValue(items, "sku")

	id, _ := items["id"].(string)
	name, _ := items["name"].(string)

	attributes := make(map[string][]string)
	attributes["azure.apim.service.name"] = []string{name}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(sku, "name"); v != "" {
		attributes["azure.apim.sku-name"] = []string{v}
	}
	if v, ok := sku["capacity"].(float64); ok {
		attributes["azure.apim.sku-capacity"] = []string{strconv.Itoa(int(v))}
	}
	if zones := topLevelStringSlice(items, "zones"); len(zones) > 0 {
		sort.Strings(zones)
		attributes["azure.apim.zones"] = zones
	}
	if v := stringFromMap(properties, "gatewayUrl"); v != "" {
		attributes["azure.apim.gateway-url"] = []string{v}
	}
	if v := stringFromMap(properties, "developerPortalUrl"); v != "" {
		attributes["azure.apim.developer-portal-url"] = []string{v}
	}
	if v := stringFromMap(properties, "virtualNetworkType"); v != "" {
		attributes["azure.apim.virtual-network-type"] = []string{v}
	}
	if v := stringFromMap(properties, "publicNetworkAccess"); v != "" {
		attributes["azure.apim.public-network-access"] = []string{v}
	}
	if v, ok := properties["disableGateway"].(bool); ok {
		attributes["azure.apim.disable-gateway"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.apim.provisioning-state"] = []string{v}
	}

	if locs, ok := properties["additionalLocations"].([]any); ok && len(locs) > 0 {
		names := make([]string, 0, len(locs))
		for _, e := range locs {
			loc, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if n := stringFromMap(loc, "location"); n != "" {
				names = append(names, n)
			}
		}
		if len(names) > 0 {
			sort.Strings(names)
			attributes["azure.apim.additional-locations"] = names
		}
	}

	for k, v := range common.GetMapValue(items, "tags") {
		attributes[fmt.Sprintf("azure.apim.label.%s", strings.ToLower(k))] = []string{extutil.ToString(v)}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDApiManagement,
		Label:      name,
		Attributes: attributes,
	}
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

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

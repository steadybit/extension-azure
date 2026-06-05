/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package exteventgrid

import (
	"context"
	"fmt"
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

type subscriptionDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*subscriptionDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*subscriptionDiscovery)(nil)
)

func NewSubscriptionDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&subscriptionDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *subscriptionDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDSubscription,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *subscriptionDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDSubscription,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Event Grid event subscription", Other: "Azure Event Grid event subscriptions"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.eventgrid.subscription.endpoint-type"},
				{Attribute: "azure.eventgrid.subscription.dlq.configured"},
				{Attribute: "azure.eventgrid.subscription.provisioning-state"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *subscriptionDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.eventgrid.subscription.name", Label: discovery_kit_api.PluralLabel{One: "Event Grid subscription name", Other: "Event Grid subscription names"}},
		{Attribute: "azure.eventgrid.subscription.topic", Label: discovery_kit_api.PluralLabel{One: "Event Grid parent topic", Other: "Event Grid parent topics"}},
		{Attribute: "azure.eventgrid.subscription.endpoint-type", Label: discovery_kit_api.PluralLabel{One: "Event Grid subscription endpoint type", Other: "Event Grid subscription endpoint types"}},
		{Attribute: "azure.eventgrid.subscription.event-delivery-schema", Label: discovery_kit_api.PluralLabel{One: "Event Grid subscription delivery schema", Other: "Event Grid subscription delivery schemas"}},
		{Attribute: "azure.eventgrid.subscription.dlq.configured", Label: discovery_kit_api.PluralLabel{One: "Event Grid subscription DLQ configured", Other: "Event Grid subscription DLQ configured"}},
		{Attribute: "azure.eventgrid.subscription.retry-policy.max-delivery-attempts", Label: discovery_kit_api.PluralLabel{One: "Event Grid subscription retry max delivery attempts", Other: "Event Grid subscription retry max delivery attempts"}},
		{Attribute: "azure.eventgrid.subscription.retry-policy.event-time-to-live", Label: discovery_kit_api.PluralLabel{One: "Event Grid subscription retry event TTL (minutes)", Other: "Event Grid subscription retry event TTL (minutes)"}},
		{Attribute: "azure.eventgrid.subscription.provisioning-state", Label: discovery_kit_api.PluralLabel{One: "Event Grid subscription provisioning state", Other: "Event Grid subscription provisioning states"}},
	}
}

func (d *subscriptionDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllSubscriptions(ctx, client)
}

func getAllSubscriptions(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.EventGrid/eventSubscriptions' or type =~ 'Microsoft.EventGrid/topics/eventSubscriptions' or type =~ 'Microsoft.EventGrid/systemTopics/eventSubscriptions' | project id, name, type, resourceGroup, location, properties, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get Event Grid subscription results")
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
		targets = append(targets, toSubscriptionTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesEventGrid), nil
}

func toSubscriptionTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")
	destination := common.GetMapValue(properties, "destination")
	deadLetter := common.GetMapValue(properties, "deadLetterDestination")
	retryPolicy := common.GetMapValue(properties, "retryPolicy")

	id, _ := items["id"].(string)
	rawName, _ := items["name"].(string)
	// Resource Graph returns the child resource name as "<parent>/<sub>" — extract.
	parent := ""
	subName := rawName
	if i := strings.LastIndex(rawName, "/"); i >= 0 {
		parent = rawName[:i]
		subName = rawName[i+1:]
	}
	label := subName
	if parent != "" {
		label = fmt.Sprintf("%s/%s", parent, subName)
	}

	attributes := make(map[string][]string)
	attributes["azure.eventgrid.subscription.name"] = []string{subName}
	if parent != "" {
		attributes["azure.eventgrid.subscription.topic"] = []string{parent}
	}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(destination, "endpointType"); v != "" {
		attributes["azure.eventgrid.subscription.endpoint-type"] = []string{v}
	}
	if v := stringFromMap(properties, "eventDeliverySchema"); v != "" {
		attributes["azure.eventgrid.subscription.event-delivery-schema"] = []string{v}
	}
	if v := stringFromMap(properties, "provisioningState"); v != "" {
		attributes["azure.eventgrid.subscription.provisioning-state"] = []string{v}
	}

	dlqConfigured := false
	if endpointType := stringFromMap(deadLetter, "endpointType"); endpointType != "" {
		dlqConfigured = true
	}
	attributes["azure.eventgrid.subscription.dlq.configured"] = []string{strconv.FormatBool(dlqConfigured)}

	if v, ok := retryPolicy["maxDeliveryAttempts"].(float64); ok {
		attributes["azure.eventgrid.subscription.retry-policy.max-delivery-attempts"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := retryPolicy["eventTimeToLiveInMinutes"].(float64); ok {
		attributes["azure.eventgrid.subscription.retry-policy.event-time-to-live"] = []string{strconv.Itoa(int(v))}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDSubscription,
		Label:      label,
		Attributes: attributes,
	}
}

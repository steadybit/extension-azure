/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extservicebus

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

const TargetIDQueue = "com.steadybit.extension_azure.servicebus.queue"

type queueDiscovery struct{}

var (
	_ discovery_kit_sdk.TargetDescriber    = (*queueDiscovery)(nil)
	_ discovery_kit_sdk.AttributeDescriber = (*queueDiscovery)(nil)
)

func NewQueueDiscovery() discovery_kit_sdk.TargetDiscovery {
	return discovery_kit_sdk.NewCachedTargetDiscovery(&queueDiscovery{},
		discovery_kit_sdk.WithRefreshTargetsNow(),
		discovery_kit_sdk.WithRefreshTargetsInterval(context.Background(), 60*time.Second),
	)
}

func (d *queueDiscovery) Describe() discovery_kit_api.DiscoveryDescription {
	return discovery_kit_api.DiscoveryDescription{
		Id:       TargetIDQueue,
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: extutil.Ptr("60s")},
	}
}

func (d *queueDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDQueue,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     extutil.Ptr(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Service Bus queue", Other: "Azure Service Bus queues"},
		Category: extutil.Ptr("cloud"),
		Table: discovery_kit_api.Table{
			Columns: []discovery_kit_api.Column{
				{Attribute: "steadybit.label"},
				{Attribute: "azure.servicebus.namespace.name"},
				{Attribute: "azure.servicebus.queue.status"},
				{Attribute: "azure.servicebus.queue.dead-lettering-on-message-expiration"},
				{Attribute: "azure.location"},
			},
			OrderBy: []discovery_kit_api.OrderBy{{Attribute: "steadybit.label", Direction: "ASC"}},
		},
	}
}

func (d *queueDiscovery) DescribeAttributes() []discovery_kit_api.AttributeDescription {
	return []discovery_kit_api.AttributeDescription{
		{Attribute: "azure.servicebus.queue.name", Label: discovery_kit_api.PluralLabel{One: "Service Bus queue name", Other: "Service Bus queue names"}},
		{Attribute: "azure.servicebus.namespace.name", Label: discovery_kit_api.PluralLabel{One: "Service Bus namespace name", Other: "Service Bus namespace names"}},
		{Attribute: "azure.servicebus.queue.status", Label: discovery_kit_api.PluralLabel{One: "Service Bus queue status", Other: "Service Bus queue statuses"}},
		{Attribute: "azure.servicebus.queue.max-delivery-count", Label: discovery_kit_api.PluralLabel{One: "Service Bus queue max delivery count", Other: "Service Bus queue max delivery counts"}},
		{Attribute: "azure.servicebus.queue.dead-lettering-on-message-expiration", Label: discovery_kit_api.PluralLabel{One: "Service Bus queue DLQ on expiration", Other: "Service Bus queue DLQ on expiration"}},
		{Attribute: "azure.servicebus.queue.requires-duplicate-detection", Label: discovery_kit_api.PluralLabel{One: "Service Bus queue requires deduplication", Other: "Service Bus queue requires deduplication"}},
		{Attribute: "azure.servicebus.queue.requires-session", Label: discovery_kit_api.PluralLabel{One: "Service Bus queue requires session", Other: "Service Bus queue requires session"}},
		{Attribute: "azure.servicebus.queue.lock-duration", Label: discovery_kit_api.PluralLabel{One: "Service Bus queue lock duration", Other: "Service Bus queue lock durations"}},
		{Attribute: "azure.servicebus.queue.default-message-time-to-live", Label: discovery_kit_api.PluralLabel{One: "Service Bus queue default message TTL", Other: "Service Bus queue default message TTLs"}},
		{Attribute: "azure.servicebus.queue.forward-to", Label: discovery_kit_api.PluralLabel{One: "Service Bus queue forward target", Other: "Service Bus queue forward targets"}},
		{Attribute: "azure.servicebus.queue.forward-dead-lettered-messages-to", Label: discovery_kit_api.PluralLabel{One: "Service Bus queue forward DLQ target", Other: "Service Bus queue forward DLQ targets"}},
	}
}

func (d *queueDiscovery) DiscoverTargets(ctx context.Context) ([]discovery_kit_api.Target, error) {
	client, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get client: %w", err)
	}
	return getAllQueues(ctx, client)
}

func getAllQueues(ctx context.Context, client common.ArmResourceGraphApi) ([]discovery_kit_api.Target, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: extutil.Ptr("Resources | where type =~ 'Microsoft.ServiceBus/namespaces/queues' | project id, name, type, resourceGroup, location, properties, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to get Service Bus queue results")
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
		targets = append(targets, toQueueTarget(items))
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesServiceBus), nil
}

func toQueueTarget(items map[string]any) discovery_kit_api.Target {
	properties := common.GetMapValue(items, "properties")

	id, _ := items["id"].(string)
	rawName, _ := items["name"].(string)
	namespace := ""
	queueName := rawName
	if i := strings.LastIndex(rawName, "/"); i >= 0 {
		namespace = rawName[:i]
		queueName = rawName[i+1:]
	}
	label := queueName
	if namespace != "" {
		label = fmt.Sprintf("%s/%s", namespace, queueName)
	}

	attributes := make(map[string][]string)
	attributes["azure.servicebus.queue.name"] = []string{queueName}
	if namespace != "" {
		attributes["azure.servicebus.namespace.name"] = []string{namespace}
	}
	attributes["azure.subscription.id"] = []string{stringFromMap(items, "subscriptionId")}
	attributes["azure.resource-group.name"] = []string{stringFromMap(items, "resourceGroup")}
	attributes["azure.location"] = []string{stringFromMap(items, "location")}

	if v := stringFromMap(properties, "status"); v != "" {
		attributes["azure.servicebus.queue.status"] = []string{v}
	}
	if v, ok := properties["maxDeliveryCount"].(float64); ok {
		attributes["azure.servicebus.queue.max-delivery-count"] = []string{strconv.Itoa(int(v))}
	}
	if v, ok := properties["deadLetteringOnMessageExpiration"].(bool); ok {
		attributes["azure.servicebus.queue.dead-lettering-on-message-expiration"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["requiresDuplicateDetection"].(bool); ok {
		attributes["azure.servicebus.queue.requires-duplicate-detection"] = []string{strconv.FormatBool(v)}
	}
	if v, ok := properties["requiresSession"].(bool); ok {
		attributes["azure.servicebus.queue.requires-session"] = []string{strconv.FormatBool(v)}
	}
	if v := stringFromMap(properties, "lockDuration"); v != "" {
		attributes["azure.servicebus.queue.lock-duration"] = []string{v}
	}
	if v := stringFromMap(properties, "defaultMessageTimeToLive"); v != "" {
		attributes["azure.servicebus.queue.default-message-time-to-live"] = []string{v}
	}
	if v := stringFromMap(properties, "forwardTo"); v != "" {
		attributes["azure.servicebus.queue.forward-to"] = []string{v}
	}
	if v := stringFromMap(properties, "forwardDeadLetteredMessagesTo"); v != "" {
		attributes["azure.servicebus.queue.forward-dead-lettered-messages-to"] = []string{v}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDQueue,
		Label:      label,
		Attributes: attributes,
	}
}

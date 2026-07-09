/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extservicebus

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/servicebus/armservicebus"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	"github.com/steadybit/discovery-kit/go/discovery_kit_commons"
	"github.com/steadybit/discovery-kit/go/discovery_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	"github.com/steadybit/extension-azure/config"
	"github.com/steadybit/extension-kit/extbuild"
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
		Discover: discovery_kit_api.DescribingEndpointReferenceWithCallInterval{CallInterval: new("60s")},
	}
}

func (d *queueDiscovery) DescribeTarget() discovery_kit_api.TargetDescription {
	return discovery_kit_api.TargetDescription{
		Id:       TargetIDQueue,
		Version:  extbuild.GetSemverVersionStringOrUnknown(),
		Icon:     new(targetIcon),
		Label:    discovery_kit_api.PluralLabel{One: "Azure Service Bus queue", Other: "Azure Service Bus queues"},
		Category: new("cloud"),
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
	rgClient, err := common.GetClientByCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to get Resource Graph client: %w", err)
	}
	return getAllQueues(ctx, rgClient, sdkQueueLister(common.GetServiceBusQueuesClient))
}

// queueLister returns the queues in (subscription, resourceGroup, namespace). Indirection over the
// SDK pager so we can swap a fake implementation in tests.
type queueLister func(ctx context.Context, subscriptionId, resourceGroup, namespace string) ([]*armservicebus.SBQueue, error)

// sdkQueueLister adapts the SDK pager into a flat queueLister. Production use only.
func sdkQueueLister(provider func(subscriptionId string) (*armservicebus.QueuesClient, error)) queueLister {
	return func(ctx context.Context, subscriptionId, resourceGroup, namespace string) ([]*armservicebus.SBQueue, error) {
		client, err := provider(subscriptionId)
		if err != nil {
			return nil, err
		}
		pager := client.NewListByNamespacePager(resourceGroup, namespace, nil)
		var queues []*armservicebus.SBQueue
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return queues, err
			}
			queues = append(queues, page.Value...)
		}
		return queues, nil
	}
}

// serviceBusNamespaceRef carries the addressing fields the queue/topic discovery need to issue
// per-namespace ARM List calls. Built from a low-cardinality Resource Graph query.
type serviceBusNamespaceRef struct {
	name           string
	resourceGroup  string
	subscriptionId string
	location       string
}

func listServiceBusNamespaceRefs(ctx context.Context, rgClient common.ArmResourceGraphApi) ([]serviceBusNamespaceRef, error) {
	subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
	var subscriptions []*string
	if subscriptionId != "" {
		subscriptions = []*string{&subscriptionId}
	}
	results, err := rgClient.Resources(ctx, armresourcegraph.QueryRequest{
		Query: new("Resources | where type =~ 'Microsoft.ServiceBus/namespaces' | project name, resourceGroup, location, subscriptionId"),
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
		},
		Subscriptions: subscriptions,
	}, nil)
	if err != nil {
		log.Error().Err(err).Msgf("failed to list Service Bus namespaces from Resource Graph")
		return nil, err
	}

	refs := make([]serviceBusNamespaceRef, 0)
	rows, ok := results.Data.([]any)
	if !ok {
		return refs, nil
	}
	for _, r := range rows {
		items, ok := r.(map[string]any)
		if !ok {
			continue
		}
		refs = append(refs, serviceBusNamespaceRef{
			name:           common.StringFromMap(items, "name"),
			resourceGroup:  common.StringFromMap(items, "resourceGroup"),
			subscriptionId: common.StringFromMap(items, "subscriptionId"),
			location:       common.StringFromMap(items, "location"),
		})
	}
	return refs, nil
}

// getAllQueues lists Service Bus queues via direct ARM. Resource Graph indexes Service Bus child
// resources with a multi-minute lag, which makes ad-hoc testing painful; the direct ARM path is
// real-time. Namespaces themselves are still enumerated via Resource Graph because their cardinality
// is low and the lag matters less at the namespace level.
func getAllQueues(ctx context.Context, rgClient common.ArmResourceGraphApi, lister queueLister) ([]discovery_kit_api.Target, error) {
	namespaces, err := listServiceBusNamespaceRefs(ctx, rgClient)
	if err != nil {
		return nil, err
	}

	targets := make([]discovery_kit_api.Target, 0)
	for _, ns := range namespaces {
		queues, err := lister(ctx, ns.subscriptionId, ns.resourceGroup, ns.name)
		if err != nil {
			log.Warn().Err(err).Msgf("failed to list queues for namespace %s/%s; skipping", ns.subscriptionId, ns.name)
			continue
		}
		for _, q := range queues {
			if q == nil {
				continue
			}
			targets = append(targets, queueToTarget(q, ns))
		}
	}
	return discovery_kit_commons.ApplyAttributeExcludes(targets, config.Config.DiscoveryAttributesExcludesServiceBus), nil
}

func queueToTarget(q *armservicebus.SBQueue, ns serviceBusNamespaceRef) discovery_kit_api.Target {
	queueName := ""
	if q.Name != nil {
		queueName = *q.Name
	}
	id := ""
	if q.ID != nil {
		id = *q.ID
	}

	attributes := map[string][]string{
		"azure.servicebus.queue.name":     {queueName},
		"azure.servicebus.namespace.name": {ns.name},
		"azure.subscription.id":           {ns.subscriptionId},
		"azure.resource-group.name":       {ns.resourceGroup},
		"azure.location":                  {ns.location},
	}

	if p := q.Properties; p != nil {
		if p.Status != nil {
			attributes["azure.servicebus.queue.status"] = []string{string(*p.Status)}
		}
		if p.MaxDeliveryCount != nil {
			attributes["azure.servicebus.queue.max-delivery-count"] = []string{strconv.Itoa(int(*p.MaxDeliveryCount))}
		}
		if p.DeadLetteringOnMessageExpiration != nil {
			attributes["azure.servicebus.queue.dead-lettering-on-message-expiration"] = []string{strconv.FormatBool(*p.DeadLetteringOnMessageExpiration)}
		}
		if p.RequiresDuplicateDetection != nil {
			attributes["azure.servicebus.queue.requires-duplicate-detection"] = []string{strconv.FormatBool(*p.RequiresDuplicateDetection)}
		}
		if p.RequiresSession != nil {
			attributes["azure.servicebus.queue.requires-session"] = []string{strconv.FormatBool(*p.RequiresSession)}
		}
		if p.LockDuration != nil {
			attributes["azure.servicebus.queue.lock-duration"] = []string{*p.LockDuration}
		}
		if p.DefaultMessageTimeToLive != nil {
			attributes["azure.servicebus.queue.default-message-time-to-live"] = []string{*p.DefaultMessageTimeToLive}
		}
		if p.ForwardTo != nil && *p.ForwardTo != "" {
			attributes["azure.servicebus.queue.forward-to"] = []string{*p.ForwardTo}
		}
		if p.ForwardDeadLetteredMessagesTo != nil && *p.ForwardDeadLetteredMessagesTo != "" {
			attributes["azure.servicebus.queue.forward-dead-lettered-messages-to"] = []string{*p.ForwardDeadLetteredMessagesTo}
		}
	}

	return discovery_kit_api.Target{
		Id:         id,
		TargetType: TargetIDQueue,
		Label:      fmt.Sprintf("%s/%s", ns.name, queueName),
		Attributes: attributes,
	}
}

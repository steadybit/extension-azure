/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extservicebus

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/servicebus/armservicebus"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	"github.com/steadybit/extension-azure/common"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	QueueDisableActionId = "com.steadybit.extension_azure.servicebus.queue.disable"
	TopicDisableActionId = "com.steadybit.extension_azure.servicebus.topic.disable"
)

// EntityDisableState captures the original entity status so we can restore it on stop.
// Used by both the queue-disable and topic-disable attacks.
type EntityDisableState struct {
	SubscriptionId    string
	ResourceGroupName string
	NamespaceName     string
	EntityName        string
	OriginalStatus    string
}

type queuesApi interface {
	Get(ctx context.Context, resourceGroupName string, namespaceName string, queueName string, options *armservicebus.QueuesClientGetOptions) (armservicebus.QueuesClientGetResponse, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName string, namespaceName string, queueName string, parameters armservicebus.SBQueue, options *armservicebus.QueuesClientCreateOrUpdateOptions) (armservicebus.QueuesClientCreateOrUpdateResponse, error)
}

type topicsApi interface {
	Get(ctx context.Context, resourceGroupName string, namespaceName string, topicName string, options *armservicebus.TopicsClientGetOptions) (armservicebus.TopicsClientGetResponse, error)
	CreateOrUpdate(ctx context.Context, resourceGroupName string, namespaceName string, topicName string, parameters armservicebus.SBTopic, options *armservicebus.TopicsClientCreateOrUpdateOptions) (armservicebus.TopicsClientCreateOrUpdateResponse, error)
}

// ---- Queue disable ----

type queueDisableAttack struct {
	clientProvider func(subscriptionId string) (queuesApi, error)
}

var _ action_kit_sdk.Action[EntityDisableState] = (*queueDisableAttack)(nil)
var _ action_kit_sdk.ActionWithStop[EntityDisableState] = (*queueDisableAttack)(nil)

func NewQueueDisableAction() action_kit_sdk.ActionWithStop[EntityDisableState] {
	return &queueDisableAttack{
		clientProvider: func(subscriptionId string) (queuesApi, error) {
			cred, err := common.ConnectionAzure()
			if err != nil {
				return nil, err
			}
			factory, err := armservicebus.NewClientFactory(subscriptionId, cred, nil)
			if err != nil {
				return nil, err
			}
			return factory.NewQueuesClient(), nil
		},
	}
}

func (a *queueDisableAttack) NewEmptyState() EntityDisableState { return EntityDisableState{} }

func (a *queueDisableAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    QueueDisableActionId,
		Label: "Disable Service Bus Queue",
		Description: "Sets the queue's status to 'Disabled' to drop both sends and receives. The original status is restored on stop. " +
			"Validates how producers and consumers handle the queue being unavailable.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDQueue,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by namespace and queue name",
					Description: extutil.Ptr("Find Service Bus queue by namespace and queue name"),
					Query:       "azure.servicebus.namespace.name=\"\" and azure.servicebus.queue.name=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("Azure"),
		Category:    extutil.Ptr("Service Bus"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long the queue stays disabled. Status is restored on stop."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("60s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *queueDisableAttack) Prepare(ctx context.Context, state *EntityDisableState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.SubscriptionId = mustHave(request.Target.Attributes, "azure.subscription.id")
	state.ResourceGroupName = mustHave(request.Target.Attributes, "azure.resource-group.name")
	state.NamespaceName = mustHave(request.Target.Attributes, "azure.servicebus.namespace.name")
	state.EntityName = mustHave(request.Target.Attributes, "azure.servicebus.queue.name")
	if state.SubscriptionId == "" || state.ResourceGroupName == "" || state.NamespaceName == "" || state.EntityName == "" {
		return nil, extension_kit.ToError("Target is missing one of: azure.subscription.id, azure.resource-group.name, azure.servicebus.namespace.name, azure.servicebus.queue.name", nil)
	}
	client, err := a.clientProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Service Bus queues client for subscription %s", state.SubscriptionId), err)
	}
	got, err := client.Get(ctx, state.ResourceGroupName, state.NamespaceName, state.EntityName, nil)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe Service Bus queue %s/%s", state.NamespaceName, state.EntityName), err)
	}
	if got.SBQueue.Properties != nil && got.SBQueue.Properties.Status != nil {
		state.OriginalStatus = string(*got.SBQueue.Properties.Status)
	} else {
		state.OriginalStatus = string(armservicebus.EntityStatusActive)
	}
	return nil, nil
}

func (a *queueDisableAttack) Start(ctx context.Context, state *EntityDisableState) (*action_kit_api.StartResult, error) {
	if err := setQueueStatus(ctx, a.clientProvider, state, armservicebus.EntityStatusDisabled); err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to disable Service Bus queue %s/%s", state.NamespaceName, state.EntityName), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Disabled Service Bus queue %s/%s (was %s)", state.NamespaceName, state.EntityName, state.OriginalStatus),
		}}),
	}, nil
}

func (a *queueDisableAttack) Stop(ctx context.Context, state *EntityDisableState) (*action_kit_api.StopResult, error) {
	if err := setQueueStatus(ctx, a.clientProvider, state, armservicebus.EntityStatus(state.OriginalStatus)); err != nil {
		log.Error().Err(err).Msgf("Failed to restore Service Bus queue %s/%s status", state.NamespaceName, state.EntityName)
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to restore Service Bus queue %s/%s status to %s", state.NamespaceName, state.EntityName, state.OriginalStatus), err)
	}
	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Restored Service Bus queue %s/%s to status %s", state.NamespaceName, state.EntityName, state.OriginalStatus),
		}}),
	}, nil
}

func setQueueStatus(ctx context.Context, provider func(subscriptionId string) (queuesApi, error), state *EntityDisableState, status armservicebus.EntityStatus) error {
	client, err := provider(state.SubscriptionId)
	if err != nil {
		return err
	}
	got, err := client.Get(ctx, state.ResourceGroupName, state.NamespaceName, state.EntityName, nil)
	if err != nil {
		return err
	}
	if got.SBQueue.Properties == nil {
		got.SBQueue.Properties = &armservicebus.SBQueueProperties{}
	}
	got.SBQueue.Properties.Status = extutil.Ptr(status)
	_, err = client.CreateOrUpdate(ctx, state.ResourceGroupName, state.NamespaceName, state.EntityName, got.SBQueue, nil)
	return err
}

// ---- Topic disable ----

type topicDisableAttack struct {
	clientProvider func(subscriptionId string) (topicsApi, error)
}

var _ action_kit_sdk.Action[EntityDisableState] = (*topicDisableAttack)(nil)
var _ action_kit_sdk.ActionWithStop[EntityDisableState] = (*topicDisableAttack)(nil)

func NewTopicDisableAction() action_kit_sdk.ActionWithStop[EntityDisableState] {
	return &topicDisableAttack{
		clientProvider: func(subscriptionId string) (topicsApi, error) {
			cred, err := common.ConnectionAzure()
			if err != nil {
				return nil, err
			}
			factory, err := armservicebus.NewClientFactory(subscriptionId, cred, nil)
			if err != nil {
				return nil, err
			}
			return factory.NewTopicsClient(), nil
		},
	}
}

func (a *topicDisableAttack) NewEmptyState() EntityDisableState { return EntityDisableState{} }

func (a *topicDisableAttack) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:    TopicDisableActionId,
		Label: "Disable Service Bus Topic",
		Description: "Sets the topic's status to 'Disabled' to drop both publishes and subscriber receives. The original status is restored on stop. " +
			"Validates how publishers and all subscribers handle the topic being unavailable.",
		Version: extbuild.GetSemverVersionStringOrUnknown(),
		Icon:    extutil.Ptr(targetIcon),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType: TargetIDTopic,
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "by namespace and topic name",
					Description: extutil.Ptr("Find Service Bus topic by namespace and topic name"),
					Query:       "azure.servicebus.namespace.name=\"\" and azure.servicebus.topic.name=\"\"",
				},
			}),
		}),
		Technology:  extutil.Ptr("Azure"),
		Category:    extutil.Ptr("Service Bus"),
		TimeControl: action_kit_api.TimeControlExternal,
		Kind:        action_kit_api.Attack,
		Parameters: []action_kit_api.ActionParameter{
			{
				Name:         "duration",
				Label:        "Duration",
				Description:  extutil.Ptr("How long the topic stays disabled. Status is restored on stop."),
				Type:         action_kit_api.ActionParameterTypeDuration,
				DefaultValue: extutil.Ptr("60s"),
				Order:        extutil.Ptr(1),
				Required:     extutil.Ptr(true),
			},
		},
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (a *topicDisableAttack) Prepare(ctx context.Context, state *EntityDisableState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	state.SubscriptionId = mustHave(request.Target.Attributes, "azure.subscription.id")
	state.ResourceGroupName = mustHave(request.Target.Attributes, "azure.resource-group.name")
	state.NamespaceName = mustHave(request.Target.Attributes, "azure.servicebus.namespace.name")
	state.EntityName = mustHave(request.Target.Attributes, "azure.servicebus.topic.name")
	if state.SubscriptionId == "" || state.ResourceGroupName == "" || state.NamespaceName == "" || state.EntityName == "" {
		return nil, extension_kit.ToError("Target is missing one of: azure.subscription.id, azure.resource-group.name, azure.servicebus.namespace.name, azure.servicebus.topic.name", nil)
	}
	client, err := a.clientProvider(state.SubscriptionId)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to initialize Service Bus topics client for subscription %s", state.SubscriptionId), err)
	}
	got, err := client.Get(ctx, state.ResourceGroupName, state.NamespaceName, state.EntityName, nil)
	if err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to describe Service Bus topic %s/%s", state.NamespaceName, state.EntityName), err)
	}
	if got.SBTopic.Properties != nil && got.SBTopic.Properties.Status != nil {
		state.OriginalStatus = string(*got.SBTopic.Properties.Status)
	} else {
		state.OriginalStatus = string(armservicebus.EntityStatusActive)
	}
	return nil, nil
}

func (a *topicDisableAttack) Start(ctx context.Context, state *EntityDisableState) (*action_kit_api.StartResult, error) {
	if err := setTopicStatus(ctx, a.clientProvider, state, armservicebus.EntityStatusDisabled); err != nil {
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to disable Service Bus topic %s/%s", state.NamespaceName, state.EntityName), err)
	}
	return &action_kit_api.StartResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Disabled Service Bus topic %s/%s (was %s)", state.NamespaceName, state.EntityName, state.OriginalStatus),
		}}),
	}, nil
}

func (a *topicDisableAttack) Stop(ctx context.Context, state *EntityDisableState) (*action_kit_api.StopResult, error) {
	if err := setTopicStatus(ctx, a.clientProvider, state, armservicebus.EntityStatus(state.OriginalStatus)); err != nil {
		log.Error().Err(err).Msgf("Failed to restore Service Bus topic %s/%s status", state.NamespaceName, state.EntityName)
		return nil, extension_kit.ToError(fmt.Sprintf("Failed to restore Service Bus topic %s/%s status to %s", state.NamespaceName, state.EntityName, state.OriginalStatus), err)
	}
	return &action_kit_api.StopResult{
		Messages: extutil.Ptr([]action_kit_api.Message{{
			Level:   extutil.Ptr(action_kit_api.Info),
			Message: fmt.Sprintf("Restored Service Bus topic %s/%s to status %s", state.NamespaceName, state.EntityName, state.OriginalStatus),
		}}),
	}, nil
}

func setTopicStatus(ctx context.Context, provider func(subscriptionId string) (topicsApi, error), state *EntityDisableState, status armservicebus.EntityStatus) error {
	client, err := provider(state.SubscriptionId)
	if err != nil {
		return err
	}
	got, err := client.Get(ctx, state.ResourceGroupName, state.NamespaceName, state.EntityName, nil)
	if err != nil {
		return err
	}
	if got.SBTopic.Properties == nil {
		got.SBTopic.Properties = &armservicebus.SBTopicProperties{}
	}
	got.SBTopic.Properties.Status = extutil.Ptr(status)
	_, err = client.CreateOrUpdate(ctx, state.ResourceGroupName, state.NamespaceName, state.EntityName, got.SBTopic, nil)
	return err
}

func mustHave(attrs map[string][]string, key string) string {
	v, ok := attrs[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}

/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extservicebus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueueDescribe(t *testing.T) {
	assert.Equal(t, TargetIDQueue, (&queueDiscovery{}).Describe().Id)
}

func TestQueueDescribeTarget(t *testing.T) {
	td := (&queueDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDQueue, td.Id)
	assert.Contains(t, td.Label.One, "Service Bus queue")
}

func TestQueueDescribeAttributes(t *testing.T) {
	require.NotEmpty(t, (&queueDiscovery{}).DescribeAttributes())
}

func TestNewQueueDiscovery(t *testing.T) {
	require.NotNil(t, NewQueueDiscovery())
}

func TestTopicDescribe(t *testing.T) {
	assert.Equal(t, TargetIDTopic, (&topicDiscovery{}).Describe().Id)
}

func TestTopicDescribeTarget(t *testing.T) {
	td := (&topicDiscovery{}).DescribeTarget()
	assert.Equal(t, TargetIDTopic, td.Id)
	assert.Contains(t, td.Label.One, "Service Bus topic")
}

func TestTopicDescribeAttributes(t *testing.T) {
	require.NotEmpty(t, (&topicDiscovery{}).DescribeAttributes())
}

func TestNewTopicDiscovery(t *testing.T) {
	require.NotNil(t, NewTopicDiscovery())
}

func TestNewQueueDisableAction(t *testing.T) {
	require.NotNil(t, NewQueueDisableAction())
}

func TestNewTopicDisableAction(t *testing.T) {
	require.NotNil(t, NewTopicDisableAction())
}

func TestQueueDisableDescribe(t *testing.T) {
	a := &queueDisableAttack{}
	desc := a.Describe()
	assert.Equal(t, QueueDisableActionId, desc.Id)
	assert.Equal(t, TargetIDQueue, desc.TargetSelection.TargetType)
}

func TestTopicDisableDescribe(t *testing.T) {
	a := &topicDisableAttack{}
	desc := a.Describe()
	assert.Equal(t, TopicDisableActionId, desc.Id)
	assert.Equal(t, TargetIDTopic, desc.TargetSelection.TargetType)
}

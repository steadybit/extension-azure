/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extaks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNodePoolTerminateInstancesAction(t *testing.T) {
	require.NotNil(t, NewNodePoolTerminateInstancesAction())
}

func TestNodePoolTerminateDescribe(t *testing.T) {
	a := &nodePoolTerminateInstancesAttack{}
	desc := a.Describe()
	assert.Equal(t, NodePoolTerminateInstancesActionId, desc.Id)
	assert.Equal(t, TargetIDNodePool, desc.TargetSelection.TargetType)
	assert.NotEmpty(t, desc.Parameters)
}

func TestClusterDescribeEnrichmentRules(t *testing.T) {
	rules := (&clusterDiscovery{}).DescribeEnrichmentRules()
	require.NotEmpty(t, rules)
	for _, r := range rules {
		assert.NotEmpty(t, r.Id)
	}
}

/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extcosmosdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCosmosDbFailoverAction(t *testing.T) {
	require.NotNil(t, NewCosmosDbFailoverAction())
}

func TestCosmosFailoverDescribe(t *testing.T) {
	a := &cosmosFailoverAttack{}
	desc := a.Describe()
	assert.Equal(t, CosmosDbFailoverActionId, desc.Id)
	assert.Equal(t, TargetIDCosmosDbAccount, desc.TargetSelection.TargetType)
}

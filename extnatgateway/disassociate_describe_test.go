/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package extnatgateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNatGatewayDisassociateAction(t *testing.T) {
	require.NotNil(t, NewNatGatewayDisassociateAction())
}

func TestNatGatewayDisassociateDescribe(t *testing.T) {
	a := &natGatewayDisassociateAttack{}
	desc := a.Describe()
	assert.Equal(t, NatGatewayDisassociateActionId, desc.Id)
	assert.Equal(t, TargetIDNatGateway, desc.TargetSelection.TargetType)
	assert.NotEmpty(t, desc.Parameters)
}

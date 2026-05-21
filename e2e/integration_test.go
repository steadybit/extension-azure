// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_test/e2e"
	actValidate "github.com/steadybit/action-kit/go/action_kit_test/validate"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	disValidate "github.com/steadybit/discovery-kit/go/discovery_kit_test/validate"
	"github.com/steadybit/extension-azure/extvm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithMinikube(t *testing.T) {
	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-azure",
		Port: 8092,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", "logging.level=debug",
				"--set", "azure.level=debug",
				// Existing discoveries.
				"--set", "discovery.enable.vm=true",
				"--set", "discovery.enable.scaleSetInstance=true",
				"--set", "discovery.enable.containerApp=true",
				"--set", "discovery.enable.networkSecurityGroup=true",
				"--set", "discovery.enable.azureFunction=true",
				// New discoveries added in feat/expand-azure-targets-and-attacks. Enabling them in the
				// e2e startup smoke-test catches accidental panics or missing registrations.
				"--set", "discovery.enable.aksCluster=true",
				"--set", "discovery.enable.aksNodePool=true",
				"--set", "discovery.enable.scaleSet=true",
				"--set", "discovery.enable.managedDisk=true",
				"--set", "discovery.enable.natGateway=true",
				"--set", "discovery.enable.cosmosDb=true",
				"--set", "discovery.enable.eventGrid=true",
				"--set", "discovery.enable.serviceBus=true",
				"--set", "discovery.enable.serviceBusQueue=true",
				"--set", "discovery.enable.serviceBusTopic=true",
				"--set", "discovery.enable.storageQueue=true",
				"--set", "discovery.enable.loadBalancer=true",
				"--set", "discovery.enable.applicationGateway=true",
				"--set", "discovery.enable.apiManagement=true",
			}
		},
	}

	e2e.WithMinikube(t, e2e.DefaultMinikubeOpts(), &extFactory, []e2e.WithMinikubeTestCase{
		{
			Name: "validate discovery",
			Test: validateDiscovery,
		},
		{
			Name: "validate action",
			Test: validateAction,
		},
	})
}

// test the installation of the extension in minikube
func validateDiscovery(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	validationErr := disValidate.ValidateEndpointReferences("/", e.Client)
	if uw, ok := validationErr.(interface{ Unwrap() []error }); ok {
		errs := uw.Unwrap()
		// Every enabled discovery hits Azure (or Resource Graph) and fails because the Minikube test
		// environment has no Azure backend. We expect one error per enabled discovery: 5 pre-existing
		// + 14 added in feat/expand-azure-targets-and-attacks = 19 total.
		const expectedDiscoveryCount = 19
		assert.Len(t, errs, expectedDiscoveryCount)
		for i, err := range errs {
			assert.Contains(t, err.Error(), "GET /com.steadybit.extension_azure",
				"error %d should be a discovery endpoint failure", i)
		}
	} else {
		assert.NoError(t, validationErr)
	}

	log.Info().Msg("Starting testDiscovery")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := e2e.PollForTarget(ctx, e, extvm.TargetIDVM, func(target discovery_kit_api.Target) bool {
		return e2e.HasAttribute(target, "azure-vm.name", "test")
	})
	// we do not have a real azure vm running, so we expect an error
	require.Error(t, err)
}

func validateAction(t *testing.T, _ *e2e.Minikube, e *e2e.Extension) {
	assert.NoError(t, actValidate.ValidateEndpointReferences("/", e.Client))
}

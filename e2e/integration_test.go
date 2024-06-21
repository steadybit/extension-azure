// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_test/e2e"
	actValidate "github.com/steadybit/action-kit/go/action_kit_test/validate"
	"github.com/steadybit/discovery-kit/go/discovery_kit_api"
	disValidate "github.com/steadybit/discovery-kit/go/discovery_kit_test/validate"
	"github.com/steadybit/extension-azure/extvm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWithMinikube(t *testing.T) {
	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-azure",
		Port: 8092,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", "logging.level=debug",
				"--set", "azure.level=debug",
			}
		},
	}

	e2e.WithMinikube(t, e2e.DefaultMinikubeOpts(), &extFactory, []e2e.WithMinikubeTestCase{
		{
			Name: "validate discovery",
			Test: validateDiscovery,
		},
		{
			Name: "valudate action",
			Test: validateAction,
		},
	})
}

// test the installation of the extension in minikube
func validateDiscovery(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	validationErr := disValidate.ValidateEndpointReferences("/", e.Client)
	if uw, ok := validationErr.(interface{ Unwrap() []error }); ok {
		errs := uw.Unwrap()
		// we expect two errors, because we do not have a real azure vm running
		assert.Len(t, errs, 2)
		assert.Contains(t, errs[0].Error(), "GET /com.steadybit.extension_azure") // failed to get all virtual machines
		assert.Contains(t, errs[1].Error(), "GET /com.steadybit.extension_azure") // failed to get all scale sets
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

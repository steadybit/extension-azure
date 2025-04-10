/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package config

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// Specification is the configuration specification for the extension. Configuration values can be applied
// through environment variables. Learn more through the documentation of the envconfig package.
// https://github.com/kelseyhightower/envconfig
type Specification struct {
	AzureCertificatePath                        string   `json:"azureCertificatePath" required:"false" split_words:"true"`
	AzureCertificatePassword                    string   `json:"azureCertificatePassword" required:"false" split_words:"true"`
	AzureUserAssertionString                    string   `json:"azureUserAssertionString" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesScaleSetInstance []string `json:"discoveryAttributesExcludesScaleSetInstance" required:"false" split_words:"true"`
	DiscoveryAttributesExcludesVM               []string `json:"discoveryAttributesExcludesVM" required:"false" split_words:"true"`
	EnrichScaleSetVMDataForTargetTypes          []string `json:"EnrichScaleSetVMDataForTargetTypes" split_words:"true" default:"com.steadybit.extension_jvm.jvm-instance,com.steadybit.extension_container.container,com.steadybit.extension_kubernetes.kubernetes-deployment,com.steadybit.extension_kubernetes.kubernetes-pod,com.steadybit.extension_kubernetes.kubernetes-daemonset,com.steadybit.extension_kubernetes.kubernetes-statefulset,com.steadybit.extension_http.client-location,com.steadybit.extension_jmeter.location,com.steadybit.extension_k6.location,com.steadybit.extension_gatling.location"`
}

var (
	Config Specification
)

func ParseConfiguration() {
	err := envconfig.Process("steadybit_extension", &Config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse configuration from environment.")
	}
}

func ValidateConfiguration() {
	// You may optionally validate the configuration here.
}

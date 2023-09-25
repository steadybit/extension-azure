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
	AzureCertificatePath                       string   `json:"azureCertificatePath" required:"false" split_words:"true"`
	AzureCertificatePassword                   string   `json:"azureCertificatePassword" required:"false" split_words:"true"`
	AzureUserAssertionString                   string   `json:"azureUserAssertionString" required:"false" split_words:"true"`
	DiscoveryAttributeExcludesScaleSetInstance []string `json:"discoveryAttributeExcludesScaleSetInstance" required:"false" split_words:"true"`
	DiscoveryAttributeExcludesVM               []string `json:"discoveryAttributeExcludesVM" required:"false" split_words:"true"`
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

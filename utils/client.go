package utils

import (
  "github.com/Azure/azure-sdk-for-go/sdk/azcore"
  "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/rs/zerolog/log"
	"os"
)

func GetClientByCredentials() *armresourcegraph.Client {
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	tenantID := os.Getenv("AZURE_TENANT_ID")

	// Constructs a ClientSecretCredential. Pass nil for options to accept defaults.
	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		log.Error().Msgf("failed to create credential: %v", err)
	}

	// Create and authorize a ResourceGraph client
	client, err := armresourcegraph.NewClient(cred, nil)

	return client
}


func GetVirtualMachinesClient(subscriptionId string) (*armcompute.VirtualMachinesClient, error) {
  conn, err := connectionAzure()
  if err != nil {
    log.Fatal().Err(err).Msgf("Failed to create Azure connection.")
    return nil, err
  }
  //subscriptionId := os.Getenv("AZURE_SUBSCRIPTION_ID")
  computeClientFactory, err := armcompute.NewClientFactory(subscriptionId, conn, nil)
  if err != nil {
    log.Fatal().Err(err).Msgf("Failed to create Azure compute client.")
    return nil, err
  }
  virtualMachinesClient := computeClientFactory.NewVirtualMachinesClient()
  return virtualMachinesClient, nil
}

func connectionAzure() (azcore.TokenCredential, error) {
  cred, err := azidentity.NewDefaultAzureCredential(nil)
  if err != nil {
    log.Warn().Err(err).Msgf("Failed to create default Azure credential.")

    clientID := os.Getenv("AZURE_CLIENT_ID")
    clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
    tenantID := os.Getenv("AZURE_TENANT_ID")

    // Constructs a ClientSecretCredential. Pass nil for options to accept defaults.
    cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
    if err != nil {
      log.Error().Msgf("failed to create credential: %v", err)
    }
    return cred, err
  }
  return cred, nil
}

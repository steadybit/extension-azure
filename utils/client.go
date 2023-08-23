package utils

import (
	"crypto"
	"crypto/x509"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-azure/config"
	"os"
)

func GetClientByCredentials() (*armresourcegraph.Client, error) {
	cred, err := connectionAzure()
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create Azure connection.")
		return nil, err
	}

	// Create and authorize a ResourceGraph client
	client, err := armresourcegraph.NewClient(cred, nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create Azure resource graph client.")
		return nil, err
	}

	return client, nil
}

func GetVirtualMachinesClient(subscriptionId string) (*armcompute.VirtualMachinesClient, error) {
	conn, err := connectionAzure()
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create Azure connection.")
		return nil, err
	}
	computeClientFactory, err := armcompute.NewClientFactory(subscriptionId, conn, nil)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create Azure compute client.")
		return nil, err
	}
	virtualMachinesClient := computeClientFactory.NewVirtualMachinesClient()
	return virtualMachinesClient, nil
}

func connectionAzure() (azcore.TokenCredential, error) {
	tenantID := os.Getenv("AZURE_TENANT_ID")
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	userAssertionString := config.Config.AzureUserAssertionString

	certificateLocation := config.Config.AzureCertificateLocation
	certificatePassphrase := config.Config.AzureCertificatePassword
	if certificateLocation != "" {
		if userAssertionString != "" {
			return credsByCertificateOnUserBehalf(certificateLocation, certificatePassphrase, userAssertionString, tenantID, clientID)
		}
		return credsByCertificate(certificateLocation, certificatePassphrase, tenantID, clientID)
	}

	if userAssertionString != "" {
		return credsByUserAssertion(userAssertionString, tenantID, clientID, clientSecret)
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to create default Azure credential.")

		// Constructs a ClientSecretCredential. Pass nil for options to accept defaults.
		cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
		if err != nil {
			log.Error().Msgf("failed to create credential: %v", err)
		}
		return cred, err
	}
	return cred, nil
}

func credsByUserAssertion(userAssertionString string, tenantID string, clientID string, clientSecret string) (azcore.TokenCredential, error) {
	cred, err := azidentity.NewOnBehalfOfCredentialWithSecret(
		tenantID,
		clientID,
		userAssertionString,
		clientSecret,
		nil,
	)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create Azure credential with user assertion.")
		return nil, err
	}
	return cred, nil
}

func credsByCertificate(certificateLocation string, passphrase string, tenantID string, clientID string) (azcore.TokenCredential, error) {
	certs, key, err := getCertsAndKey(certificateLocation, passphrase)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get certificate and key.")
		return nil, err
	}

	cred, err := azidentity.NewClientCertificateCredential(
		tenantID,
		clientID,
		certs,
		key,
		nil,
	)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create credential by certificate.")
		return nil, err
	}
	return cred, nil
}

func getCertsAndKey(certificateLocation string, passphrase string) ([]*x509.Certificate, crypto.PrivateKey, error) {
	log.Info().Msgf("Using certificate authentication.")
	certData, err := os.ReadFile(certificateLocation)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to read certificate file.")
		return nil, nil, err
	}
	certs, key, err := azidentity.ParseCertificates(certData, []byte(passphrase))
	if err != nil {
		log.Error().Err(err).Msgf("Failed to parse certificate.")
		return nil, nil, err
	}
	return certs, key, nil
}

func credsByCertificateOnUserBehalf(certificateLocation string, passphrase string, userAssertionString string, tenantID string, clientID string) (azcore.TokenCredential, error) {
	certs, key, err := getCertsAndKey(certificateLocation, passphrase)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to get certificate and key.")
		return nil, err
	}
	creds, err := azidentity.NewOnBehalfOfCredentialWithCertificate(tenantID, clientID, userAssertionString, certs, key, nil)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to create Azure credential by NewOnBehalfOfCredentialWithCertificate.")
		return nil, err
	}
	return creds, nil
}

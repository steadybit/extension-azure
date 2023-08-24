<img src="./logo.svg" height="130" align="right" alt="JVM logo">

# Steadybit extension-azure

A [Steadybit](https://www.steadybit.com/) discovery and attack implementation to inject faults into various Azure services.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_azure).

## Configuration

| Environment Variable                             | Helm value                | Meaning                                                      | Required | Default |
|--------------------------------------------------|---------------------------|--------------------------------------------------------------|----------|---------|
| `AZURE_CLIENT_ID`                                | azure.clientID            | Azure Client Id                                              | true     |         |
| `AZURE_TENANT_ID`                                | azure.tenantID            | Azure Tenant ID                                              | true     |         |
| `AZURE_CLIENT_SECRET`                            | azure.clientSecret        | Azure Client Secret                                          | false    |         |
| `AZURE_SUBSCRIPTION_ID`                          | azure.subscriptionID      | Azure Subscription ID                                        | false    |         |
| `STEADYBIT_EXTENSION_AZURE_CERTIFICATE_PATH`     | azure.certificatePath     | Location of a certificate used to authenticate to azure      | false    |         |
| `STEADYBIT_EXTENSION_AZURE_CERTIFICATE_PASSWORD` | azure.certificatePassword | Passphrase for the certificate used to authenticate to azure | false    |         |


The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

The obtain the needed azure keys, please refer to this documentation:
https://learn.microsoft.com/en-us/azure/active-directory/develop/howto-create-service-principal-portal#get-tenant-and-app-id-values-for-signing-in

## Installation

### Using Docker

```sh
docker run \
  --rm \
  -p 8092 \
  --name steadybit-extension-azure \
  -e AZURE_CLIENT_ID='YOUR_CLIENT_ID' \
  -e AZURE_CLIENT_SECRET='YOUR_CLIENT_SECRET' \
  -e AZURE_TENANT_ID='YOUR_TENANT_ID' \
  ghcr.io/steadybit/extension-azure:latest
```

### Using Helm in Kubernetes

```sh
helm repo add steadybit-extension-azure https://steadybit.github.io/extension-azure
helm repo update
helm upgrade steadybit-extension-azure \
    --install \
    --wait \
    --timeout 5m0s \
    --create-namespace \
    --namespace steadybit-extension \
    --set azure.clientID=YOUR_CLIENT_ID \
    --set azure.clientSecret=YOUR_CLIENT_SECRET \
    --set azure.tenantID=YOUR_TENANT_ID \
    steadybit-extension-azure/steadybit-extension-azure
```

## Register the extension

Make sure to register the extension at the steadybit platform. Please refer to
the [documentation](https://docs.steadybit.com/integrate-with-steadybit/extensions/extension-installation) for more information.

<img src="./logo.svg" height="130" align="right" alt="JVM logo">

# Steadybit extension-azure

A [Steadybit](https://www.steadybit.com/) discovery and attack implementation to inject faults into various Azure services.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_azure).

## Configuration

| Environment Variable    | Helm value | Meaning               | Required | Default |
|-------------------------|------------|-----------------------|----------|---------|
| `AZURE_CLIENT_ID`       |            | Azure Client Id       | yes      |         |
| `AZURE_CLIENT_SECRET`   |            | Azure Client Secret   | yes      |         |
| `AZURE_SUBSCRIPTION_ID` |            | Azure Subscription ID | yes      |         |
| `AZURE_TENANT_ID`       |            | Azure Tenant ID       | yes      |         |


The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

The obtain the needed azure keys, please refer to this documentation: https://github.com/Azure-Samples/azure-sdk-for-go-samples/tree/main and
https://learn.microsoft.com/en-us/azure/active-directory/develop/howto-create-service-principal-portal#get-tenant-and-app-id-values-for-signing-in
## Installation

### Using Docker

```sh
docker run \
  --rm \
  -p 8092 \
  --name steadybit-extension-azure \
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
    steadybit-extension-azure/steadybit-extension-azure
```

## Register the extension

Make sure to register the extension at the steadybit platform. Please refer to
the [documentation](https://docs.steadybit.com/integrate-with-steadybit/extensions/extension-installation) for more information.

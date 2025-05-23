<img src="./logo.svg" height="130" align="right" alt="Azure logo">

# Steadybit extension-azure

A [Steadybit](https://www.steadybit.com/) discovery and attack implementation to inject faults into various Azure services.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_azure).

## Configuration

| Environment Variable                                                   | Helm value                                     | Meaning                                                                                                                | Required | Default |
|------------------------------------------------------------------------|------------------------------------------------|------------------------------------------------------------------------------------------------------------------------|----------|---------|
| `AZURE_CLIENT_ID`                                                      | azure.clientID                                 | Azure Client Id                                                                                                        | true     |         |
| `AZURE_TENANT_ID`                                                      | azure.tenantID                                 | Azure Tenant ID                                                                                                        | true     |         |
| `AZURE_CLIENT_SECRET`                                                  | azure.clientSecret                             | Azure Client Secret                                                                                                    | false    |         |
| `AZURE_SUBSCRIPTION_ID`                                                | azure.subscriptionID                           | Azure Subscription ID                                                                                                  | false    |         |
| `STEADYBIT_EXTENSION_AZURE_CERTIFICATE_PATH`                           | azure.certificatePath                          | Location of a certificate used to authenticate to azure                                                                | false    |         |
| `STEADYBIT_EXTENSION_AZURE_CERTIFICATE_PASSWORD`                       | azure.certificatePassword                      | Passphrase for the certificate used to authenticate to azure                                                           | false    |         |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_VM`                 | discovery.attributes.excludes.vm               | List of Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*" | false    |         |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_SCALE_SET_INSTANCE` | discovery.attributes.excludes.scaleSetInstance | List of Target Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*" | false    |         |

The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

When installed as linux package this configuration is in`/etc/steadybit/extension-azure`.

To obtain the needed azure keys, please refer to this documentation:
https://learn.microsoft.com/en-us/azure/active-directory/develop/howto-create-service-principal-portal#get-tenant-and-app-id-values-for-signing-in

## Installation

### Kubernetes

Detailed information about agent and extension installation in kubernetes can also be found in
our [documentation](https://docs.steadybit.com/install-and-configure/install-agent/install-on-kubernetes).

#### Recommended (via agent helm chart)

All extensions provide a helm chart that is also integrated in the
[helm-chart](https://github.com/steadybit/helm-charts/tree/main/charts/steadybit-agent) of the agent.

You must provide additional values to activate this extension.

```
--set extension-azure.enabled=true \
--set extension-azure.azure.clientID=YOUR_CLIENT_ID \
--set extension-azure.azure.clientSecret=YOUR_CLIENT_SECRET \
--set extension-azure.azure.tenantID=YOUR_TENANT_ID \
```

Additional configuration options can be found in
the [helm-chart](https://github.com/steadybit/extension-azure/blob/main/charts/steadybit-extension-azure/values.yaml) of the
extension.

#### Alternative (via own helm chart)

If you need more control, you can install the extension via its
dedicated [helm-chart](https://github.com/steadybit/extension-azure/blob/main/charts/steadybit-extension-azure).

```sh
helm repo add steadybit-extension-azure https://steadybit.github.io/extension-azure
helm repo update
helm upgrade steadybit-extension-azure \
    --install \
    --wait \
    --timeout 5m0s \
    --create-namespace \
    --namespace steadybit-agent \
    --set azure.clientID=YOUR_CLIENT_ID \
    --set azure.clientSecret=YOUR_CLIENT_SECRET \
    --set azure.tenantID=YOUR_TENANT_ID \
    steadybit-extension-azure/steadybit-extension-azure
```

### Linux Package

Please use
our [agent-linux.sh script](https://docs.steadybit.com/install-and-configure/install-agent/install-on-linux-hosts)
to install the extension on your Linux machine. The script will download the latest version of the extension and install
it using the package manager.

After installing, configure the extension by editing `/etc/steadybit/extension-azure` and then restart the service.

## Extension registration

Make sure that the extension is registered with the agent. In most cases this is done automatically. Please refer to
the [documentation](https://docs.steadybit.com/install-and-configure/install-agent/extension-registration) for more
information about extension registration and how to verify.

## Version and Revision

The version and revision of the extension:
- are printed during the startup of the extension
- are added as a Docker label to the image
- are available via the `version.txt`/`revision.txt` files in the root of the image

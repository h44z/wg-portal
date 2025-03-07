# wg-portal

![Version: 0.7.1](https://img.shields.io/badge/Version-0.7.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v2](https://img.shields.io/badge/AppVersion-v2-informational?style=flat-square)

WireGuard Configuration Portal with LDAP, OAuth, OIDC authentication

**Homepage:** <https://wgportal.org>

## Source Code

* <https://github.com/h44z/wg-portal>

## Requirements

Kubernetes: `>=1.19.0`

## Installing the Chart

To install the chart with the release name `wg-portal`:

```console
helm install wg-portal oci://ghcr.io/h44z/charts/wg-portal
```

This command deploy wg-portal on the Kubernetes cluster in the default configuration.
The [Values](#values) section lists the parameters that can be configured during installation.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| nameOverride | string | `""` | Partially override resource names (adds suffix) |
| fullnameOverride | string | `""` | Fully override resource names |
| extraDeploy | list | `[]` | Array of extra objects to deploy with the release |
| config.advanced | tpl/object | `{}` | [Advanced configuration](https://wgportal.org/latest/documentation/configuration/overview/#advanced) options. |
| config.auth | tpl/object | `{}` | [Auth configuration](https://wgportal.org/latest/documentation/configuration/overview/#auth) options. |
| config.core | tpl/object | `{}` | [Core configuration](https://wgportal.org/latest/documentation/configuration/overview/#core) options.<br> If external admins in `auth` are defined and there are no `admin_user` and `admin_password` defined here, the default admin account will be disabled. |
| config.database | tpl/object | `{}` | [Database configuration](https://wgportal.org/latest/documentation/configuration/overview/#database) options |
| config.mail | tpl/object | `{}` | [Mail configuration](https://wgportal.org/latest/documentation/configuration/overview/#mail) options |
| config.statistics | tpl/object | `{}` | [Statistics configuration](https://wgportal.org/latest/documentation/configuration/overview/#statistics) options |
| config.web | tpl/object | `{}` | [Web configuration](https://wgportal.org/latest/documentation/configuration/overview/#web) options.<br> `listening_address` will be set automatically from `service.web.port`. `external_url` is required to enable ingress and certificate resources. |
| revisionHistoryLimit | string | `10` | The number of old ReplicaSets to retain to allow rollback. |
| workloadType | string | `"Deployment"` | Workload type - `Deployment` or `StatefulSet` |
| strategy | object | `{"type":"RollingUpdate"}` | Update strategy for the workload Valid values are:  `RollingUpdate` or `Recreate` for Deployment,  `RollingUpdate` or `OnDelete` for StatefulSet |
| image.repository | string | `"ghcr.io/h44z/wg-portal"` | Image repository |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion |
| imagePullSecrets | list | `[]` | Image pull secrets |
| podAnnotations | tpl/object | `{}` | Extra annotations to add to the pod |
| podLabels | object | `{}` | Extra labels to add to the pod |
| podSecurityContext | object | `{}` | Pod Security Context |
| securityContext.capabilities.add | list | `["NET_ADMIN"]` | Add capabilities to the container |
| initContainers | tpl/list | `[]` | Pod init containers |
| sidecarContainers | tpl/list | `[]` | Pod sidecar containers |
| dnsPolicy | string | `"ClusterFirst"` | Set DNS policy for the pod. Valid values are `ClusterFirstWithHostNet`, `ClusterFirst`, `Default` or `None`. |
| restartPolicy | string | `"Always"` | Restart policy for all containers within the pod. Valid values are `Always`, `OnFailure` or `Never`. |
| hostNetwork | string | `false`. | Use the host's network namespace. |
| resources | object | `{}` | Resources requests and limits |
| command | list | `[]` | Overwrite pod command |
| args | list | `[]` | Additional pod arguments |
| env | tpl/list | `[]` | Additional environment variables |
| envFrom | tpl/list | `[]` | Additional environment variables from a secret or configMap |
| livenessProbe | object | `{}` | Liveness probe configuration |
| readinessProbe | object | `{}` | Readiness probe configuration |
| startupProbe | object | `{}` | Startup probe configuration |
| volumes | tpl/list | `[]` | Additional volumes |
| volumeMounts | tpl/list | `[]` | Additional volumeMounts |
| nodeSelector | object | `{"kubernetes.io/os":"linux"}` | Node Selector configuration |
| tolerations | list | `[]` | Tolerations configuration |
| affinity | object | `{}` | Affinity configuration |
| service.mixed.enabled | bool | `false` | Whether to create a single service for the web and wireguard interfaces |
| service.mixed.type | string | `"LoadBalancer"` | Service type |
| service.web.annotations | object | `{}` | Annotations for the web service |
| service.web.type | string | `"ClusterIP"` | Web service type |
| service.web.port | int | `8888` | Web service port Used for the web interface listener |
| service.web.appProtocol | string | `"http"` | Web service appProtocol. Will be auto set to `https` if certificate is enabled. |
| service.wireguard.annotations | object | `{}` | Annotations for the WireGuard service |
| service.wireguard.type | string | `"LoadBalancer"` | Wireguard service type |
| service.wireguard.ports | list | `[51820]` | Wireguard service ports. Exposes the WireGuard ports for created interfaces. Lowerest port is selected as start port for the first interface. Increment next port by 1 for each additional interface. |
| service.metrics.port | int | `8787` |  |
| ingress.enabled | bool | `false` | Specifies whether an ingress resource should be created |
| ingress.className | string | `""` | Ingress class name |
| ingress.annotations | object | `{}` | Ingress annotations |
| ingress.tls | bool | `false` | Ingress TLS configuration. Enable certificate resource or add ingress annotation to create required secret |
| certificate.enabled | bool | `false` | Specifies whether a certificate resource should be created. If enabled, certificate will be used for the web. |
| certificate.issuer.name | string | `""` | Certificate issuer name |
| certificate.issuer.kind | string | `""` | Certificate issuer kind (ClusterIssuer or Issuer) |
| certificate.issuer.group | string | `"cert-manager.io"` | Certificate issuer group |
| certificate.duration | string | `""` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| certificate.renewBefore | string | `""` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| certificate.commonName | string | `""` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| certificate.emailAddresses | list | `[]` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| certificate.ipAddresses | list | `[]` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| certificate.keystores | object | `{}` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| certificate.privateKey | object | `{}` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| certificate.secretTemplate | object | `{}` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| certificate.subject | object | `{}` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| certificate.uris | list | `[]` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| certificate.usages | list | `[]` | Optional. [Documentation](https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources) |
| persistence.enabled | bool | `false` | Specifies whether an persistent volume should be created |
| persistence.annotations | object | `{}` | Persistent Volume Claim annotations |
| persistence.storageClass | string | `""` | Persistent Volume storage class. If undefined (the default) cluster's default provisioner will be used. |
| persistence.accessMode | string | `"ReadWriteOnce"` | Persistent Volume Access Mode |
| persistence.size | string | `"1Gi"` | Persistent Volume size |
| persistence.volumeName | string | `""` | Persistent Volume Name (optional) |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.annotations | object | `{}` | Service account annotations |
| serviceAccount.automount | bool | `false` | Automatically mount a ServiceAccount's API credentials |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |
| monitoring.enabled | bool | `false` | Enable Prometheus monitoring. |
| monitoring.apiVersion | string | `"monitoring.coreos.com/v1"` | API version of the Prometheus resource. Use `azmonitoring.coreos.com/v1` for Azure Managed Prometheus. |
| monitoring.kind | string | `"PodMonitor"` | Kind of the Prometheus resource. Could be `PodMonitor` or `ServiceMonitor`. |
| monitoring.labels | object | `{}` | Resource labels. |
| monitoring.annotations | object | `{}` | Resource annotations. |
| monitoring.interval | string | `1m` | Interval at which metrics should be scraped. If not specified `config.statistics.data_collection_interval` interval is used. |
| monitoring.metricRelabelings | list | `[]` | Relabelings to samples before ingestion. |
| monitoring.relabelings | list | `[]` | Relabelings to samples before scraping. |
| monitoring.scrapeTimeout | string | `""` | Timeout after which the scrape is ended If not specified, the Prometheus global scrape interval is used. |
| monitoring.jobLabel | string | `""` | The label to use to retrieve the job name from. |
| monitoring.podTargetLabels | object | `{}` | Transfers labels on the Kubernetes Pod onto the target. |
| monitoring.dashboard.enabled | bool | `false` | Enable Grafana dashboard. |
| monitoring.dashboard.annotations | object | `{}` | Annotations for the dashboard ConfigMap. |
| monitoring.dashboard.labels | object | `{}` | Additional labels for the dashboard ConfigMap. |
| monitoring.dashboard.namespace | string | `""` | Dashboard ConfigMap namespace Overrides the namespace for the dashboard ConfigMap. |

# wg-portal

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v2.0.0-alpha.2](https://img.shields.io/badge/AppVersion-v2.0.0--alpha.2-informational?style=flat-square)

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

### Parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity configuration |
| args | list | `[]` | Additional pod arguments |
| command | list | `[]` | Overwrite pod command |
| dnsPolicy | string | `"ClusterFirst"` | Set DNS policy for the pod. Valid values are `ClusterFirstWithHostNet`, `ClusterFirst`, `Default` or `None`. |
| env | list | `[]` | Additional environment variables |
| envFrom | list | `[]` | Additional environment variables from a secret or configMap |
| hostNetwork | string | `false`. | Use the host's network namespace. |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.repository | string | `"ghcr.io/h44z/wg-portal"` | Image repository |
| image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion. |
| imagePullSecrets | list | `[]` | Image pull secrets |
| initContainers | list | `[]` | Pod init containers. Evaluated as a template |
| nodeSelector | object | `{"kubernetes.io/os":"linux"}` | Node Selector configuration |
| podAnnotations | object | `{}` | Extra annotations to add to the pod |
| podLabels | object | `{}` | Extra labels to add to the pod |
| podSecurityContext | object | `{}` | Pod Security Context |
| resources | object | `{}` | Resources requests and limits |
| restartPolicy | string | `"Always"` | Restart policy for all containers within the pod. Valid values are `Always`, `OnFailure` or `Never`. |
| revisionHistoryLimit | string | `10` | The number of old ReplicaSets to retain to allow rollback. |
| securityContext.capabilities.add | list | `["NET_ADMIN"]` | Add capabilities to the container |
| sidecarContainers | list | `[]` | Pod sidecar containers. Evaluated as a template |
| strategy | object | `{"type":"RollingUpdate"}` | Update strategy for the workload Valid values are:  `RollingUpdate` or `Recreate` for Deployment,  `RollingUpdate` or `OnDelete` for StatefulSet |
| tolerations | list | `[]` | Tolerations configuration |
| volumeMounts | list | `[]` | Additional volumeMounts |
| volumes | list | `[]` | Additional volumes |
| workloadType | string | `"Deployment"` | Workload type - `Deployment` or `StatefulSet` |

### Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| config.advanced | object | `{}` | Advanced configuration options |
| config.auth | object | `{}` | Auth configuration options |
| config.core | object | `{}` | Core configuration options.<br> If external admins in `auth` are not defined and there are no `admin_user` and `admin_password` defined here, the default credentials will be generated. |
| config.database | object | `{}` | Database configuration options |
| config.mail | object | `{}` | Mail configuration options |
| config.statistics | object | `{}` | Statistics configuration options |
| config.web | object | `{}` | Web configuration options.<br> The chart will set `listening_address` automatically from `service.web.port`, and `external_url` from `ingress.host` if enabled. |

### Common

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| extraDeploy | list | `[]` | Array of extra objects to deploy with the release |
| fullnameOverride | string | `""` | Fully override resource names |
| nameOverride | string | `""` | Partially override resource names (adds suffix) |

### Traffic exposure

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| ingress.annotations | object | `{}` | Ingress annotations |
| ingress.className | string | `""` | Ingress class name |
| ingress.enabled | bool | `false` | Specifies whether an ingress resource should be created |
| ingress.host | string | `""` | Ingress host FQDN |
| ingress.path | string | `"/"` | Ingress path |
| ingress.pathType | string | `"ImplementationSpecific"` | Ingress path type |
| ingress.tls | list | `[]` | Ingress TLS configuration |
| service.web.annotations | object | `{}` | Annotations for the web service |
| service.web.port | int | `8888` | Web service port Used for the web interface listener |
| service.web.type | string | `"ClusterIP"` | Web service type |
| service.wireguard.annotations | object | `{}` | Annotations for the WireGuard service |
| service.wireguard.ports | list | `[51820]` | Wireguard service ports. Exposes the WireGuard ports for created interfaces. Lowerest port is selected as start port for the first interface. Increment next port by 1 for each additional interface. |
| service.wireguard.type | string | `"LoadBalancer"` | Wireguard service type |

### Persistence

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| persistence.accessMode | string | `"ReadWriteOnce"` | Persistent Volume Access Mode |
| persistence.annotations | object | `{}` | Persistent Volume Claim annotations |
| persistence.enabled | bool | `false` | Specifies whether an persistent volume should be created |
| persistence.size | string | `"1Gi"` | Persistent Volume size |
| persistence.storageClass | string | `""` | Persistent Volume storage class. If undefined (the default) cluster's default provisioner will be used. |

### RBAC

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| serviceAccount.annotations | object | `{}` | Service account annotations |
| serviceAccount.automount | bool | `false` | Automatically mount a ServiceAccount's API credentials |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | The name of the service account to use. If not set and create is true, a name is generated using the fullname template |

# MinIO Community Helm Chart

[![license](https://img.shields.io/badge/license-AGPL%20V3-blue)](https://github.com/minio/minio/blob/master/LICENSE)

MinIO is a High Performance Object Storage released under GNU Affero General Public License v3.0. It is API compatible with Amazon S3 cloud storage service. Use MinIO to build high performance infrastructure for machine learning, analytics and application data workloads.

| IMPORTANT |
| -------------------------- |
| This Helm chart is community built, maintained, and supported. MinIO does not guarantee support for any given bug, feature request, or update referencing this chart. <br/><br/> MinIO publishes a separate [MinIO Kubernetes Operator and Tenant Helm Chart](https://github.com/minio/operator/tree/master/helm) that is officially maintained and supported. MinIO strongly recommends using the MinIO Kubernetes Operator for production deployments. See [Deploy Operator With Helm](https://docs.min.io/community/minio-object-store/operations/deployments/k8s-deploy-operator-helm-on-kubernetes.html?ref=github) for additional documentation. |

## Introduction

This chart bootstraps MinIO Cluster on [Kubernetes](http://kubernetes.io) using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Helm cli with Kubernetes cluster configured.
- PV provisioner support in the underlying infrastructure. (We recommend using <https://github.com/minio/direct-csi>)
- Use Kubernetes version v1.19 and later for best experience.

## Configure MinIO Helm repo

```bash
helm repo add minio https://charts.min.io/
```

> **Note for this fork**: `charts.min.io` publishes the **upstream** MinIO chart,
> not this one - installing from that repo does not give you the minoss build or
> the metrics v3 scrape configuration in this directory. The fork publishes no
> chart repository; install the chart from a clone of this repository instead:
>
> ```bash
> helm install minio ./helm/minio --namespace minio --create-namespace
> ```
>
> The examples below install from a clone of this repository (`./helm/minio`),
> run from its root; they used to name the `minio/minio` chart, which resolves to
> upstream. `--name` (a Helm 2 flag) is gone as well - Helm 3 takes the release
> name as a positional argument.

### Installing the Chart

Install this chart using:

```bash
helm install --namespace minio --set rootUser=rootuser,rootPassword=rootpass123 --generate-name ./helm/minio
```

The command deploys MinIO on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

### Installing the Chart (toy-setup)

Minimal toy setup for testing purposes can be deployed using:

```bash
helm install --set resources.requests.memory=512Mi --set replicas=1 --set persistence.enabled=false --set mode=standalone --set rootUser=rootuser,rootPassword=rootpass123 --generate-name ./helm/minio
```

### Upgrading the Chart

You can use Helm to update MinIO version in a live release. Assuming your release is named as `my-release`, get the values using the command:

```bash
helm get values my-release > old_values.yaml
```

Then change the field `image.tag` in `old_values.yaml` file with MinIO image tag you want to use. Now update the chart using

```bash
helm upgrade -f old_values.yaml my-release ./helm/minio
```

Default upgrade strategies are specified in the `values.yaml` file. Update these fields if you'd like to use a different strategy.

### Configuration

Refer the [Values file](./values.yaml) for all the possible config fields.

You can specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```bash
helm install my-release --set persistence.size=1Ti ./helm/minio
```

The above command deploys MinIO server with a 1Ti backing persistent volume.

Alternately, you can provide a YAML file that specifies parameter values while installing the chart. For example,

```bash
helm install my-release -f values.yaml ./helm/minio
```

### Metrics scraping

`metrics.serviceMonitor.enabled` installs Prometheus Operator objects that scrape
**metrics v3** (`/minio/metrics/v3/<collector>`). The legacy
`/minio/v2/metrics/{node,cluster}` endpoints were removed from this fork, so
v2-era scrape configs no longer return metrics (the request falls through to the
reserved-bucket handler and is refused with `403`).

Unlike v2, v3 exposes one endpoint per collector group, so scraping is a list of
paths rather than two fixed URLs:

| Parameter | Purpose |
|-----------|---------|
| `metrics.serviceMonitor.paths` | Collector groups scraped **per pod** (node-local ones: `debug/go`, `system/process`, `system/cpu`, `system/memory`). Requires `includeNode: true`. |
| `metrics.serviceMonitor.clusterPaths` | Collector groups scraped **once for the cluster** (each aggregates from every peer, so scraping them per pod would collect the same data N times). One `Probe` is generated per path. |
| `metrics.serviceMonitor.bearerTokenSecret` | Name of a Secret holding the scrape token under `token`. Required when `public: false` - this chart does not create that Secret, and leaving it unset now fails the render instead of installing scrapes that can never authenticate. |

Every scrape carries a `collector` target label naming the collector group it
reads (e.g. `collector="cluster-health"`). Prometheus creates one scrape pool per
endpoint, and pools that share a target identity write to the *same* `up` /
`scrape_duration_seconds` series - so without this label, a `ServiceMonitor` with
four paths per pod reports four pools as one target, and one dead collector makes
the whole scrape look intermittently down. Alert on
`up{namespace="...",job="..."}` per collector, not per instance.

Some collectors answer `200` with an empty body on a fresh server (e.g.
`cluster/config` before any config is set, `cluster/usage/objects` before the
crawler has run). That is the collector having nothing to report, not a broken
scrape - `up` stays `1`.

Request any collector path with `?list` to see the metrics it serves; the full
set of 21 collectors is documented in [`docs/metrics/README.md`](../../docs/metrics/README.md).

```bash
helm install my-release ./helm/minio \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.includeNode=true \
  --set metrics.serviceMonitor.clusterPaths[0]=/minio/metrics/v3/cluster/health \
  --set metrics.serviceMonitor.clusterPaths[1]=/minio/metrics/v3/ilm
```

### Persistence

This chart provisions a PersistentVolumeClaim and mounts corresponding persistent volume to default location `/export`. You'll need physical storage available in the Kubernetes cluster for this to work. If you'd rather use `emptyDir`, disable PersistentVolumeClaim by:

```bash
helm install --set persistence.enabled=false ./helm/minio
```

> *"An emptyDir volume is first created when a Pod is assigned to a Node, and exists as long as that Pod is running on that node. When a Pod is removed from a node for any reason, the data in the emptyDir is deleted forever."*

### Existing PersistentVolumeClaim

If a Persistent Volume Claim already exists, specify it during installation.

1. Create the PersistentVolume
2. Create the PersistentVolumeClaim
3. Install the chart

```bash
helm install --set persistence.existingClaim=PVC_NAME ./helm/minio
```

### NetworkPolicy

To enable network policy for MinIO,
install [a networking plugin that implements the Kubernetes
NetworkPolicy spec](https://kubernetes.io/docs/tasks/administer-cluster/declare-network-policy#before-you-begin),
and set `networkPolicy.enabled` to `true`.

For Kubernetes v1.5 & v1.6, you must also turn on NetworkPolicy by setting
the DefaultDeny namespace annotation. Note: this will enforce policy for *all* pods in the namespace:

```
kubectl annotate namespace default "net.beta.kubernetes.io/network-policy={\"ingress\":{\"isolation\":\"DefaultDeny\"}}"
```

When using `Cilium` as a CNI in your cluster, please edit the `flavor` field to `cilium`.

With NetworkPolicy enabled, traffic will be limited to just port 9000.

For more precise policy, set `networkPolicy.allowExternal=true`. This will
only allow pods with the generated client label to connect to MinIO.
This label will be displayed in the output of a successful install.

### Existing secret

Instead of having this chart create the secret for you, you can supply a preexisting secret, much
like an existing PersistentVolumeClaim.

First, create the secret:

```bash
kubectl create secret generic my-minio-secret --from-literal=rootUser=foobarbaz --from-literal=rootPassword=foobarbazqux
```

Then install the chart, specifying that you want to use an existing secret:

```bash
helm install --set existingSecret=my-minio-secret ./helm/minio
```

The following fields are expected in the secret:

| .data.\<key\> in Secret | Corresponding variable | Description    | Required |
|:------------------------|:-----------------------|:---------------|:---------|
| `rootUser`              | `rootUser`             | Root user.     | yes      |
| `rootPassword`          | `rootPassword`         | Root password. | yes      |

All corresponding variables will be ignored in values file.

### Configure TLS

To enable TLS for MinIO containers, acquire TLS certificates from a CA or create self-signed certificates. While creating / acquiring certificates ensure the corresponding domain names are set as per the standard [DNS naming conventions](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#pod-identity) in a Kubernetes StatefulSet (for a distributed MinIO setup). Then create a secret using

```bash
kubectl create secret generic tls-ssl-minio --from-file=path/to/private.key --from-file=path/to/public.crt
```

Then install the chart, specifying that you want to use the TLS secret:

```bash
helm install --set tls.enabled=true,tls.certSecret=tls-ssl-minio ./helm/minio
```

### Installing certificates from third party CAs

MinIO can connect to other servers, including MinIO nodes or other server types such as NATs and Redis. If these servers use certificates that were not registered with a known CA, add trust for these certificates to MinIO Server by bundling these certificates into a Kubernetes secret and providing it to Helm via the `trustedCertsSecret` value. If `.Values.tls.enabled` is `true` and you're installing certificates for third party CAs, remember to include MinIO's own certificate with key `public.crt`, if it also needs to be trusted.

For instance, given that TLS is enabled and you need to add trust for MinIO's own CA and for the CA of a Keycloak server, a Kubernetes secret can be created from the certificate files using `kubectl`:

```
kubectl -n minio create secret generic minio-trusted-certs --from-file=public.crt --from-file=keycloak.crt
```

If TLS is not enabled, you would need only the third party CA:

```
kubectl -n minio create secret generic minio-trusted-certs --from-file=keycloak.crt
```

The name of the generated secret can then be passed to Helm using a values file or the `--set` parameter:

```
trustedCertsSecret: "minio-trusted-certs"

or

--set trustedCertsSecret=minio-trusted-certs
```

### Create buckets after install

Install the chart, specifying the buckets you want to create after install:

```bash
helm install --set buckets[0].name=bucket1,buckets[0].policy=none,buckets[0].purge=false ./helm/minio
```

Description of the configuration parameters used above -

- `buckets[].name` - name of the bucket to create, must be a string with length > 0
- `buckets[].policy` - can be one of none|download|upload|public
- `buckets[].purge` - purge if bucket exists already

### Create policies after install

Install the chart, specifying the policies you want to create after install:

```bash
helm install --set policies[0].name=mypolicy,policies[0].statements[0].resources[0]='arn:aws:s3:::bucket1',policies[0].statements[0].actions[0]='s3:ListBucket',policies[0].statements[0].actions[1]='s3:GetObject' ./helm/minio
```

Description of the configuration parameters used above -

- `policies[].name` - name of the policy to create, must be a string with length > 0
- `policies[].statements[]` - list of statements, includes actions and resources
- `policies[].statements[].resources[]` - list of resources that applies the statement
- `policies[].statements[].actions[]` - list of actions granted

### Create user after install

Install the chart, specifying the users you want to create after install:

```bash
helm install --set users[0].accessKey=accessKey,users[0].secretKey=secretKey,users[0].policy=none,users[1].accessKey=accessKey2,users[1].secretRef=existingSecret,users[1].secretKey=password,users[1].policy=none ./helm/minio
```

Description of the configuration parameters used above -

- `users[].accessKey` - accessKey of user
- `users[].secretKey` - secretKey of usersecretRef
- `users[].existingSecret` - secret name that contains the secretKey of user
- `users[].existingSecretKey` - data key in existingSecret secret containing the secretKey
- `users[].policy` - name of the policy to assign to user

### Create service account after install

Install the chart, specifying the service accounts you want to create after install:

```bash
helm install --set svcaccts[0].accessKey=accessKey,svcaccts[0].secretKey=secretKey,svcaccts[0].user=parentUser,svcaccts[1].accessKey=accessKey2,svcaccts[1].secretRef=existingSecret,svcaccts[1].secretKey=password,svcaccts[1].user=parentUser2 ./helm/minio
```

Description of the configuration parameters used above -

- `svcaccts[].accessKey` - accessKey of service account
- `svcaccts[].secretKey` - secretKey of svcacctsecretRef
- `svcaccts[].existingSecret` - secret name that contains the secretKey of service account
- `svcaccts[].existingSecretKey` - data key in existingSecret secret containing the secretKey
- `svcaccts[].user` - name of the parent user to assign to service account

## Uninstalling the Chart

Assuming your release is named as `my-release`, delete it using the command:

```bash
helm delete my-release
```

or

```bash
helm uninstall my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

# How to monitor MinIO server with Prometheus?

[Prometheus](https://prometheus.io) is a cloud-native monitoring platform. Prometheus offers a multi-dimensional data model with time series data identified by metric name and key/value pairs. The data collection happens via a pull model over HTTP/HTTPS. Users looking to monitor their MinIO instances can point Prometheus configuration to scrape data from following endpoints. 

> **Removed in this fork** — the metrics v1 and v2 scrape endpoints (`/minio/prometheus/metrics` and `/minio/v2/metrics/{cluster,bucket,node,resource}`) were removed in `86cada680`. Only the metrics v3 endpoints, `/minio/metrics/v3/<collector>`, are served; the examples below use them.

MinIO exports Prometheus compatible data by default as an authorized endpoint per collector group under `/minio/metrics/v3`, for example `/minio/metrics/v3/cluster/health` for cluster health and `/minio/metrics/v3/bucket/api/<bucket-name>` for bucket centric metrics.

This document explains how to setup Prometheus and configure it to scrape data from MinIO servers.

## Prerequisites

To get started with MinIO, refer [MinIO QuickStart Document](https://docs.min.io/community/minio-object-store/operations/deployments/baremetal-deploy-minio-on-redhat-linux.html).
Follow below steps to get started with MinIO monitoring using Prometheus.

### 1. Download Prometheus

[Download the latest release](https://prometheus.io/download) of Prometheus for your platform, then extract it

```sh
tar xvfz prometheus-*.tar.gz
cd prometheus-*
```

Prometheus server is a single binary called `prometheus` (or `prometheus.exe` on Microsoft Windows). Run the binary and pass `--help` flag to see available options

```sh
./prometheus --help
usage: prometheus [<flags>]

The Prometheus monitoring server

. . .
```

Refer [Prometheus documentation](https://prometheus.io/docs/introduction/first_steps/) for more details.

### 2. Configure authentication type for Prometheus metrics

MinIO supports two authentication modes for Prometheus either `jwt` or `public`, by default MinIO runs in `jwt` mode. To allow public access without authentication for prometheus metrics set environment as follows.

```
export MINIO_PROMETHEUS_AUTH_TYPE="public"
minio server ~/test
```

### 3. Configuring Prometheus

#### 3.1 Authenticated Prometheus config

> If MinIO is configured to expose metrics without authentication, you don't need to use `mc` to generate prometheus config. You can skip reading further and move to 3.2 section.

The Prometheus endpoint in MinIO requires authentication by default. Prometheus supports a bearer token approach to authenticate prometheus scrape requests, override the default Prometheus config with the one generated using mc. To generate a Prometheus config for an alias, use [mc](https://docs.min.io/community/minio-object-store/reference/minio-mc.html#quickstart) as follows `mc admin prometheus generate <alias> [METRIC-TYPE]`. The `cluster`, `node`, `bucket` and `resource` METRIC-TYPE values addressed the v2 endpoints removed in `86cada680`; metrics v3 exposes one endpoint per collector group instead, so a scrape job is needed per `/minio/metrics/v3/<collector>` path you want to collect.

The command will generate the `scrape_configs` section of the prometheus.yml as follows, adjust `metrics_path` to the collector group you need:

##### Cluster

```yaml
scrape_configs:
- job_name: minio-job
  bearer_token: <secret>
  metrics_path: /minio/metrics/v3/cluster/health
  scheme: http
  static_configs:
  - targets: ['localhost:9000']
```

##### Bucket centric

```yaml
- job_name: minio-job-bucket
  bearer_token: <secret>
  metrics_path: /minio/metrics/v3/bucket/api/<bucket-name>
  scheme: http
  static_configs:
  - targets: ['localhost:9000']
```

##### Node centric (optional)

```yaml
- job_name: minio-job-node
  bearer_token: <secret>
  metrics_path: /minio/metrics/v3/system/process
  scheme: http
  static_configs:
  - targets: ['localhost:9000']
```

##### Resource centric (optional)

```yaml
- job_name: minio-job-resource
  bearer_token: <secret>
  metrics_path: /minio/metrics/v3/system/drive
  scheme: http
  static_configs:
  - targets: ['localhost:9000']
```

#### 3.2 Public Prometheus config

If Prometheus endpoint authentication type is set to `public`. Following prometheus config is sufficient to start scraping metrics data from MinIO.
This can be collected from any server once per collection.

##### Cluster

```yaml
scrape_configs:
- job_name: minio-job
  metrics_path: /minio/metrics/v3/cluster/health
  scheme: http
  static_configs:
  - targets: ['localhost:9000']
```

#### Bucket centric

```yaml
scrape_configs:
- job_name: minio-job-bucket
  metrics_path: /minio/metrics/v3/bucket/api/<bucket-name>
  scheme: http
  static_configs:
  - targets: ['localhost:9000']
```

##### Node (optional)

Optionally you can also collect per node metrics. This needs to be done on a per server instance.
The scrape configurations should use all the servers under `targets` so that graphing systems like
grafana can visualize them for all the nodes

```yaml
scrape_configs:
- job_name: minio-job
  metrics_path: /minio/metrics/v3/system/process
  scheme: http
  static_configs:
  - targets: ['server1:9000','server2:9000','server3:9000','server4:9000']
```

##### Resource (optional)

Optionally you can also collect resource metrics.

```yaml
scrape_configs:
- job_name: minio-job
  metrics_path: /minio/metrics/v3/system/drive
  scheme: http
  static_configs:
  - targets: ['localhost:9000']
```

### 4. Update `scrape_configs` section in prometheus.yml

To authorize every scrape request, copy and paste the generated `scrape_configs` section in the prometheus.yml and restart the Prometheus service.

### 5. Start Prometheus

Start (or) Restart Prometheus service by running

```sh
./prometheus --config.file=prometheus.yml
```

Here `prometheus.yml` is the name of configuration file. You can now see MinIO metrics in Prometheus dashboard. By default Prometheus dashboard is accessible at `http://localhost:9090`.

Prometheus sets the `Host` header to `domain:port` as part of HTTP operations against the MinIO metrics endpoint. For MinIO deployments behind a load balancer, reverse proxy, or other control plane (HAProxy, nginx, pfsense, opnsense, etc.), ensure the network service supports routing these requests to the deployment.

### 6. Configure Grafana

After Prometheus is configured, you can use Grafana to visualize MinIO metrics. Refer the [document here to setup Grafana with MinIO prometheus metrics](https://github.com/minio/minio/blob/master/docs/metrics/prometheus/grafana/README.md).

## List of metrics exposed by MinIO

- MinIO exports Prometheus compatible data by default as an authorized endpoint per collector group under `/minio/metrics/v3`, e.g. `/minio/metrics/v3/cluster/health`.
- MinIO exports bucket centric data as an authorized endpoint at `/minio/metrics/v3/bucket/api/<bucket-name>` and `/minio/metrics/v3/bucket/replication/<bucket-name>`.
- MinIO exports node and resource centric data as authorized endpoints at `/minio/metrics/v3/system/*` and `/minio/metrics/v3/debug/go`.

All of these can be accessed via Prometheus dashboard. Every collector path also
serves a listing request, which returns the metrics that path can expose along
with their definition, e.g:

```sh
curl -H "Authorization: Bearer <secret>" "http://localhost:9000/minio/metrics/v3/cluster/health?list"
```

### List of metrics reported Cluster and Bucket level

[The list of metrics reported can be here](https://github.com/minio/minio/blob/master/docs/metrics/prometheus/list.md)

### Configure Alerts for Prometheus

[The Prometheus AlertManager and alerts can be configured following this](https://github.com/minio/minio/blob/master/docs/metrics/prometheus/alerts.md)

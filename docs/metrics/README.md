# MinIO Monitoring Guide

MinIO server exposes monitoring data over endpoints. Monitoring tools can pick the data from these endpoints. This document lists the monitoring endpoints and relevant documentation.

## Healthcheck Probe

MinIO server has two healthcheck related un-authenticated endpoints, a liveness probe to indicate if server is responding, cluster probe to check if server can be taken down for maintenance.

- Liveness probe available at `/minio/health/live`
- Cluster probe available at `/minio/health/cluster`

Read more on how to use these endpoints in [MinIO healthcheck guide](https://github.com/minio/minio/blob/master/docs/metrics/healthcheck/README.md).

## Prometheus Probe

MinIO allows reading metrics for the entire cluster from any single node. This allows for metrics collection for a MinIO instance across all servers. Thus, metrics collection for instances behind a load balancer can be done without any knowledge of the individual node addresses.

> **Removed in this fork** — the legacy metrics v1 and v2 scrape endpoints (`/minio/prometheus/metrics` and `/minio/v2/metrics/{cluster,bucket,node,resource}`) were removed in `86cada680`. Only metrics v3, described below, is served.

Metrics v3 are exposed as one endpoint per collector group, under
`<Address for MinIO Service>/minio/metrics/v3`. The collector groups served by
this build are:

```
/minio/metrics/v3/api/requests
/minio/metrics/v3/bucket/api/<bucket-name>
/minio/metrics/v3/bucket/replication/<bucket-name>
/minio/metrics/v3/system/network/internode
/minio/metrics/v3/system/drive
/minio/metrics/v3/system/memory
/minio/metrics/v3/system/cpu
/minio/metrics/v3/system/process
/minio/metrics/v3/debug/go
/minio/metrics/v3/cluster/health
/minio/metrics/v3/cluster/usage/objects
/minio/metrics/v3/cluster/usage/buckets
/minio/metrics/v3/cluster/erasure-set
/minio/metrics/v3/cluster/iam
/minio/metrics/v3/cluster/config
/minio/metrics/v3/ilm
/minio/metrics/v3/audit
/minio/metrics/v3/logger/webhook
/minio/metrics/v3/replication
/minio/metrics/v3/notification
/minio/metrics/v3/scanner
```

Bucket-centric collectors take the bucket name as the last path component. Any
collector path also accepts a `?list` query parameter, which returns the metrics
that path can serve.

To use this endpoint, setup Prometheus to scrape data from this endpoint. Read more on how to configure and use Prometheus to monitor MinIO server in [How to monitor MinIO server with Prometheus](https://github.com/minio/minio/blob/master/docs/metrics/prometheus/README.md).

### **Removed metrics monitoring**

- Prometheus' data previously available at `/minio/prometheus/metrics` was removed in `86cada680`

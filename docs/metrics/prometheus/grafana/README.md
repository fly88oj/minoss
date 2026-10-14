# How to monitor MinIO server with Grafana

> **Removed in this fork** — the metrics v1 and v2 scrape endpoints were removed in `86cada680`. The dashboards in this directory query v2 metric names (for example `minio_cluster_health_erasure_set_online_drives`, `minio_node_process_resident_memory_bytes`) that this build no longer serves, so their panels will show no data until the queries are rewritten against metrics v3. v3 serves one endpoint per collector group under `/minio/metrics/v3/<collector>` and derives names from the path (the erasure-set drive counts above live on `/minio/metrics/v3/cluster/erasure-set` as `minio_cluster_erasure_set_online_drives_count`); append `?list` to any collector path to obtain the names it serves.

[Grafana](https://grafana.com/) allows you to query, visualize, alert on and understand your metrics no matter where they are stored. Create, explore, and share dashboards with your team and foster a data driven culture.

## Prerequisites

- Prometheus and MinIO configured as explained in [document here](https://github.com/minio/minio/blob/master/docs/metrics/prometheus/README.md).
- Grafana installed as explained [here](https://grafana.com/grafana/download).

## MinIO Grafana Dashboard

Visualize MinIO metrics with our official Grafana dashboard available on the [Grafana dashboard portal](https://grafana.com/grafana/dashboards/13502).

Refer to the dashboard [json file here](https://raw.githubusercontent.com/minio/minio/master/docs/metrics/prometheus/grafana/minio-dashboard.json).

![Grafana](https://raw.githubusercontent.com/minio/minio/master/docs/metrics/prometheus/grafana/grafana-minio.png)

Node level Replication metrics can be viewed in the Grafana dashboard using [json file here](https://raw.githubusercontent.com/minio/minio/master/docs/metrics/prometheus/grafana/replication/minio-replication-node.json)

![Grafana](https://raw.githubusercontent.com/minio/minio/master/docs/metrics/prometheus/grafana/replication/grafana-replication-node.png)

Cluster level Replication metrics can be viewed in the Grafana dashboard using [json file here](https://raw.githubusercontent.com/minio/minio/master/docs/metrics/prometheus/grafana/replication/minio-replication-cluster.json)

![Grafana](https://raw.githubusercontent.com/minio/minio/master/docs/metrics/prometheus/grafana/replication/grafana-replication-cluster.png)

Bucket metrics can be viewed in the Grafana dashboard using [json file here](https://raw.githubusercontent.com/minio/minio/master/docs/metrics/prometheus/grafana/bucket/minio-bucket.json)

![Grafana](https://raw.githubusercontent.com/minio/minio/master/docs/metrics/prometheus/grafana/bucket/grafana-bucket.png)

Node metrics can be viewed in the Grafana dashboard using [json file here](https://raw.githubusercontent.com/minio/minio/master/docs/metrics/prometheus/grafana/node/minio-node.json)

![Grafana](https://raw.githubusercontent.com/minio/minio/master/docs/metrics/prometheus/grafana/node/grafana-node.png)

Note: All these dashboards are provided as an example and need basis they should be customized as well as new graphs should be added.

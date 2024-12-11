# CephFS XAttr Prometheus Exporter

Prometheus exporter that publishes size information to Prometheus, by reading xattributes.

## Environment Variables

- `CEPH_USER` : User to connect to ceph cluster (default: `admin`).
- `CEPH_CONFIG` : Config to connect to ceph cluster (default: `/etc/ceph/ceph.conf`).
- `TELEMETRY_PORT` : Port of the ceph exporter (default: `:9128`).
- `TELEMETRY_PATH` : URL path for surfacing metrics to Prometheus (default: `/metrics`).

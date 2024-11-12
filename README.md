# CephFS XAttr Prometheus Exporter
Prometheus exporter that publishes xattributes as metrics.

### Configuration

Create `paths.json` configuration with paths for which the attributes should be exported (see `example.paths.json`)

### Environment Variables
- `CEPH_USER` : User to connect to ceph cluster (default: `admin`).
- `CEPH_CONFIG` : Config to connect to ceph cluster (default: `/etc/ceph/ceph.conf`).
- `TELEMETRY_PORT` : Port of the ceph exporter (default: `:9128`).
- `TELEMETRY_PATH` : URL path for surfacing metrics to Prometheus (default: `/metrics`).
- `UPDATE_INTERVAL` : Interval to scrape ceph metrics in seconds (default: `15`)


### TODO
- [ x ] Make custom `CEPH_USER`, `CEPH_CONFIG`
- [ x ] Add option to choose `TELMETRY_PORT`, `TELEMETRY_PATH`
- [ x ] Add option to choose `UPDATE_INTERVAL`
- [ ] Recurse the directory if it is large enough
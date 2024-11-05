# CephFS XAttr Prometheus Exporter
Prometheus exporter that publishes xattributes as metrics.

### Configuration

Create `paths.json` configuration with paths for which the attributes should be exported (see `example.paths.json`)

For now, the default user is `admin`. Ceph configuration should be located at `/etc/ceph/ceph.conf`. You should also have `/etc/ceph/ceph.<user>.keyring` to authenticate to the ceph cluster.

### TODO
- [ ] Make custom `CEPH_CLUSTER`, `CEPH_USER`, `CEPH_CONFIG`
- [ ] Add option to choose `TELMETRY_PORT`
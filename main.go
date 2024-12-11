package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/ceph/go-ceph/cephfs"
	rados "github.com/ceph/go-ceph/rados"
	"github.com/ianschenck/envflag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultCephConfigPath  = "/etc/ceph/ceph.conf"
	defaultCephUser        = "admin"
	directorySizeToRecurse = 100_000_000_000 // 100 GB
)

var (
	rbytesDesc = prometheus.NewDesc(
		"cephfs_rbytes",
		"Total size of directory in bytes",
		[]string{"path"}, nil,
	)
	rentriesDesc = prometheus.NewDesc(
		"cephfs_rentries",
		"Total number of files and subdirectories",
		[]string{"path"}, nil,
	)
)

type Collector struct {
	prometheus.Collector
	filesystem *cephfs.MountInfo
}

func (c Collector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

func (c Collector) Collect(ch chan<- prometheus.Metric) {
	err := c.observePath("/", ch, false)
	if err != nil {
		log.Print(err)
	}
}

func getNumXattr(filesystem *cephfs.MountInfo, path string, attr string) (float64, error) {
	value, err := filesystem.GetXattr(path, attr)
	if err != nil {
		return -1, err
	}
	num, err := strconv.ParseFloat(string(value), 64)
	if err != nil {
		return -1, fmt.Errorf("Invalid number")
	}
	return num, nil
}

func (c Collector) observePath(path string, ch chan<- prometheus.Metric, optional bool, level int) error {
	// Read rbytes
	rbytes, err := getNumXattr(c.filesystem, path, "ceph.dir.rbytes")
	if err != nil {
		return fmt.Errorf("Getting rbytes: %w", err)
	}

	// If we are recursing and this directory is small, stop
	if optional && rbytes < directorySizeToRecurse {
		return nil
	}

	// Read entries
	rentries, err := getNumXattr(c.filesystem, path, "ceph.dir.rentries")
	if err != nil {
		return fmt.Errorf("Getting rentries: %w", err)
	}

	// Emit metrics
	ch <- prometheus.MustNewConstMetric(
		rbytesDesc,
		prometheus.GaugeValue,
		rbytes,
		path,
	)
	ch <- prometheus.MustNewConstMetric(
		rentriesDesc,
		prometheus.GaugeValue,
		rentries,
		path,
	)

	// Recurse
	if rbytes >= directorySizeToRecurse {
		dir, err := c.filesystem.OpenDir(path)
		if err != nil {
			return fmt.Errorf("Opening directory: %w", err)
		}
		for {
			entryDir, err := dir.ReadDir()
			if err != nil {
				return fmt.Errorf("Reading directory: %w", err)
			}
			if entryDir == nil {
				break
			}
			if entryDir.Name() == "." || entryDir.Name() == ".." {
				continue
			}
			if entryDir.DType() == 4 {
				err := c.observePath(
					filepath.Join(path, entryDir.Name()),
					ch,
					true, // optional, only observe if big enough
				)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func main() {
	var (
		metricsAddr = envflag.String("TELEMETRY_ADDR", ":9128", "Host:Port for metrics endpoint")
		metricsPath = envflag.String("TELEMETRY_PATH", "/metrics", "URL path for metrics endpoint")
		cephConfig  = envflag.String("CEPH_CONFIG", defaultCephConfigPath, "Path to Ceph config file")
		cephUser    = envflag.String("CEPH_USER", defaultCephUser, "Ceph user to connect to cluster")
	)

	envflag.Parse()
	conn, err := rados.NewConnWithUser(*cephUser)
	if err != nil {
		log.Fatalf("Failed to create rados connection: %v", err)
	}
	err = conn.ReadConfigFile(*cephConfig)
	if err != nil {
		log.Fatalf("Failed to read config file: %s", err)
	}

	err = conn.ReadDefaultConfigFile()
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	err = conn.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to the cluster: %v", err)
	}
	defer conn.Shutdown()
	log.Print("Successfully connected to Ceph cluster!")

	filesystem, err := cephfs.CreateFromRados(conn)
	if err != nil {
		log.Fatalf("Failed to create cephfs mountinfo: %v", err)
	}

	if err := filesystem.Mount(); err != nil {
		log.Fatalf("Failed to mount filesystem: %v", err)
	}
	defer filesystem.Unmount()
	log.Print("Successfully mounted Ceph filesystem!")

	prometheus.MustRegister(Collector{filesystem: filesystem})
	http.Handle(*metricsPath, promhttp.Handler())

	log.Printf("Starting server on %s\n", *metricsAddr)
	log.Fatal(http.ListenAndServe(*metricsAddr, nil))
}

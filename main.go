package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/ceph/go-ceph/cephfs"
	rados "github.com/ceph/go-ceph/rados"
	"github.com/ianschenck/envflag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultCephClusterLabel = "ceph"
	defaultCephConfigPath   = "/etc/ceph/ceph.conf"
	defaultCephUser         = "admin"
)

type PathEntry struct {
	Organisation string `json:"Organisation"`
	User         string `json:"User"`
	Path         string `json:"Path"`
}

func (p *PathEntry) Tags() map[string]string {
	return map[string]string{
		"org":  p.Organisation,
		"user": p.User,
		"path": p.Path,
	}
}

type DynamicMetrics struct {
	gauges map[string]prometheus.GaugeVec
}

func NewDynamicMetrics() *DynamicMetrics {
	return &DynamicMetrics{
		gauges: make(map[string]prometheus.GaugeVec),
	}
}
func (dm *DynamicMetrics) AddGauge(name, help string) prometheus.GaugeVec {
	if gauge, exists := dm.gauges[name]; exists {
		return gauge
	}

	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, []string{"org", "user", "path"})
	prometheus.MustRegister(gauge)
	dm.gauges[name] = *gauge
	return *gauge
}

func LoadDirsConfig(path string) []PathEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read JSON file: %v", err)
	}

	var entries []PathEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		log.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	return entries
}

func (dm *DynamicMetrics) UpdateMetrics(info *cephfs.MountInfo) {
	attrs := []string{"ceph.dir.rbytes", "ceph.dir.rentries", "ceph.dir.rfiles"}
	entries := LoadDirsConfig("paths.json")
	for _, entry := range entries {
		fmt.Printf("Organisation: %s, User: %s, Path: %s\n", entry.Organisation, entry.User, entry.Path)
	}
	for _, entry := range entries {
		for _, attr := range attrs {
			value, err := info.GetXattr(entry.Path, attr)
			if err != nil {
				fmt.Printf("Error for path '%s' and attribute '%s'. Error msg: %s", entry.Path, attr, err)
				continue
			}
			f, err := strconv.ParseFloat(string(value), 64)
			fmt.Println("path:", entry.Path, "attr:", attr, "value:", f)
			if err != nil {
				log.Printf("unable to convert %q to float: %v", value, err)
				continue
			}
			name := fmt.Sprintf("cephfs_xattr_%s", strings.ReplaceAll(attr, ".", "_"))
			gauge := dm.AddGauge(name, "A dynamically generated metric")
			gauge.With(entry.Tags()).Set(f)
		}
	}
}

func main() {
	var (
		metricsAddr = envflag.String("TELEMETRY_ADDR", ":9128", "Host:Port for ceph exporter's metrics endpoint")
		metricsPath = envflag.String("TELEMETRY_PATH", "/metrics", "URL path for surfacing metrics to Prometheus")
		cephConfig  = envflag.String("CEPH_CONFIG", defaultCephConfigPath, "Path to Ceph config file")
		cephUser    = envflag.String("CEPH_USER", defaultCephUser, "Ceph user to connect to cluster")
	)

	envflag.Parse()
	conn, err := rados.NewConnWithUser(*cephUser)
	if err != nil {
		log.Fatalf("failed to create rados connection: %v", err)
	}
	err = conn.ReadConfigFile(*cephConfig)
	if err != nil {
		log.Fatalf("failed to read config file: %s", err)
	}

	err = conn.ReadDefaultConfigFile()
	if err != nil {
		log.Fatalf("failed to read config file: %v", err)
	}

	err = conn.Connect()
	if err != nil {
		log.Fatalf("failed to connect to the cluster: %v", err)
	}
	defer conn.Shutdown()
	fmt.Println("Successfully connected to Ceph cluster!")

	info, err := cephfs.CreateFromRados(conn)
	if err != nil {
		log.Fatalf("unable to create cephfs mountinfo: %v", err)
	}

	if err := info.Mount(); err != nil {
		log.Fatalf("unable to mount: %v", err)
	}
	defer info.Unmount()

	dm := NewDynamicMetrics()

	http.HandleFunc(*metricsPath, func(w http.ResponseWriter, r *http.Request) {
		dm.UpdateMetrics(info)
		promhttp.Handler().ServeHTTP(w, r)
	})

	fmt.Printf("Starting server on %s\n", *metricsAddr)
	log.Fatal(http.ListenAndServe(*metricsAddr, nil))
}

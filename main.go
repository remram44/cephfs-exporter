package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ceph/go-ceph/cephfs"
	rados "github.com/ceph/go-ceph/rados"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	entries := LoadDirsConfig("paths.json")
	for _, entry := range entries {
		fmt.Printf("Organisation: %s, User: %s, Path: %s\n", entry.Organisation, entry.User, entry.Path)
	}
	for _, entry := range entries {
		attrs, err := info.ListXattr(entry.Path)
		if err != nil {
			log.Printf("unable to get list of xattr for %q: %v\n", entry.Path, err)
			continue
		}
		fields := make(map[string]interface{}, len(attrs))

		for _, attr := range attrs {
			b, err := info.GetXattr(entry.Path, attr)
			if err != nil {
				continue
			}

			f, err := strconv.ParseFloat(string(b), 64)
			fmt.Println("path:", entry.Path, "attr:", attr, "value:", f)
			if err != nil {
				log.Printf("unable to convert %q to float: %v", b, err)
			}
			fields[attr] = f
			name := fmt.Sprintf("cephfs_xattr_%s", strings.ReplaceAll(attr, ".", "_"))
			gauge := dm.AddGauge(name, "A dynamically generated metric")
			gauge.With(entry.Tags()).Set(f)
		}
	}
}

func main() {
	conn, err := rados.NewConn()
	if err != nil {
		log.Fatalf("failed to create connection: %v", err)
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

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		for {
			dm.UpdateMetrics(info)
			time.Sleep(5 * time.Second)
		}
	}()

	fmt.Println("Starting server on :2112")
	log.Fatal(http.ListenAndServe(":2112", nil))
}

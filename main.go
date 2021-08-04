package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/yasker/kstat/pkg/version"
)

type OverviewType string
type Overview map[OverviewType]int

const (
	OverviewTypeTotal   = OverviewType("total")
	OverviewTypeAverage = OverviewType("avg")
)

type CPUMode string

const (
	CPUModeUser   = CPUMode("user")
	CPUModeSystem = CPUMode("system")
	CPUModeIdle   = CPUMode("idle")
	CPUModeWait   = CPUMode("iowait")
	CPUModeSteal  = CPUMode("steal")
)

type CPUIndex string

type NodeCPU map[CPUMode]NodeCPUMode
type NodeCPUMode map[CPUIndex]int
type OverviewNodeCPU map[CPUMode]Overview

type DiskIOType string

const (
	DiskIOTypeRead  = DiskIOType("read")
	DiskIOTypeWrite = DiskIOType("write")
)

type DiskDevice string
type NodeDisk map[DiskIOType]NodeDiskIO
type NodeDiskIO map[DiskDevice]int
type OverviewNodeDisk map[DiskIOType]Overview

type NetworkIOType string

const (
	NetworkIOTypeReceive  = NetworkIOType("receive")
	NetworkIOTypeTransmit = NetworkIOType("transmit")
)

type NetworkDevice string
type NodeNetwork map[NetworkIOType]NodeNetworkIO
type NodeNetworkIO map[NetworkDevice]int
type OverviewNodeNetwork map[NetworkIOType]Overview

const (
	SampleInterval = "10s"
)

func main() {
	app := cli.NewApp()
	app.Name = "kstat"
	app.Version = version.FriendlyVersion()
	app.Usage = "dstat for Kubernetes"
	app.Flags = []cli.Flag{}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) {
	if err := startServer(c); err != nil {
		logrus.Fatalf("Error starting server: %v", err)
	}
}

func query(client promv1.API, ctx context.Context, queryString string) (model.Vector, error) {
	result, warnings, err := client.Query(ctx, queryString, time.Now())
	if err != nil {
		return nil, fmt.Errorf("Error querying Prometheus: %v", err)
	}

	if len(warnings) > 0 {
		logrus.Warnf("Warnings: %v", warnings)
	}

	if result.Type() != model.ValVector {
		return nil, fmt.Errorf("Didn't get expected vector output, get %v instead", result.Type())
	}
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("BUG: output indicated as vector but failed to convert: %+v", result)
	}
	return vector, nil
}

func getCPUMetrics(client promv1.API, ctx context.Context, interval string) (report map[string]NodeCPU, overview map[string]OverviewNodeCPU) {
	report = map[string]NodeCPU{}
	overview = map[string]OverviewNodeCPU{}

	vector, err := query(client, ctx, fmt.Sprintf("rate(node_cpu_seconds_total{job=\"node-exporter\"}[%s])", interval))
	if err != nil {
		logrus.Errorf("failed to get CPU metrics: %v", err)
		return
	}

	for _, s := range vector {
		inst := string(s.Metric["instance"])
		cpu := CPUIndex(s.Metric["cpu"])
		mode := CPUMode(s.Metric["mode"])
		if report[inst] == nil {
			report[inst] = map[CPUMode]NodeCPUMode{}
		}
		if report[inst][mode] == nil {
			report[inst][mode] = map[CPUIndex]int{}
		}
		report[inst][mode][cpu] = int(s.Value * 100)
	}

	for inst := range report {
		overview[inst] = map[CPUMode]Overview{}
		for mode := range report[inst] {
			overview[inst][mode] = Overview{}
			for cpu := range report[inst][mode] {
				overview[inst][mode][OverviewTypeTotal] += report[inst][mode][cpu]
			}
			overview[inst][mode][OverviewTypeAverage] = overview[inst][mode][OverviewTypeTotal] / len(report[inst][mode])
		}
	}

	for inst := range overview {
		for mode := range overview[inst] {
			fmt.Printf("instance %s mode %s: total %d%%, average %d%%\n", inst, mode,
				overview[inst][mode][OverviewTypeTotal], overview[inst][mode][OverviewTypeAverage])
		}
	}
	return
}

func getDiskMetrics(client promv1.API, ctx context.Context, interval string) (report map[string]NodeDisk, overview map[string]OverviewNodeDisk) {
	report = map[string]NodeDisk{}
	overview = map[string]OverviewNodeDisk{}

	vector, err := query(client, ctx, fmt.Sprintf("rate(node_disk_read_bytes_total{job=\"node-exporter\"}[%s])", interval))
	if err != nil {
		logrus.Errorf("failed to get disk read metrics: %v", err)
		return
	}

	for _, s := range vector {
		inst := string(s.Metric["instance"])
		dev := DiskDevice(s.Metric["device"])
		if report[inst] == nil {
			report[inst] = map[DiskIOType]NodeDiskIO{}
		}
		if report[inst][DiskIOTypeRead] == nil {
			report[inst][DiskIOTypeRead] = map[DiskDevice]int{}
		}
		report[inst][DiskIOTypeRead][dev] = int(s.Value)
	}

	vector, err = query(client, ctx, fmt.Sprintf("rate(node_disk_written_bytes_total{job=\"node-exporter\"}[%s])", interval))
	if err != nil {
		logrus.Errorf("failed to get disk write metrics: %v", err)
		return
	}

	for _, s := range vector {
		inst := string(s.Metric["instance"])
		dev := DiskDevice(s.Metric["device"])
		if report[inst] == nil {
			report[inst] = map[DiskIOType]NodeDiskIO{}
		}
		if report[inst][DiskIOTypeWrite] == nil {
			report[inst][DiskIOTypeWrite] = map[DiskDevice]int{}
		}
		report[inst][DiskIOTypeWrite][dev] = int(s.Value)
	}

	for inst := range report {
		overview[inst] = map[DiskIOType]Overview{}
		for t := range report[inst] {
			overview[inst][t] = Overview{}
			for dev := range report[inst][t] {
				overview[inst][t][OverviewTypeTotal] += report[inst][t][dev]
			}
			// Doesn't make sense to calculate the average for disk throughput, skip it
		}
	}
	for inst := range overview {
		for t := range overview[inst] {
			fmt.Printf("instance %s disk IO type %s: total %d bytes/sec\n", inst, t,
				overview[inst][t][OverviewTypeTotal])
		}
	}
	return
}

func startServer(c *cli.Context) error {
	flag.Parse()
	logrus.Info("kstat starts")

	server := "http://localhost:9090"
	client, err := promapi.NewClient(promapi.Config{
		Address: server,
	})
	if err != nil {
		logrus.Errorf("Error connecting to %s: %v", server, err)
		return err
	}

	api := promv1.NewAPI(client)
	pollInterval := 5 * time.Second

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		getCPUMetrics(api, ctx, SampleInterval)
		getDiskMetrics(api, ctx, SampleInterval)

		time.Sleep(pollInterval)
	}
	return nil
}

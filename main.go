package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/yasker/kstat/pkg/version"
)

type CPUMode string

const (
	CPUModeUser   = CPUMode("user")
	CPUModeSystem = CPUMode("system")
	CPUModeIdle   = CPUMode("idle")
	CPUModeWait   = CPUMode("iowait")
	CPUModeSteal  = CPUMode("steal")
)

// ClusterMetric use the instance name as the key
type ClusterMetric map[string]*InstanceMetric

type InstanceMetric struct {
	// DeviceMetrics use the device name as the key
	DeviceMetrics map[string]int64
	Total         int64
	Average       int64

	// Value stores the metric value if there is no associated device
	Value int64
}

type MetricType string

const (
	MetricTypeDiskRead        = MetricType("disk-read")
	MetricTypeDiskWrite       = MetricType("disk-write")
	MetricTypeNetworkReceive  = MetricType("network-receive")
	MetricTypeNetworkTransmit = MetricType("network-transmit")
	MetricTypeCPUUser         = MetricType("cpu-user")
	MetricTypeCPUSystem       = MetricType("cpu-system")
	MetricTypeCPUIdle         = MetricType("cpu-idle")
	MetricTypeCPUWait         = MetricType("cpu-wait")
	MetricTypeCPUSteal        = MetricType("cpu-steal")
	MetricTypeMemAvailable    = MetricType("mem-avail")
)

const (
	InstanceLabel = model.LabelName("instance")
)

type MetricConfig struct {
	Type        MetricType
	DeviceLabel model.LabelName
	QueryString string
	Scale       float64
}

const (
	SampleInterval = "10s"
)

var (
	MetricConfigMap = map[MetricType]*MetricConfig{
		MetricTypeDiskRead: {
			Type:        MetricTypeDiskRead,
			QueryString: fmt.Sprintf("rate(node_disk_read_bytes_total{job=\"node-exporter\"}[%s])", SampleInterval),
			DeviceLabel: model.LabelName("device"),
			Scale:       1,
		},
		MetricTypeDiskWrite: {
			Type:        MetricTypeDiskWrite,
			QueryString: fmt.Sprintf("rate(node_disk_written_bytes_total{job=\"node-exporter\"}[%s])", SampleInterval),
			DeviceLabel: model.LabelName("device"),
			Scale:       1,
		},
		MetricTypeNetworkReceive: {
			Type:        MetricTypeNetworkReceive,
			QueryString: fmt.Sprintf("rate(node_network_receive_bytes_total{job=\"node-exporter\"}[%s])", SampleInterval),
			DeviceLabel: model.LabelName("device"),
			Scale:       1,
		},
		MetricTypeNetworkTransmit: {
			Type:        MetricTypeNetworkTransmit,
			QueryString: fmt.Sprintf("rate(node_network_transmit_bytes_total{job=\"node-exporter\"}[%s])", SampleInterval),
			DeviceLabel: model.LabelName("device"),
			Scale:       1,
		},
		MetricTypeCPUUser: {
			Type:        MetricTypeCPUUser,
			QueryString: fmt.Sprintf("rate(node_cpu_seconds_total{job=\"node-exporter\", mode=\"%s\"}[%s])", CPUModeUser, SampleInterval),
			DeviceLabel: model.LabelName("cpu"),
			Scale:       100,
		},
		MetricTypeCPUSystem: {
			Type:        MetricTypeCPUSystem,
			QueryString: fmt.Sprintf("rate(node_cpu_seconds_total{job=\"node-exporter\", mode=\"%s\"}[%s])", CPUModeSystem, SampleInterval),
			DeviceLabel: model.LabelName("cpu"),
			Scale:       100,
		},
		MetricTypeCPUIdle: {
			Type:        MetricTypeCPUIdle,
			QueryString: fmt.Sprintf("rate(node_cpu_seconds_total{job=\"node-exporter\", mode=\"%s\"}[%s])", CPUModeIdle, SampleInterval),
			DeviceLabel: model.LabelName("cpu"),
			Scale:       100,
		},
		MetricTypeCPUWait: {
			Type:        MetricTypeCPUWait,
			QueryString: fmt.Sprintf("rate(node_cpu_seconds_total{job=\"node-exporter\", mode=\"%s\"}[%s])", CPUModeWait, SampleInterval),
			DeviceLabel: model.LabelName("cpu"),
			Scale:       100,
		},
		MetricTypeCPUSteal: {
			Type:        MetricTypeCPUSteal,
			QueryString: fmt.Sprintf("rate(node_cpu_seconds_total{job=\"node-exporter\", mode=\"%s\"}[%s])", CPUModeSteal, SampleInterval),
			DeviceLabel: model.LabelName("cpu"),
			Scale:       100,
		},
		MetricTypeMemAvailable: {
			Type:        MetricTypeMemAvailable,
			QueryString: fmt.Sprintf("node_memory_MemAvailable_bytes{job=\"node-exporter\"}"),
			DeviceLabel: "",
			Scale:       1,
		},
	}
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

func getClusterMetric(client promv1.API, ctx context.Context, cfg *MetricConfig) (*ClusterMetric, error) {
	vector, err := query(client, ctx, cfg.QueryString)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get metric for %v", cfg.Type)
	}

	report := ClusterMetric{}
	for _, s := range vector {
		inst := string(s.Metric[InstanceLabel])
		dev := ""
		if report[inst] == nil {
			report[inst] = &InstanceMetric{}
		}
		if cfg.DeviceLabel != "" {
			dev = string(s.Metric[cfg.DeviceLabel])
			if report[inst].DeviceMetrics == nil {
				report[inst].DeviceMetrics = map[string]int64{}
			}
			report[inst].DeviceMetrics[dev] = int64(float64(s.Value) * cfg.Scale)
		} else {
			report[inst].Value = int64(float64(s.Value) * cfg.Scale)
		}
	}
	for _, m := range report {
		devCount := int64(len(m.DeviceMetrics))
		if devCount != 0 {
			for _, v := range m.DeviceMetrics {
				m.Total += v
			}
			m.Average = m.Total / devCount
		}
	}
	return &report, nil
}

func startServer(c *cli.Context) error {
	flag.Parse()

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

		metrics := map[MetricType]*ClusterMetric{}

		for k, c := range MetricConfigMap {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cm, err := getClusterMetric(api, ctx, c)
			if err != nil {
				logrus.Errorf("failed to get metric for %v: %v", k, err)
				continue
			}
			metrics[k] = cm
		}

		printMetrics(metrics)

		time.Sleep(pollInterval)
	}
	return nil
}

func printMetrics(metrics map[MetricType]*ClusterMetric) {
	instanceList := []string{}

	// choose a random one to get the instance list
	for inst := range *metrics[MetricTypeCPUIdle] {
		instanceList = append(instanceList, inst)
	}
	sort.Strings(instanceList)

	header := fmt.Sprintf("%20s : %24s | %7s | %15s | %15s\n",
		"", "----------cpu----------", "--mem--", "-----disk-----", "---network---")
	subheader := fmt.Sprintf("%20s : %4s %4s %4s %4s %4s | %7s | %7s %7s | %7s %7s\n",
		"instance", "usr", "sys", "idl", "wai", "stl", "avail", "read", "write", "recv", "send")
	output := ""
	for _, inst := range instanceList {
		output += fmt.Sprintf("%20s : %3d%% %3d%% %3d%% %3d%% %3d%% | %7s | %7s %7s | %7s %7s\n",
			inst,
			(*metrics[MetricTypeCPUUser])[inst].Average,
			(*metrics[MetricTypeCPUSystem])[inst].Average,
			(*metrics[MetricTypeCPUIdle])[inst].Average,
			(*metrics[MetricTypeCPUWait])[inst].Average,
			(*metrics[MetricTypeCPUSteal])[inst].Average,
			bytefmt.ByteSize(uint64((*metrics[MetricTypeMemAvailable])[inst].Value)),
			bytefmt.ByteSize(uint64((*metrics[MetricTypeDiskRead])[inst].Total)),
			bytefmt.ByteSize(uint64((*metrics[MetricTypeDiskWrite])[inst].Total)),
			bytefmt.ByteSize(uint64((*metrics[MetricTypeNetworkReceive])[inst].Total)),
			bytefmt.ByteSize(uint64((*metrics[MetricTypeNetworkTransmit])[inst].Total)))
	}

	fmt.Print(header)
	fmt.Print(subheader)
	fmt.Print(output)
}

package main

import (
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

const (
	FlagPrometheusServer = "prometheus-server"
	FlagServiceAccount   = "service-account"
)

func ServerCmd() cli.Command {
	return cli.Command{
		Name: "server",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  FlagPrometheusServer,
				Usage: "Specify the Prometheus Server address",
				Value: "http://localhost:9090",
			},
		},
		Action: func(c *cli.Context) {
			if err := startServer(c); err != nil {
				logrus.Fatalf("Error starting kstat server: %v", err)
			}
		},
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "kstat"
	app.Version = version.FriendlyVersion()
	app.Usage = "dstat for Kubernetes"
	app.Flags = []cli.Flag{}
	app.Commands = []cli.Command{
		ServerCmd(),
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func startServer(c *cli.Context) error {
	flag.Parse()

	server := c.String(FlagPrometheusServer)
	client, err := promapi.NewClient(promapi.Config{
		Address: server,
	})
	if err != nil {
		logrus.Errorf("Error start client to %s: %v", server, err)
		return err
	}

	api := promv1.NewAPI(client)
	pollInterval := 5 * time.Second

	if err := testConnection(api); err != nil {
		logrus.Errorf("Error connecting to %s: %v", server, err)
		return err

	}

	lineCounter := new(int)
	*lineCounter = 0
	for {
		metrics, err := getMetrics(api)
		if err != nil {
			logrus.Errorf("failed to complete metrics retrieval: %v", err)
			*lineCounter++
		} else {
			printMetrics(metrics, lineCounter)
		}

		time.Sleep(pollInterval)
	}
	return nil
}

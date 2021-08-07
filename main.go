package main

import (
	"flag"
	"os"
	"text/template"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/yasker/kstat/pkg/version"
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

const (
	InstanceLabel = model.LabelName("instance")
)

const (
	ValueTypeCPU  = "cpu"
	ValueTypeSize = "size"

	ValueTypeCPUFormat  = "%5s"
	ValueTypeSizeFormat = "%8s"
)

type MetricConfig struct {
	Name        string  `yaml:"name"`
	DeviceLabel string  `yaml:"device_label"`
	QueryString string  `yaml:"query_string"`
	Scale       float64 `yaml:"scale"`
	ValueType   string  `yaml:"value_type"`
	Shorthand   string  `yaml:"shorthand"`
}

const (
	SampleInterval = "10s"
)

const (
	FlagPrometheusServer   = "prometheus-server"
	FlagMetricConfigFile   = "metrics-config"
	FlagHeaderTemplateFile = "header-template"
	FlagOutputTemplateFile = "output-template"
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
			cli.StringFlag{
				Name:  FlagMetricConfigFile,
				Usage: "Specify the metric config yaml",
				Value: "metrics.yaml",
			},
			cli.StringFlag{
				Name:  FlagHeaderTemplateFile,
				Usage: "Specify the header template file",
				Value: "header.tmpl",
			},
			cli.StringFlag{
				Name:  FlagOutputTemplateFile,
				Usage: "Specify the output template file",
				Value: "output.tmpl",
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
		return errors.Wrapf(err, "cannot start client for %s", server)
	}

	api := promv1.NewAPI(client)
	pollInterval := 5 * time.Second

	if err := testConnection(api); err != nil {
		return errors.Wrapf(err, "cannot connecting to %s", server)

	}

	cfgFile := c.String(FlagMetricConfigFile)

	f, err := os.Open(cfgFile)
	if err != nil {
		return errors.Wrapf(err, "cannot open the metrics config file %v", cfgFile)
	}
	defer f.Close()

	metricConfigs := []*MetricConfig{}

	if err := yaml.NewDecoder(f).Decode(&metricConfigs); err != nil {
		return errors.Wrapf(err, "cannot decode the metrics config file %v", cfgFile)
	}
	metricConfigMap := map[string]*MetricConfig{}
	for _, m := range metricConfigs {
		metricConfigMap[m.Name] = m
	}

	headerTmplFile := c.String(FlagHeaderTemplateFile)
	headerTmpl, err := template.ParseFiles(headerTmplFile)
	if err != nil {
		return errors.Wrapf(err, "cannot read or parse the header template file %v", headerTmplFile)
	}

	outputTmplFile := c.String(FlagOutputTemplateFile)
	outputTmpl, err := template.ParseFiles(outputTmplFile)
	if err != nil {
		return errors.Wrapf(err, "cannot read or parse the output template file %v", outputTmplFile)
	}

	lineCounter := new(int)
	*lineCounter = 0
	for {
		metrics, err := getMetrics(api, metricConfigMap)
		if err != nil {
			logrus.Errorf("failed to complete metrics retrieval: %v", err)
			*lineCounter++
		} else {
			printMetrics(metrics, metricConfigMap, headerTmpl, outputTmpl, lineCounter)
		}

		time.Sleep(pollInterval)
	}
	return nil
}

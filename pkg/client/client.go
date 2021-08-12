package client

import (
	"fmt"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"

	pb "github.com/yasker/kstat/pkg/pb/v1"
	"github.com/yasker/kstat/pkg/types"
)

var (
	ConfigCheckedAt time.Time
)

type MetricFormat struct {
	Name      string `yaml:"name"`
	ValueType string `yaml:"value_type"`
	Shorthand string `yaml:"shorthand"`
}

type Client struct {
	ServerAddress      string
	MetricFormatFile   string
	HeaderTemplateFile string
	OutputTemplateFile string
	ShowDevices        bool
	ShowAsTop          bool

	rwMutex         *sync.RWMutex
	metricFormatMap map[string]*MetricFormat
	headerTemplate  *template.Template
	outputTemplate  *template.Template
}

func NewClient(serverAddr, metricFormatFile, headerTmplFile, outputTmplFile string) *Client {
	return &Client{
		ServerAddress:      serverAddr,
		MetricFormatFile:   metricFormatFile,
		HeaderTemplateFile: headerTmplFile,
		OutputTemplateFile: outputTmplFile,

		rwMutex: &sync.RWMutex{},
	}
}

func (c *Client) Start() error {
	lineCounter := new(int)
	*lineCounter = 0

	for {
		if time.Now().After(ConfigCheckedAt.Add(types.ConfigCheckInterval)) {
			if err := c.reloadTemplateFiles(); err != nil {
				logrus.Errorf("failed to reload the configuration files: %v", err)
			}
			ConfigCheckedAt = time.Now()
		}

		metrics, err := c.GetMetrics()
		if err != nil {
			logrus.Errorf("Failed to get metrics from server %v", err)
		} else {
			if !c.ShowAsTop {
				c.printMetrics(metrics, lineCounter)
			} else {
				fmt.Print("\033[H\033[2J")
				c.printTop(metrics)
			}
		}

		time.Sleep(types.PollInterval)
	}
	return nil
}

func (c *Client) reloadTemplateFiles() error {
	var err error

	f, err := os.Open(c.MetricFormatFile)
	if err != nil {
		return errors.Wrapf(err, "cannot open the metrics format config file %v", c.MetricFormatFile)
	}
	defer f.Close()

	cfgs := []*MetricFormat{}

	if err := yaml.NewDecoder(f).Decode(&cfgs); err != nil {
		return errors.Wrapf(err, "cannot decode the metrics format config file %v", c.MetricFormatFile)
	}

	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	c.metricFormatMap = map[string]*MetricFormat{}
	for _, m := range cfgs {
		c.metricFormatMap[m.Name] = m
	}

	c.headerTemplate, err = template.ParseFiles(c.HeaderTemplateFile)
	if err != nil {
		return errors.Wrapf(err, "cannot read or parse the header template file %v", c.HeaderTemplateFile)
	}

	c.outputTemplate, err = template.ParseFiles(c.OutputTemplateFile)
	if err != nil {
		return errors.Wrapf(err, "cannot read or parse the output template file %v", c.OutputTemplateFile)
	}
	return nil
}

func (c *Client) GetMetrics() (map[string]*types.ClusterMetric, error) {
	conn, err := grpc.Dial(c.ServerAddress, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrapf(err, "cannot connect to metric server %v", c.ServerAddress)
	}
	defer conn.Close()
	metricsServiceClient := pb.NewMetricsServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), types.GRPCServiceTimeout)
	defer cancel()

	resp, err := metricsServiceClient.GetMetrics(ctx, &pb.GetMetricsRequest{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get metrics from %v", c.ServerAddress)
	}

	return PBToMetrics(resp), nil
}

func PBToMetrics(resp *pb.GetMetricsResponse) map[string]*types.ClusterMetric {
	result := map[string]*types.ClusterMetric{}
	for k, v := range resp.ClusterMetrics {
		cm := &types.ClusterMetric{
			InstanceMetrics: map[string]*types.InstanceMetric{},
		}
		for ki, vi := range v.InstanceMetrics {
			im := &types.InstanceMetric{
				DeviceMetrics: map[string]int64{},
				Total:         vi.Total,
				Average:       vi.Average,
				Value:         vi.Value,
			}
			for kd, kv := range vi.DeviceMetrics {
				im.DeviceMetrics[kd] = kv
			}
			cm.InstanceMetrics[ki] = im
		}
		result[k] = cm
	}

	return result
}

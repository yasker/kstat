package server

import (
	"net"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/yasker/kstat/pkg/types"
)

type MetricConfig struct {
	Name         string  `yaml:"name"`
	DeviceLabel  string  `yaml:"device_label"`
	DevicePrefix string  `yaml:"device_prefix"`
	QueryString  string  `yaml:"query_string"`
	Scale        float64 `yaml:"scale"`
}

type Server struct {
	ListenAddr       string
	PrometheusServer string
	ConfigFile       string

	rwMutex         *sync.RWMutex
	metricConfigMap map[string]*MetricConfig
	promClient      promv1.API
	shutdownWG      sync.WaitGroup
	metrics         map[string]*types.ClusterMetric

	grpcServer *grpc.Server
}

var (
	ConfigCheckedAt time.Time
)

func NewServer(listenAddr, promServer, cfgFile string) *Server {
	return &Server{
		ListenAddr:       listenAddr,
		PrometheusServer: promServer,
		ConfigFile:       cfgFile,

		rwMutex: &sync.RWMutex{},
	}
}

func (s *Server) startGRPCServer() error {
	s.shutdownWG.Add(1)
	go func() {
		defer s.shutdownWG.Done()

		grpcAddress := s.ListenAddr
		listener, err := net.Listen("tcp", grpcAddress)
		if err != nil {
			logrus.Errorf("Failed to listen %v: %v", grpcAddress, err)
			return
		}

		logrus.Infof("Listening on gRPC Controller server: %v", grpcAddress)
		err = s.grpcServer.Serve(listener)
		logrus.Errorf("Server at %v is down: %v", grpcAddress, err)
		return
	}()
	return nil
}

func (s *Server) Start() error {
	client, err := promapi.NewClient(promapi.Config{
		Address: s.PrometheusServer,
	})
	if err != nil {
		return errors.Wrapf(err, "cannot start client for %s", s.PrometheusServer)
	}

	s.promClient = promv1.NewAPI(client)
	if err := s.testConnection(); err != nil {
		return errors.Wrapf(err, "cannot connecting to %s", s.PrometheusServer)
	}

	s.grpcServer = NewGRPCServer(s)
	s.startGRPCServer()

	for {
		if time.Now().After(ConfigCheckedAt.Add(types.ConfigCheckInterval)) {
			if err := s.reloadMetricConfigMap(); err != nil {
				logrus.Errorf("failed to reload the configuration files: %v", err)
			}
			ConfigCheckedAt = time.Now()
		}

		metrics, err := s.getMetrics()
		if err != nil {
			logrus.Errorf("failed to complete metrics retrieval: %v", err)
		}
		s.refreshMetrics(metrics)

		time.Sleep(types.PollInterval)
	}
	return nil
}

func (s *Server) reloadMetricConfigMap() error {
	var err error

	f, err := os.Open(s.ConfigFile)
	if err != nil {
		return errors.Wrapf(err, "cannot open the metrics config file %v", s.ConfigFile)
	}
	defer f.Close()

	metricConfigs := []*MetricConfig{}

	if err := yaml.NewDecoder(f).Decode(&metricConfigs); err != nil {
		return errors.Wrapf(err, "cannot decode the metrics config file %v", s.ConfigFile)
	}

	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()

	s.metricConfigMap = map[string]*MetricConfig{}
	for _, m := range metricConfigs {
		s.metricConfigMap[m.Name] = m
	}

	return nil
}

func (s *Server) refreshMetrics(metrics map[string]*types.ClusterMetric) {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()
	s.metrics = metrics
}

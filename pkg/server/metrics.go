package server

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/prometheus/common/model"

	"github.com/yasker/kstat/pkg/types"
)

func (s *Server) query(ctx context.Context, queryString string) (model.Vector, error) {
	result, warnings, err := s.promClient.Query(ctx, queryString, time.Now())
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

func (s *Server) getClusterMetric(ctx context.Context, cfg *MetricConfig) (*types.ClusterMetric, error) {
	vector, err := s.query(ctx, cfg.QueryString)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get metric for %v", cfg.Name)
	}

	report := types.ClusterMetric{
		InstanceMetrics: map[string]*types.InstanceMetric{},
	}
	for _, smp := range vector {
		inst := string(smp.Metric[types.InstanceLabel])
		dev := ""
		if report.InstanceMetrics[inst] == nil {
			report.InstanceMetrics[inst] = &types.InstanceMetric{}
		}
		if cfg.DeviceLabel != "" {
			dev = cfg.DevicePrefix + ": " + string(smp.Metric[model.LabelName(cfg.DeviceLabel)])
			if report.InstanceMetrics[inst].DeviceMetrics == nil {
				report.InstanceMetrics[inst].DeviceMetrics = map[string]int64{}
			}
			report.InstanceMetrics[inst].DeviceMetrics[dev] = int64(float64(smp.Value) * cfg.Scale)
		} else {
			report.InstanceMetrics[inst].Value = int64(float64(smp.Value) * cfg.Scale)
		}
	}
	for _, m := range report.InstanceMetrics {
		devCount := int64(len(m.DeviceMetrics))
		if devCount != 0 {
			for _, v := range m.DeviceMetrics {
				m.Total += v
			}
			m.Average = m.Total / devCount
		} else {
			m.Total = m.Value
		}
	}
	return &report, nil
}

func (s *Server) testConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := s.query(ctx, "up")
	return err
}

func (s *Server) getMetrics() (map[string]*types.ClusterMetric, error) {
	metrics := map[string]*types.ClusterMetric{}

	for _, c := range s.metricConfigMap {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cm, err := s.getClusterMetric(ctx, c)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get metric for %v", c.Name)
		}
		metrics[c.Name] = cm
	}
	return metrics, nil
}

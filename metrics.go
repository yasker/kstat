package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

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
		return nil, errors.Wrapf(err, "failed to get metric for %v", cfg.Name)
	}

	report := ClusterMetric{}
	for _, s := range vector {
		inst := string(s.Metric[InstanceLabel])
		dev := ""
		if report[inst] == nil {
			report[inst] = &InstanceMetric{}
		}
		if cfg.DeviceLabel != "" {
			dev = cfg.DevicePrefix + ": " + string(s.Metric[model.LabelName(cfg.DeviceLabel)])
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
		} else {
			m.Total = m.Value
		}
	}
	return &report, nil
}

func testConnection(client promv1.API) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := query(client, ctx, "up")
	return err
}

func getMetrics(client promv1.API, cfgs map[string]*MetricConfig) (map[string]*ClusterMetric, error) {
	metrics := map[string]*ClusterMetric{}

	for _, c := range cfgs {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cm, err := getClusterMetric(client, ctx, c)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get metric for %v", c.Name)
		}
		metrics[c.Name] = cm
	}
	return metrics, nil
}

package types

import (
	"time"

	"github.com/prometheus/common/model"
)

var (
	ConfigCheckInterval = 30 * time.Second
	PollInterval        = 5 * time.Second
	GRPCServiceTimeout  = 10 * time.Second
)

const (
	SampleInterval = "10s"
)

const (
	InstanceLabel = model.LabelName("instance")
)

// ClusterMetric use the instance name as the key
type ClusterMetric struct {
	InstanceMetrics map[string]*InstanceMetric
}

type InstanceMetric struct {
	// DeviceMetrics use the device name as the key
	DeviceMetrics map[string]int64
	Total         int64
	Average       int64

	// Value stores the metric value if there is no associated device
	Value int64
}

const (
	ValueTypeCPU  = "cpu"
	ValueTypeSize = "size"

	ValueTypeCPUFormat  = "%5s"
	ValueTypeSizeFormat = "%8s"
)

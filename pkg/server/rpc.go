package server

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/yasker/kstat/pkg/pb/v1"
	"github.com/yasker/kstat/pkg/types"
)

func NewGRPCServer(s *Server) *grpc.Server {
	grpcServer := grpc.NewServer()

	pb.RegisterMetricsServiceServer(grpcServer, s)

	reflection.Register(grpcServer)

	return grpcServer
}

func (s *Server) Watch(req *pb.WatchRequest, srv pb.MetricsService_WatchServer) error {
	logrus.Errorf("DEBUGGG: Get Watch")
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}

func (s *Server) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	s.rwMutex.RLock()
	defer s.rwMutex.RUnlock()

	return MetricsToPB(s.metrics), nil
}

func MetricsToPB(metrics map[string]*types.ClusterMetric) *pb.GetMetricsResponse {
	resp := &pb.GetMetricsResponse{}
	resp.ClusterMetrics = map[string]*pb.ClusterMetric{}
	for k, v := range metrics {
		cm := &pb.ClusterMetric{
			InstanceMetrics: map[string]*pb.InstanceMetric{},
		}
		for ki, vi := range v.InstanceMetrics {
			im := &pb.InstanceMetric{
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
		resp.ClusterMetrics[k] = cm
	}

	return resp
}

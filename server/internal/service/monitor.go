package service

import (
	"context"
	"github.com/jinzhu/copier"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"time"
	pb "vgpu/api/v1"
	"vgpu/internal/biz"
	"vgpu/internal/data/prom"
)

type MonitorService struct {
	promClient  *prom.Client
	nodeUsecase *biz.NodeUsecase
	podUsecase  *biz.PodUseCase

	pb.UnimplementedMonitorServer
}

func NewMonitorService(
	promClient *prom.Client,
	nodeUsecase *biz.NodeUsecase,
	podUsecase *biz.PodUseCase,
) *MonitorService {
	return &MonitorService{
		promClient:  promClient,
		nodeUsecase: nodeUsecase,
		podUsecase:  podUsecase,
	}
}

func (s *MonitorService) QueryRange(ctx context.Context, req *pb.QueryRangeRequest) (*pb.RangeResponse, error) {
	startTime, err := time.ParseInLocation(time.DateTime, req.Range.GetStart(), time.Local)
	if err != nil {
		return nil, pb.ErrorTransformError(err.Error())
	}
	endTime, err := time.ParseInLocation(time.DateTime, req.Range.GetEnd(), time.Local)
	if err != nil {
		return nil, pb.ErrorTransformError(err.Error())
	}
	step, err := time.ParseDuration(req.Range.GetStep())
	if err != nil {
		return nil, pb.ErrorTransformError(err.Error())
	}
	value, err := s.promClient.QueryRange(ctx, req.GetQuery(), v1.Range{Start: startTime, End: endTime, Step: step})
	if err != nil {
		return nil, pb.ErrorVgpuDomainError(err.Error())
	}
	matrixValue, ok := value.(model.Matrix)
	if !ok {
		return nil, pb.ErrorTransformError("Error casting result to model.Matrix")
	}
	var res = &pb.RangeResponse{}
	copier.Copy(&res.Data, &matrixValue)

	for _, sample := range res.Data {
		sample.Values = fillLessSamplePoint(startTime, endTime, step, sample.Values)
	}
	return res, nil
}

func fillLessSamplePoint(startTime, endTime time.Time, step time.Duration, values []*pb.SamplePair) []*pb.SamplePair {
	existingPoints := make(map[int64]float32)
	for _, pair := range values {
		existingPoints[pair.Timestamp] = pair.Value
	}

	var filledValues []*pb.SamplePair
	currentTime := startTime
	for !currentTime.After(endTime) {
		currentTimestamp := currentTime.UnixMilli()
		if value, exists := existingPoints[currentTimestamp]; exists {
			filledValues = append(filledValues, &pb.SamplePair{Value: value, Timestamp: currentTimestamp})
		} else {
			filledValues = append(filledValues, &pb.SamplePair{Value: 0, Timestamp: currentTimestamp})
		}
		currentTime = currentTime.Add(step)
	}
	return filledValues
}

func (s *MonitorService) QueryInstant(ctx context.Context, req *pb.QueryInstantRequest) (*pb.InstantResponse, error) {
	value, err := s.promClient.Query(ctx, req.GetQuery())
	if err != nil {
		return nil, pb.ErrorVgpuDomainError(err.Error())
	}
	vectorValue, ok := value.(model.Vector)
	if !ok {
		return nil, pb.ErrorTransformError("Error casting result to model.Vector")
	}
	var res = &pb.InstantResponse{}
	copier.Copy(&res.Data, &vectorValue)
	return res, nil
}
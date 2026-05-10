package server

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	clusterv1 "hvc/api/pb/clusterv1"
	"hvc/internal/cluster"
	"hvc/internal/config"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	grpcinterceptor "hvc/internal/interfaces/grpc/interceptor"
	"hvc/internal/model"
	"hvc/pkg/logx"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// InternalGRPCServer 管理集群内部 gRPC 服务。
//
// 这一层属于 bootstrap 通信基础设施，节点启动后即建立监听，
// 不参与运行期 public gRPC 的热启停。
type InternalGRPCServer struct {
	cfg           config.InternalGRPCConfig
	stateCache    *cluster.StateCache
	progressStore *rediscache.ProgressStore
	segmentRepo   *mysql.SegmentRepository
	jobRepo       *mysql.JobRepository
	nodeRepo      *mysql.ClusterNodeRepository
}

// NewInternalGRPCServer 创建集群内部 gRPC 服务。
func NewInternalGRPCServer(cfg config.InternalGRPCConfig, stateCache *cluster.StateCache, progressStore *rediscache.ProgressStore, segmentRepo *mysql.SegmentRepository, jobRepo *mysql.JobRepository, nodeRepo *mysql.ClusterNodeRepository) *InternalGRPCServer {
	return &InternalGRPCServer{
		cfg:           cfg,
		stateCache:    stateCache,
		progressStore: progressStore,
		segmentRepo:   segmentRepo,
		jobRepo:       jobRepo,
		nodeRepo:      nodeRepo,
	}
}

// Start 启动集群内部 gRPC 服务。
func (s *InternalGRPCServer) Start(ctx context.Context) error {
	if !s.cfg.Enabled || s.cfg.ListenAddress == "" {
		return nil
	}

	listener, err := net.Listen("tcp", s.cfg.ListenAddress)
	if err != nil {
		return err
	}
	logx.Info("grpc.internal.listening", logx.Fields{
		"address": s.cfg.ListenAddress,
	})

	serverOpts := make([]grpc.ServerOption, 0, 3)
	if strings.TrimSpace(s.cfg.SharedToken) != "" {
		serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(
			grpcinterceptor.UnaryLogInterceptor(),
			internalGRPCTokenInterceptor(strings.TrimSpace(s.cfg.SharedToken)),
		))
	} else {
		serverOpts = append(serverOpts, grpc.UnaryInterceptor(grpcinterceptor.UnaryLogInterceptor()))
	}
	if s.cfg.MaxRecvMsgSizeMB > 0 {
		serverOpts = append(serverOpts, grpc.MaxRecvMsgSize(s.cfg.MaxRecvMsgSizeMB*1024*1024))
	}
	if s.cfg.MaxSendMsgSizeMB > 0 {
		serverOpts = append(serverOpts, grpc.MaxSendMsgSize(s.cfg.MaxSendMsgSizeMB*1024*1024))
	}

	grpcServer := grpc.NewServer(serverOpts...)
	clusterv1.RegisterClusterInternalServiceServer(grpcServer, &clusterInternalRPCServer{
		stateCache:    s.stateCache,
		progressStore: s.progressStore,
		segmentRepo:   s.segmentRepo,
		jobRepo:       s.jobRepo,
		nodeRepo:      s.nodeRepo,
	})

	go func() {
		<-ctx.Done()
		stopTimeout := s.cfg.GracefulStopTimeout
		if stopTimeout <= 0 {
			stopTimeout = 5 * time.Second
		}
		done := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(stopTimeout):
			grpcServer.Stop()
		}
		_ = listener.Close()
	}()

	if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

type clusterInternalRPCServer struct {
	clusterv1.UnimplementedClusterInternalServiceServer
	stateCache    *cluster.StateCache
	progressStore *rediscache.ProgressStore
	segmentRepo   *mysql.SegmentRepository
	jobRepo       *mysql.JobRepository
	nodeRepo      *mysql.ClusterNodeRepository
}

func internalGRPCTokenInterceptor(sharedToken string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if sharedToken == "" {
			return handler(ctx, req)
		}
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		values := md.Get("x-hvc-internal-token")
		if len(values) == 0 || strings.TrimSpace(values[0]) != sharedToken {
			return nil, status.Error(codes.Unauthenticated, "invalid internal token")
		}
		return handler(ctx, req)
	}
}

func (s *clusterInternalRPCServer) WorkerHeartbeat(ctx context.Context, req *clusterv1.WorkerHeartbeatRequest) (*clusterv1.WorkerHeartbeatResponse, error) {
	heartbeatAt := time.Now()
	if s.stateCache != nil {
		s.stateCache.SaveHeartbeat(ctx, model.WorkerHeartbeat{
			NodeID:             req.GetNodeId(),
			WorkerID:           req.GetWorkerId(),
			StartupInstanceID:  req.GetStartupInstanceId(),
			MachineFingerprint: req.GetMachineFingerprint(),
			Timestamp:          heartbeatAt,
		})
	}
	if s.nodeRepo != nil {
		_ = s.nodeRepo.TouchHeartbeat(ctx, req.GetNodeId(), heartbeatAt)
	}
	return &clusterv1.WorkerHeartbeatResponse{Accepted: true}, nil
}

func (s *clusterInternalRPCServer) ReportProgress(ctx context.Context, req *clusterv1.ReportProgressRequest) (*clusterv1.ReportProgressResponse, error) {
	snapshot := model.ProgressSnapshot{
		JobID:              req.GetJobId(),
		Status:             int(req.GetStatus()),
		Stage:              req.GetStage(),
		ProgressPermille:   int(req.GetProgressPermille()),
		CurrentFPS:         req.GetFps(),
		CurrentBitrateKbps: req.GetBitrateKbps(),
		CurrentSpeed:       req.GetSpeed(),
		UpdatedAt:          time.Now(),
	}
	if s.progressStore != nil {
		s.progressStore.Save(ctx, snapshot)
	}
	if s.jobRepo != nil {
		_ = s.jobRepo.UpdateProgress(ctx, snapshot.JobID, snapshot.ProgressPermille, snapshot.Stage)
	}
	return &clusterv1.ReportProgressResponse{Accepted: true}, nil
}

func (s *clusterInternalRPCServer) SegmentUploaded(ctx context.Context, req *clusterv1.SegmentUploadedRequest) (*clusterv1.SegmentUploadedResponse, error) {
	if s.segmentRepo == nil {
		return &clusterv1.SegmentUploadedResponse{Accepted: true}, nil
	}
	if err := s.segmentRepo.MarkUploaded(ctx, req.GetSegmentId(), req.GetObjectEtag(), req.GetObjectSizeBytes()); err != nil {
		return nil, err
	}
	return &clusterv1.SegmentUploadedResponse{Accepted: true}, nil
}

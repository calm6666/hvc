// Package interceptor 提供 gRPC 拦截器实现。
//
// 本文件实现 gRPC 一元调用日志拦截器，用于记录每个 gRPC 请求的
// 方法名、耗时、错误信息等，方便排查问题和性能分析。
//
// 使用方式：
//
//	s := grpc.NewServer(grpc.UnaryInterceptor(interceptor.UnaryLogInterceptor()))
package interceptor

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"hvc/pkg/logx"
)

// UnaryLogInterceptor 创建 gRPC 一元调用日志拦截器。
//
// 拦截器在每次 gRPC 调用前后记录日志：
//   - 调用前：记录方法名和开始时间
//   - 调用后：记录方法名、耗时和错误信息
func UnaryLogInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		method := info.FullMethod

		resp, err := handler(ctx, req)
		elapsed := time.Since(start)

		if err != nil {
			logx.Error("grpc.unary_call_failed", err, logx.Fields{
				"method":  method,
				"elapsed": elapsed.String(),
			})
		} else {
			logx.Info("grpc.unary_call", logx.Fields{
				"method":  method,
				"elapsed": elapsed.String(),
			})
		}

		return resp, err
	}
}

// StreamLogInterceptor 创建 gRPC 流式调用日志拦截器。
//
// 拦截器在流式调用开始和结束时记录日志。
func StreamLogInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		method := info.FullMethod

		err := handler(srv, ss)
		elapsed := time.Since(start)

		if err != nil {
			logx.Error("grpc.stream_call_failed", err, logx.Fields{
				"method":  method,
				"elapsed": elapsed.String(),
			})
		} else {
			logx.Info("grpc.stream_call", logx.Fields{
				"method":  method,
				"elapsed": elapsed.String(),
			})
		}

		return err
	}
}

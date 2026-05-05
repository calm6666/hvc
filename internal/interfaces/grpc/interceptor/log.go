package interceptor

import "context"

// UnaryLogInterceptor 作为 gRPC 一元拦截器占位实现。
func UnaryLogInterceptor(ctx context.Context) context.Context { return ctx }

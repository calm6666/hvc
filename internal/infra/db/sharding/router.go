package sharding

// RouteResult 表示分片路由结果。
type RouteResult struct {
	Shard int
}

// Router 表示分库分表路由器。
type Router struct {
	Enabled    bool
	ShardCount int
}

// NewRouter 创建分库分表路由器。
func NewRouter(enabled bool, shardCount int) *Router {
	return &Router{Enabled: enabled, ShardCount: shardCount}
}

// RouteByID 根据 ID 路由。
func (r *Router) RouteByID(id uint64) RouteResult {
	if !r.Enabled || r.ShardCount <= 0 {
		return RouteResult{Shard: 0}
	}
	return RouteResult{Shard: int(id % uint64(r.ShardCount))}
}

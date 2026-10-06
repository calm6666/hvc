package config

import "strings"

const (
	NodeModeStandalone      = "standalone"
	NodeModeClusterControl  = "cluster-control"
	NodeModeClusterWorker   = "cluster-worker"
	NodeModeClusterAllInOne = "cluster-allinone"
)

// EffectiveNodeMode 返回 bootstrap 明确定义的节点模式。
//
// 设计原则：
// 1. 节点角色属于部署拓扑语义，必须来自 bootstrap；
// 2. 运行期模块开关只能表达“当前是否启停某个模块”，不能反推出节点本来的角色；
// 3. 老配置未显式写 node_mode 时，兼容回退到 standalone。
func (cfg RuntimeConfig) EffectiveNodeMode() string {
	mode := strings.TrimSpace(cfg.Server.NodeMode)
	if mode == "" {
		return NodeModeStandalone
	}
	return mode
}

// IsStandalone 返回是否为单机模式。
func (cfg RuntimeConfig) IsStandalone() bool {
	return cfg.EffectiveNodeMode() == NodeModeStandalone
}

// IsClusterControl 返回是否为集群控制节点模式。
func (cfg RuntimeConfig) IsClusterControl() bool {
	return cfg.EffectiveNodeMode() == NodeModeClusterControl
}

// IsClusterWorker 返回是否为集群执行节点模式。
func (cfg RuntimeConfig) IsClusterWorker() bool {
	return cfg.EffectiveNodeMode() == NodeModeClusterWorker
}

// IsClusterAllInOne 返回是否为集群一体化模式。
func (cfg RuntimeConfig) IsClusterAllInOne() bool {
	return cfg.EffectiveNodeMode() == NodeModeClusterAllInOne
}

// ApplyNodeModeRuntimeConstraints 根据 bootstrap 节点角色收敛运行时模块边界。
//
// 设计原因：
// 1. runtime config 是全局共享快照，会被所有节点共同读取；
// 2. scheduler / worker / callback / public gRPC / MQ consumer / 北向 HTTP
//    是否允许在本节点运行，属于“节点角色边界”，不是全局热配置语义；
// 3. 因此每个节点在加载已发布 runtime config 后，都必须再按本地 node_mode 做一次最终收口；
// 4. internal_grpc 仍是 bootstrap 固定入口，不受这里影响。
func ApplyNodeModeRuntimeConstraints(nodeMode string, cfg DynamicRuntimeConfig) DynamicRuntimeConfig {
	switch strings.TrimSpace(nodeMode) {
	case NodeModeClusterControl:
		// 控制面节点承担调度，但是否开放北向能力仍由 runtime config 热更新控制。
		cfg.Mode.EnableScheduler = true
		cfg.Mode.EnableWorker = false
	case NodeModeClusterWorker:
		// 执行节点只保留 worker，避免共享 runtime config 把控制面能力误开到 worker 上。
		cfg.Mode.EnableHTTPServer = false
		cfg.Mode.EnableCallback = false
		cfg.Mode.EnableGRPCServer = false
		cfg.Mode.EnableMQConsumer = false
		cfg.Mode.EnableScheduler = false
		cfg.Mode.EnableWorker = true
	case NodeModeStandalone, NodeModeClusterAllInOne:
		// 单机与一体化节点既能控制也能执行，运行期开关仍由 runtime config 决定。
		cfg.Mode.EnableScheduler = true
		cfg.Mode.EnableWorker = true
	}
	return cfg
}

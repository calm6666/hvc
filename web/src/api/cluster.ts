import { Alova } from '@/utils/http/alova/index';
import type { ListNodeResponse, ListWorkersResponse, ListMembersResponse, ClusterOverviewData, SchedulerInsightData, SetNodeEnabledRequest, SetNodeQuarantinedRequest, SetNodeDrainingRequest, SetWorkerOfflineRequest, SetWorkerExitedRequest } from '@/types/api';

/**
 * 集群管理 API — ClusterHandler (internal/interfaces/http/admin/cluster.go)
 */

// ===== 集群概览 & 实时 =====

/** 集群总览（缓存 2s）。GET /v1/admin/cluster/overview。权限: cluster.read */
export function clusterOverview() { return Alova.Get<ClusterOverviewData>('/cluster/overview'); }

/** 集群实时指标（缓存 2s）。GET /v1/admin/cluster/realtime。权限: cluster.read */
export function clusterRealtime() { return Alova.Get<Record<string, unknown>>('/cluster/realtime'); }

/** 集群拓扑结构。GET /v1/admin/cluster/topology。权限: cluster.read */
export function clusterTopology() { return Alova.Get<Record<string, unknown>>('/cluster/topology'); }

/** 资源分布数据。GET /v1/admin/cluster/resource/distribution。权限: cluster.read */
export function resourceDistribution() { return Alova.Get<Record<string, unknown>>('/cluster/resource/distribution'); }

// ===== 节点管理 =====

/** 节点分页列表（含 GPU 汇总、在线状态、心跳等）。GET /v1/admin/cluster/node/list。权限: cluster.node.read */
export function listNodes(params: { page: number; pageSize: number; keyword?: string; enabled?: boolean; quarantined?: boolean; draining?: boolean }) {
  return Alova.Get<ListNodeResponse>('/cluster/node/list', { params });
}

/** 节点详情（含 GPU 设备列表）。GET /v1/admin/cluster/node/detail?node_id=xxx。权限: cluster.node.read */
export function nodeDetail(nodeId: number) { return Alova.Get<Record<string, unknown>>('/cluster/node/detail', { params: { nodeId } }); }

/** 节点指标分页。GET /v1/admin/cluster/node/metrics。权限: cluster.node.metrics.read */
export function listNodeMetrics(params: { page: number; pageSize: number; nodeId?: number }) {
  return Alova.Get<Record<string, unknown>>('/cluster/node/metrics', { params });
}

/** 启用/禁用节点。POST /v1/admin/cluster/node/enabled。权限: cluster.node.enable */
export function setNodeEnabled(data: SetNodeEnabledRequest) { return Alova.Post('/cluster/node/enabled', data); }

/** 隔离/解除隔离节点。POST /v1/admin/cluster/node/quarantined。权限: cluster.node.quarantine */
export function setNodeQuarantined(data: SetNodeQuarantinedRequest) { return Alova.Post('/cluster/node/quarantined', data); }

/** 节点排水（停止接收新任务）。POST /v1/admin/cluster/node/draining。权限: cluster.node.drain */
export function setNodeDraining(data: SetNodeDrainingRequest) { return Alova.Post('/cluster/node/draining', data); }

// ===== Worker 管理 =====

/** Worker 分页列表。GET /v1/admin/cluster/worker/list。权限: cluster.read */
export function listWorkers(params: { page: number; pageSize: number; nodeId?: number; workerId?: string; status?: number; onlineOnly?: boolean }) {
  return Alova.Get<ListWorkersResponse>('/cluster/worker/list', { params });
}

/** Worker 下线。POST /v1/admin/cluster/worker/offline。权限: cluster.worker.offline */
export function setWorkerOffline(data: SetWorkerOfflineRequest) { return Alova.Post('/cluster/worker/offline', data); }

/** Worker 强制退出。POST /v1/admin/cluster/worker/exit。权限: cluster.worker.exit */
export function setWorkerExited(data: SetWorkerExitedRequest) { return Alova.Post('/cluster/worker/exit', data); }

// ===== 成员 & 调度器 =====

/** 集群成员分页列表。GET /v1/admin/cluster/member/list。权限: cluster.read */
export function listMembers(params: { page: number; pageSize: number; keyword?: string }) {
  return Alova.Get<ListMembersResponse>('/cluster/member/list', { params });
}

/** 调度器洞察（候选节点/决策信息）。GET /v1/admin/cluster/scheduler/insight。权限: cluster.read */
export function schedulerInsight(params?: { enableWatermark?: boolean; preferredHwAccel?: string; videoCodec?: string }) {
  return Alova.Get<SchedulerInsightData>('/cluster/scheduler/insight', { params });
}

/** 强制任务接管（从故障节点/Worker 转移任务）。POST /v1/admin/cluster/job/takeover。权限: cluster.job.takeover */
export function forceTakeoverJobs(data: { nodeId?: number; workerId?: string; reason?: string }) {
  return Alova.Post('/cluster/job/takeover', data);
}

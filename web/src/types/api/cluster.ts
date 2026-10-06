import type { PageData } from './common';

// ============================================================
// 集群管理 — ClusterHandler (internal/interfaces/http/admin/cluster.go)
// ============================================================

/**
 * 集群节点行数据。
 * 对应后端: clusterNodeListItemView (cluster_view.go)
 */
export interface ClusterNodeRow {
  nodeId: number;
  nodeName: string;
  nodeRole: string;
  hostIp: string;
  /** 是否启用 */
  enabled: boolean;
  /** 是否隔离 */
  quarantined: boolean;
  /** 隔离原因 */
  quarantineReason: string;
  /** 是否排水 */
  draining: boolean;
  /** 排水原因 */
  drainReason: string;
  /** 是否控制面节点 */
  controlPlane: boolean;
  /** 是否在线 */
  onlineEstimate: boolean;
  /** 在线信号来源 */
  onlineSignalSource: string;
  /** 是否调度就绪 */
  schedulerReady: boolean;
  cpuCores: number;
  memoryTotalMb: number;
  diskTotalGb: number;
  maxTranscodeSessions: number;
  gpuSummary?: ClusterNodeGpuSummary;
  lastHeartbeatAt: string;
  lastStateChangeAt: string;
}

/** GPU 汇总信息 */
export interface ClusterNodeGpuSummary {
  total: number;
  healthyTotal: number;
  schedulableTotal: number;
  maxTranscodeSessions: number;
}

/** 节点操作请求 */
export interface SetNodeEnabledRequest { nodeId: number; enabled: boolean; reason?: string; takeoverActiveJobs?: boolean; }
export interface SetNodeQuarantinedRequest { nodeId: number; quarantined: boolean; reason?: string; takeoverActiveJobs?: boolean; }
export interface SetNodeDrainingRequest { nodeId: number; draining: boolean; reason?: string; takeoverActiveJobs?: boolean; }

/**
 * Worker 行数据。
 * 对应后端: clusterWorkerListItemView (cluster_view.go)
 */
export interface WorkerRow {
  id: number;
  workerId: string;
  nodeId: number;
  /** 状态名称: "online" | "offline" | "exited" */
  statusName: string;
  /** 是否在线评估 */
  onlineEstimate: boolean;
  version: string;
  startAt: string;
  exitedAt: string | null;
  exitReason: string;
  lastHeartbeatAt: string;
  createdAt: string;
  updatedAt: string;
}

export interface SetWorkerOfflineRequest { workerId: string; reason?: string; takeoverActiveJobs?: boolean; }
export interface SetWorkerExitedRequest { workerId: string; reason?: string; takeoverActiveJobs?: boolean; }

/** 集群成员行数据 */
export interface ClusterMemberRow {
  nodeId: number;
  nodeName: string;
  nodeRole: string;
  hostIp: string;
  enabled: boolean;
  quarantined: boolean;
  draining: boolean;
  controlPlane: boolean;
  onlineEstimate: boolean;
  workerTotal: number;
  workerOnlineTotal: number;
  lastHeartbeatAt: string;
}

/** 集群总览 data */
export interface ClusterOverviewData {
  mode: string;
  nodeMode: string;
  cluster: {
    nodeTotal: number;
    nodeEnabledTotal: number;
    nodeQuarantinedTotal: number;
    nodeDrainingTotal: number;
    nodeOnlineTotal: number;
    memberTotal: number;
    workerTotal: number;
    workerOnlineTotal: number;
    gpuTotal: number;
    gpuHealthyTotal: number;
    gpuSchedulableTotal: number;
    activeSessions: number;
    uploadQueueDepth: number;
    maxTranscodeSessions: number;
  };
}

/** 调度器洞察 data */
export interface SchedulerInsightData {
  candidates: unknown[];
  decisions: unknown[];
}

export type ListNodeResponse = PageData<ClusterNodeRow>;
export type ListWorkersResponse = PageData<WorkerRow>;
export type ListMembersResponse = PageData<ClusterMemberRow>;

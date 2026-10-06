import type { PageData } from './common';

// ============================================================
// 转码管理 — TranscodeHandler (internal/interfaces/http/admin/transcode.go)
// ============================================================

/**
 * 转码任务行数据。
 * 对应后端 ListJobs 返回的 map[string]any 中的字段。
 */
export interface TranscodeJobRow {
  jobId: number;
  requestId: string;
  /** 业务标识 */
  bizKey: string;
  /** 源文件 URL */
  sourceUrl: string;
  /** 转码配置 ID */
  profileId: number;
  /** 状态名 */
  statusName: string;
  /** 进度（千分比，0-1000） */
  progressPermille: number;
  /** 当前阶段 */
  stage: string;
  /** 分配的节点 ID */
  assignedNodeId: number;
  /** 分配的 Worker ID */
  assignedWorkerId: string;
  createdAt: string;
  updatedAt: string;
}

/** 转码任务详情 */
export interface JobDetailData extends TranscodeJobRow {
  enableWatermark: boolean;
  segmentDurationSec: number;
  supportDash: boolean;
  supportHls: boolean;
  selectedExecutionHw: string;
  leaseGeneration: number;
  realtimeProgress?: {
    currentFps: number;
    currentBitrateKbps: number;
    currentSpeed: string;
    elapsedMs: number;
    estimatedRemainingMs: number;
  };
}

/** 转码任务进度 */
export interface JobProgressData {
  jobId: number;
  status: number;
  statusName: string;
  stage: string;
  progressPermille: number;
  currentFps?: number;
  currentBitrateKbps?: number;
}

export interface RetryJobRequest { jobId: number; }
export interface CancelJobRequest { jobId: number; }

export type ListJobsResponse = PageData<TranscodeJobRow>;

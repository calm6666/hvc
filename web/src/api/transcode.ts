import { Alova } from '@/utils/http/alova/index';
import type { ListJobsResponse, JobDetailData, JobProgressData, RetryJobRequest, CancelJobRequest } from '@/types/api';

/**
 * 转码管理 API — TranscodeHandler (internal/interfaces/http/admin/transcode.go)
 */

/** 转码任务分页列表。GET /v1/admin/transcode/job/list。权限: transcode.job.read */
export function listJobs(params: { page: number; pageSize: number; status?: number; bizKey?: string; requestId?: string }) {
  return Alova.Get<ListJobsResponse>('/transcode/job/list', { params });
}

/** 转码任务详情（含实时进度字段 realtimeProgress）。GET /v1/admin/transcode/job/detail?job_id=xxx。权限: transcode.job.detail.read */
export function jobDetail(jobId: number) { return Alova.Get<JobDetailData>('/transcode/job/detail', { params: { jobId } }); }

/** 转码任务实时进度快照（从 Redis 读取）。GET /v1/admin/transcode/job/progress?job_id=xxx。权限: transcode.job.read */
export function jobProgress(jobId: number) { return Alova.Get<JobProgressData>('/transcode/job/progress', { params: { jobId } }); }

/** 重试失败的转码任务。POST /v1/admin/transcode/job/retry。权限: transcode.job.retry */
export function retryJob(data: RetryJobRequest) { return Alova.Post<{ code: number; message: string }>('/transcode/job/retry', data); }

/** 取消进行中的转码任务。POST /v1/admin/transcode/job/cancel。权限: transcode.job.cancel */
export function cancelJob(data: CancelJobRequest) { return Alova.Post<{ code: number; message: string }>('/transcode/job/cancel', data); }

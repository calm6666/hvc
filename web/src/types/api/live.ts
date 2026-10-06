import type { PageData } from './common';

// ============================================================
// 直播管理 — LiveHandler (internal/interfaces/http/admin/live.go)
// ============================================================

/**
 * 直播频道行数据。
 * 对应后端: model.LiveChannel
 *
 * Go struct → toCamelCase → TS:
 *   ChannelID             → channelId
 *   ChannelKey            → channelKey
 *   ChannelName           → channelName
 *   ProfileID             → profileId
 *   Status                → status            // "active" | "idle" | "stopped"
 *   EnableSourceRendition → enableSourceRendition
 *   EnableWatermark       → enableWatermark
 *   PlayDomain            → playDomain
 *   PushDomain            → pushDomain
 *   AssignedNodeID        → assignedNodeId
 *   AssignedWorkerID      → assignedWorkerId
 *   CreatedAt             → createdAt
 *   UpdatedAt             → updatedAt
 */
export interface LiveChannelRow {
  channelId: number;
  /** 频道唯一标识 */
  channelKey: string;
  /** 频道名称 */
  channelName: string;
  /** 转码配置 ID */
  profileId: number;
  /** 状态: "active"=推流中, "idle"=空闲, "stopped"=已停止 */
  status: string;
  enableSourceRendition: boolean;
  enableWatermark: boolean;
  /** 播放域名 */
  playDomain: string;
  /** 推流域名 */
  pushDomain: string;
  assignedNodeId: number;
  assignedWorkerId: string;
  createdAt: string;
  updatedAt: string;
}

/**
 * 直播会话行数据。
 * 对应后端: ListSessions 返回
 */
export interface LiveSessionRow {
  sessionId: number;
  channelId: number;
  channelKey: string;
  status: string;
  startAt: string;
  endAt: string | null;
}

/** 创建频道请求 */
export interface CreateChannelRequest {
  channelKey: string;
  channelName: string;
  profileId: number;
}

/** 更新频道请求 */
export interface UpdateChannelRequest {
  channelId: number;
  channelName?: string;
  profileId?: number;
  enableSourceRendition?: boolean;
  enableWatermark?: boolean;
  playDomain?: string;
  pushDomain?: string;
}

export interface StartChannelRequest { channelId: number; }
export interface StopChannelRequest { channelId: number; }
export interface DeleteChannelRequest { channelId: number; }

export type ListChannelsResponse = PageData<LiveChannelRow>;
export type ListSessionsResponse = PageData<LiveSessionRow>;

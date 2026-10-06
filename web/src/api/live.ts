import { Alova } from '@/utils/http/alova/index';
import type { ListChannelsResponse, ListSessionsResponse, LiveChannelRow, CreateChannelRequest, UpdateChannelRequest, StartChannelRequest, StopChannelRequest, DeleteChannelRequest } from '@/types/api';

/**
 * 直播管理 API — LiveHandler (internal/interfaces/http/admin/live.go)
 */

/** 直播频道分页列表。GET /v1/admin/live/channel/list。权限: live.channel.read */
export function listChannels(params: { page: number; pageSize: number; status?: string; channelKey?: string }) {
  return Alova.Get<ListChannelsResponse>('/live/channel/list', { params });
}

/** 频道详情（含播放回放信息 + 活跃会话）。GET /v1/admin/live/channel/detail。权限: live.channel.read */
export function channelDetail(params: { channelId?: number; channelKey?: string }) {
  return Alova.Get<Record<string, unknown>>('/live/channel/detail', { params });
}

/** 创建频道。POST /v1/admin/live/channel/create。权限: live.channel.create */
export function createChannel(data: CreateChannelRequest) { return Alova.Post<LiveChannelRow>('/live/channel/create', data); }

/** 更新频道（域名/水印/画质等）。POST /v1/admin/live/channel/update。权限: live.channel.update */
export function updateChannel(data: UpdateChannelRequest) { return Alova.Post<LiveChannelRow>('/live/channel/update', data); }

/** 启动频道（开始拉流转码）。POST /v1/admin/live/channel/start。权限: live.channel.start */
export function startChannel(data: StartChannelRequest) { return Alova.Post('/live/channel/start', data); }

/** 停止频道。POST /v1/admin/live/channel/stop。权限: live.channel.stop */
export function stopChannel(data: StopChannelRequest) { return Alova.Post('/live/channel/stop', data); }

/** 删除频道。POST /v1/admin/live/channel/delete。权限: live.channel.delete */
export function deleteChannel(data: DeleteChannelRequest) { return Alova.Post('/live/channel/delete', data); }

/** 直播会话分页列表。GET /v1/admin/live/session/list。权限: live.session.read */
export function listSessions(params: { page: number; pageSize: number; channelId?: number; status?: string }) {
  return Alova.Get<ListSessionsResponse>('/live/session/list', { params });
}

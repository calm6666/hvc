import { Alova } from '@/utils/http/alova/index';
import type { ListRuntimeVersionsResponse, ListCallbacksResponse, ListConfigCenterResponse, ListEtcdRegistriesResponse, ListNamingTemplatesResponse, PublishRuntimeRequest, SetCallbackEnabledRequest, SetConfigCenterEnabledRequest, SetEtcdRegistryEnabledRequest, ActivateNamingTemplateRequest } from '@/types/api';

/**
 * 配置管理 API — ConfigHandler / CallbackHandler / NamingTemplateHandler / ConfigCenterHandler / RegistryEtcdHandler
 * (internal/interfaces/http/admin/)
 */

// ===== 运行配置版本 =====

/** 运行配置版本列表（分页）。GET /v1/admin/config/runtime/versions。权限: config.version.read */
export function listRuntimeVersions(params: { page: number; pageSize: number; published?: boolean }) {
  return Alova.Get<ListRuntimeVersionsResponse>('/config/runtime/versions', { params });
}

/** 发布配置版本（使指定版本生效）。POST /v1/admin/config/publish。权限: config.version.publish */
export function publishRuntime(data: PublishRuntimeRequest) { return Alova.Post('/config/publish', data); }

/** 更新 Server 运行配置。POST /v1/admin/config/runtime/server/update。权限: config.runtime.update */
export function updateRuntimeServer(data: Record<string, unknown>) { return Alova.Post('/config/runtime/server/update', data); }

/** 更新 Scheduler 运行配置。权限: config.runtime.update */
export function updateRuntimeScheduler(data: Record<string, unknown>) { return Alova.Post('/config/runtime/scheduler/update', data); }

/** 更新 Worker 运行配置。权限: config.runtime.update */
export function updateRuntimeWorker(data: Record<string, unknown>) { return Alova.Post('/config/runtime/worker/update', data); }

/** 更新 Storage 运行配置（对象存储类型/端点/Bucket/域名）。权限: config.runtime.update */
export function updateRuntimeStorage(data: Record<string, unknown>) { return Alova.Post('/config/runtime/storage/update', data); }

/** 更新 Callback 运行配置。权限: config.runtime.update */
export function updateRuntimeCallback(data: Record<string, unknown>) { return Alova.Post('/config/runtime/callback/update', data); }

/** 更新 MQ 运行配置（RabbitMQ 连接参数）。权限: config.runtime.update */
export function updateRuntimeMq(data: Record<string, unknown>) { return Alova.Post('/config/runtime/mq/update', data); }

/** 更新 gRPC 运行配置（监听地址/注册中心绑定）。权限: config.runtime.update */
export function updateRuntimeGrpc(data: Record<string, unknown>) { return Alova.Post('/config/runtime/grpc/update', data); }

// ===== 回调配置 =====

/** 回调配置列表。GET /v1/admin/config/callback/list。权限: config.callback.read */
export function listCallbacks(params: { page: number; pageSize: number; callbackName?: string; enabled?: boolean }) {
  return Alova.Get<ListCallbacksResponse>('/config/callback/list', { params });
}

/** 创建/更新回调配置。POST /v1/admin/config/callback/upsert。权限: config.callback.update */
export function upsertCallback(data: Record<string, unknown>) { return Alova.Post('/config/callback/upsert', data); }

/** 启用/禁用回调。POST /v1/admin/config/callback/enabled。权限: config.callback.update */
export function setCallbackEnabled(data: SetCallbackEnabledRequest) { return Alova.Post('/config/callback/enabled', data); }

// ===== 命名模板 =====

/** 命名模板列表（含 6 个预设模板）。GET /v1/admin/config/naming-template/list。权限: config.naming_template.read */
export function listNamingTemplates(params: { page: number; pageSize: number }) {
  return Alova.Get<ListNamingTemplatesResponse>('/config/naming-template/list', { params });
}

/** 配置命名模板（选预设或自定义）。POST /v1/admin/config/naming-template/configure。权限: config.naming_template.update */
export function configureNamingTemplate(data: { templateId?: number; customTemplate?: string }) { return Alova.Post('/config/naming-template/configure', data); }

/** 激活命名模板（立即生效）。POST /v1/admin/config/naming-template/activate。权限: config.naming_template.update */
export function activateNamingTemplate(data: ActivateNamingTemplateRequest) { return Alova.Post('/config/naming-template/activate', data); }

// ===== 配置中心 =====

/** 配置中心绑定列表。GET /v1/admin/config-center/list。权限: config.version.read */
export function listConfigCenter(params: { page: number; pageSize: number; providerType?: string; enabled?: boolean }) {
  return Alova.Get<ListConfigCenterResponse>('/config-center/list', { params });
}

/** 创建/更新配置中心绑定。POST /v1/admin/config-center/upsert。权限: config.version.publish */
export function upsertConfigCenter(data: Record<string, unknown>) { return Alova.Post('/config-center/upsert', data); }

/** 启用/禁用配置中心。POST /v1/admin/config-center/enabled。权限: config.version.publish */
export function setConfigCenterEnabled(data: SetConfigCenterEnabledRequest) { return Alova.Post('/config-center/enabled', data); }

// ===== Etcd 注册中心 =====

/** Etcd 注册中心列表。GET /v1/admin/registry/etcd/list。权限: config.version.read */
export function listEtcdRegistries(params: { page: number; pageSize: number; registryName?: string; enabled?: boolean }) {
  return Alova.Get<ListEtcdRegistriesResponse>('/registry/etcd/list', { params });
}

/** 创建/更新 Etcd 注册中心。POST /v1/admin/registry/etcd/upsert。权限: config.runtime.update */
export function upsertEtcdRegistry(data: Record<string, unknown>) { return Alova.Post('/registry/etcd/upsert', data); }

/** 启用/禁用 Etcd 注册中心。POST /v1/admin/registry/etcd/enabled。权限: config.runtime.update */
export function setEtcdRegistryEnabled(data: SetEtcdRegistryEnabledRequest) { return Alova.Post('/registry/etcd/enabled', data); }

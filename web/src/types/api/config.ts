import type { PageData } from './common';

// ============================================================
// 配置管理 — ConfigHandler / CallbackHandler / etc. (internal/interfaces/http/admin/)
// ============================================================

/** 回调配置行数据。对应: callbackConfigView */
export interface CallbackRow {
  callbackConfigId: number;
  callbackName: string;
  callbackType: number;
  targetUrl: string;
  rpcEndpoint: string;
  rpcServiceName: string;
  mqExchange: string;
  mqRoutingKey: string;
  timeoutMs: number;
  retryTimes: number;
  enabled: boolean;
  priority: number;
  registryId: number;
  createdAt: string;
  updatedAt: string;
}

/** 配置中心绑定行数据。对应: configCenterBindingView */
export interface ConfigCenterRow {
  bindingId: number;
  bindingName: string;
  providerType: string;
  endpoint: string;
  namespace: string;
  authMode: string;
  enabled: boolean;
  priority: number;
  lastSyncStatus: string;
  lastSyncMessage: string;
  lastSyncAt: string | null;
  createdAt: string;
  updatedAt: string;
}

/** Etcd 注册中心行数据。对应: registryEtcdConfigView */
export interface EtcdRegistryRow {
  registryId: number;
  registryName: string;
  endpoints: string;
  serviceNamespace: string;
  leaseTtlSec: number;
  dialTimeoutMs: number;
  enabled: boolean;
  priority: number;
  createdAt: string;
  updatedAt: string;
}

/** 运行配置版本行数据。对应: runtimeConfigVersionView */
export interface RuntimeVersionRow {
  configVersion: number;
  changeSummary: string;
  published: boolean;
  publishedBy: string;
  publishedAt: string | null;
  effectiveConfigHash: string;
  createdAt: string;
  updatedAt: string;
}

/** 命名模板行数据。对应: PresetTemplate */
export interface NamingTemplateRow {
  id: number;
  name: string;
  template: string;
  exampleInitVideo: string;
  exampleMediaVideo: string;
  exampleInitAudio: string;
  exampleMediaAudio: string;
  description: string;
}

/** 发布配置请求 */
export interface PublishRuntimeRequest { configVersion: number; publishReason?: string; }
/** 回调启用/禁用请求 */
export interface SetCallbackEnabledRequest { callbackConfigId: number; enabled: boolean; }
/** 配置中心启用/禁用请求 */
export interface SetConfigCenterEnabledRequest { bindingId: number; enabled: boolean; }
/** Etcd 启用/禁用请求 */
export interface SetEtcdRegistryEnabledRequest { registryId: number; enabled: boolean; }
/** 激活命名模板请求 */
export interface ActivateNamingTemplateRequest { template: string; }

export type ListCallbacksResponse = PageData<CallbackRow>;
export type ListConfigCenterResponse = PageData<ConfigCenterRow>;
export type ListEtcdRegistriesResponse = PageData<EtcdRegistryRow>;
export type ListRuntimeVersionsResponse = PageData<RuntimeVersionRow>;
export type ListNamingTemplatesResponse = PageData<NamingTemplateRow>;

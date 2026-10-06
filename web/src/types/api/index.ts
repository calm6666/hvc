/**
 * HVC 后端 API 类型定义 — 统一导出入口。
 *
 * 所有与后端 Go 结构体对标的 TypeScript 类型集中在此处管理。
 * 命名规则：
 *   - Row 后缀 — 列表行数据类型
 *   - Request 后缀 — 请求体类型
 *   - Response 后缀 — API 响应 data 类型
 *   - Data 后缀 — 非分页/非标准响应 data 类型
 */

export type {
  ApiEnvelope,
  PageData,
  ItemsData,
} from './common';

export type {
  LoginRequest,
  LoginResponseData,
  LoginResponse,
  AdminUserView,
  WhoAmIData,
  WhoAmIResponse,
} from './auth';

export type {
  AdminUserRow,
  ListUsersResponse,
  UpsertUserRequest,
  SetUserStatusRequest,
  BindUserRoleRequest,
} from './user';

export type {
  AdminRoleRow,
  ListRolesResponse,
  AllRolesResponse,
  UpsertRoleRequest,
  RoleMenuTreeData,
  AssignRoleMenusRequest,
  BindRolePermissionRequest,
} from './role';

export type {
  AdminMenuNode,
  UpsertMenuRequest,
  DeleteMenuRequest,
} from './menu';

export type {
  AdminPermission,
  PermissionTreeNode,
  UpsertPermissionRequest,
} from './permission';

export type {
  AuditLogRow,
  ListAuditLogsResponse,
  ListAuditLogsParams,
} from './audit';

export type {
  RuntimeLogRow,
  ListRuntimeLogsResponse,
  ListRuntimeLogsParams,
} from './log';

export type {
  ClusterNodeRow,
  ClusterNodeGpuSummary,
  WorkerRow,
  ClusterMemberRow,
  ClusterOverviewData,
  SchedulerInsightData,
  SetNodeEnabledRequest,
  SetNodeQuarantinedRequest,
  SetNodeDrainingRequest,
  SetWorkerOfflineRequest,
  SetWorkerExitedRequest,
  ListNodeResponse,
  ListWorkersResponse,
  ListMembersResponse,
} from './cluster';

export type {
  TranscodeJobRow,
  JobDetailData,
  JobProgressData,
  RetryJobRequest,
  CancelJobRequest,
  ListJobsResponse,
} from './transcode';

export type {
  LiveChannelRow,
  LiveSessionRow,
  CreateChannelRequest,
  UpdateChannelRequest,
  StartChannelRequest,
  StopChannelRequest,
  DeleteChannelRequest,
  ListChannelsResponse,
  ListSessionsResponse,
} from './live';

export type {
  CallbackRow,
  ConfigCenterRow,
  EtcdRegistryRow,
  RuntimeVersionRow,
  NamingTemplateRow,
  PublishRuntimeRequest,
  SetCallbackEnabledRequest,
  SetConfigCenterEnabledRequest,
  SetEtcdRegistryEnabledRequest,
  ActivateNamingTemplateRequest,
  ListCallbacksResponse,
  ListConfigCenterResponse,
  ListEtcdRegistriesResponse,
  ListRuntimeVersionsResponse,
  ListNamingTemplatesResponse,
} from './config';

# HVC Web TypeScript 全面改造方案

> **核心目标**：将整个项目从"any 泛滥的弱类型项目"改造为"完整的 TypeScript 强类型项目"。
>
> **强制规则**：
> 1. 全项目零 `any`（工具类型守卫、泛型约束除外）
> 2. 所有 `Alova.Get/Post` 必须加泛型 `<TResponded, TTransformed>`
> 3. 所有函数必须有返回值类型标注（`: void` / `: Promise<T>` 等）
> 4. 所有 interface / type / function 必须有中文注释说明字段来源和用途
> 5. 封装组件（Table/Form/Modal/Upload）的类型系统优先修复

---

## 一、现状诊断

### 1.1 `any` 分布统计

```
目录                 any 数量    严重程度    原因
──────────────────────────────────────────────────────────────
components/Table/    20+         🔴 严重     原始封装组件未定义类型
components/Form/      6          🟡 中等     原始封装组件未定义类型
store/                7          🟡 中等     asyncRoute 路由树类型
router/               7          🟡 中等     generateRoutes 输入输出
utils/                16         🟡 中等     tool functions
views/                18         🟢 较低     视图行类型 + catch 块
api/                  0 泛型     🔴 严重     全部 API 缺少泛型参数
──────────────────────────────────────────────────────────────
合计                  74+
```

### 1.2 根因分析

| 根因 | 说明 |
|------|------|
| **API 层零泛型** | 全部 `Alova.Get()` 返回 `Promise<unknown>`，调用方无类型信息 |
| **组件层类型不完整** | Table 的 columns/request/dataSource 全是 `any` |
| **路由层类型不完整** | `generateRoutes` 入参/出参都是 `any[]` |
| **后端字段无前端映射** | Go 结构体 → JSON snake_case → toCamelCase 后，前端无对应的 TS interface |
| **Form/Modal 动态类型** | `FormSchema.component` 是字符串，`componentProps` 是 `object`，无法做组件级类型校验 |

---

## 二、Alova 泛型机制

Alova v3 的 `Get/Post` 方法签名（来自 `alova/typings/index.d.ts:643`）：

```typescript
Get<Responded = unknown, Transformed = unknown>(
  url: string,
  config?: AlovaRequestConfig
): Method<...>

Post<Responded = unknown, Transformed = unknown>(
  url: string,
  data?: RequestBody,
  config?: AlovaRequestConfig
): Method<...>
```

**两个泛型参数**：
- `Responded` — `responded.onSuccess` 拦截器返回的类型（对调用方暴露的内部类型）
- `Transformed` — 经 `transformData` 可选钩子转换后的最终类型

**本项目拦截器返回逻辑**：

```
isReturnNativeResponse: true  → return res         → 类型: { code: number; message: string; data: D }
默认                          → return toCamelCase(data)  → 类型: CamelCaseKeys<D>
```

**泛型使用示例**：

```typescript
// 分页列表 — 默认模式，返回 data 对象（已 toCamelCase）
interface PageData<T> { items: T[]; page: number; pageSize: number; total: number; }

export function listUsers(params: ListUsersParams) {
  return Alova.Get<PageData<AdminUserRow>>('/system/user/list', { params });
}
// 返回类型: Promise<PageData<AdminUserRow>>

// 原生响应 — isReturnNativeResponse: true，返回完整信封
export function whoAmI() {
  return Alova.Get<WhoAmIResponse>('/auth/me', { meta: { isReturnNativeResponse: true } });
}
// 返回类型: Promise<WhoAmIResponse>
```

---

## 三、类型系统设计

### 3.1 后端数据 → 前端 TS 类型映射

所有后端 Go 结构体在 `src/types/api/` 下建对应的 TS interface：

```
后端 Go                        → 前端 TS (toCamelCase后)
═══════════════════════════════════════════════════════════════
model.Response                 → ApiEnvelope<T>
adminUserView                  → AdminUserRow
adminRoleView                  → AdminRoleRow
adminMenuNode                  → AdminMenuNode
adminPermissionView            → AdminPermission
permissionTreeNode             → PermissionTreeNode
auditLogView                   → AuditLogRow
runtimeLogView                 → RuntimeLogRow
clusterNodeListItemView        → ClusterNodeRow
clusterWorkerListItemView      → WorkerRow
clusterMemberView              → ClusterMemberRow
LiveChannel                    → LiveChannelRow
callbackConfigView             → CallbackRow
configCenterBindingView        → ConfigCenterRow
registryEtcdConfigView         → EtcdRegistryRow
runtimeConfigVersionView       → RuntimeVersionRow
```

### 3.2 新增类型文件

```
src/types/
  ├── api/
  │   ├── auth.ts          # Auth 模块类型
  │   ├── user.ts          # 用户管理类型
  │   ├── role.ts          # 角色管理类型
  │   ├── menu.ts          # 菜单管理类型
  │   ├── permission.ts    # 权限管理类型
  │   ├── audit.ts         # 审计日志类型
  │   ├── log.ts           # 运行日志类型
  │   ├── cluster.ts       # 集群管理类型
  │   ├── transcode.ts     # 转码管理类型
  │   ├── live.ts          # 直播管理类型
  │   ├── config.ts        # 配置管理类型
  │   └── common.ts        # 通用类型（Envelope, PageData）
  ├── component/
  │   ├── table.ts         # BasicTable 相关类型
  │   └── form.ts          # BasicForm 相关类型
  ├── router.ts            # 路由类型（AppRouteRecordRaw, Menu）
  └── store.ts             # Store 类型
```

### 3.3 通用类型定义

```typescript
// src/types/api/common.ts

/**
 * 后端统一响应信封。
 * Go: model.Response{ Code int; Message string; Data interface{} }
 */
export interface ApiEnvelope<T = unknown> {
  code: number;
  message: string;
  data: T;
}

/**
 * 分页响应 data 结构。
 * Go: writePageResponse → { page, page_size, total, items }
 * 经 toCamelCase 后：{ page, pageSize, total, items }
 */
export interface PageData<T> {
  page: number;
  pageSize: number;
  total: number;   // Go 字段 "total" 是总条数，不是总页数
  items: T[];
}

/**
 * 非分页列表响应 data 结构。
 * Go: writeItemsResponse → { items, ...meta }
 * 经 toCamelCase 后：{ items, ...meta }
 */
export interface ItemsData<T> {
  items: T[];
  [metaKey: string]: unknown;  // 扩展元数据（如 tree: true, roleId 等）
}
```

---

## 四、分阶段修改计划

### 阶段 1：公共类型 + API 泛型（核心基础）

**目标**：建立类型基础，后续所有修改都依赖此阶段。

| 文件 | 改动 | 预计行数 |
|------|------|---------|
| `src/types/api/common.ts` | **新建** — `ApiEnvelope<T>`, `PageData<T>`, `ItemsData<T>` | 30 |
| `src/types/api/auth.ts` | **新建** — `LoginRequest`, `LoginResponse`, `WhoAmIResponse`, `WhoAmIData`, `AdminUserView` | 50 |
| `src/types/api/user.ts` | **新建** — `AdminUserRow`, `ListUsersParams`, `UpsertUserParams`, `SetUserStatusParams`, `BindUserRoleParams` | 40 |
| `src/types/api/role.ts` | **新建** — `AdminRoleRow`, `ListRolesParams`, `UpsertRoleParams`, `RoleMenuTreeData` | 40 |
| `src/types/api/menu.ts` | **新建** — `AdminMenuNode`, `UpsertMenuParams`, `DeleteMenuParams` | 35 |
| `src/types/api/permission.ts` | **新建** — `PermissionTreeNode`, `AdminPermission`, `UpsertPermissionParams` | 35 |
| `src/types/api/audit.ts` | **新建** — `AuditLogRow`, `ListAuditLogsParams` | 25 |
| `src/types/api/log.ts` | **新建** — `RuntimeLogRow`, `ListRuntimeLogsParams` | 25 |
| `src/types/api/cluster.ts` | **新建** — `ClusterNodeRow`, `WorkerRow`, `ClusterMemberRow`, `ClusterOverviewData`, 各操作参数 | 80 |
| `src/types/api/transcode.ts` | **新建** — `TranscodeJobRow`, `JobDetailData`, 各操作参数 | 40 |
| `src/types/api/live.ts` | **新建** — `LiveChannelRow`, `LiveSessionRow`, 各操作参数 | 50 |
| `src/types/api/config.ts` | **新建** — `CallbackRow`, `ConfigCenterRow`, `EtcdRegistryRow`, `RuntimeVersionRow`, `NamingTemplateRow`, 各操作参数 | 70 |
| `src/types/api/index.ts` | **新建** — 统一 re-export | 15 |

**同时修改所有 API 文件，加上泛型参数**：

| 文件 | 改动 |
|------|------|
| `api/auth.ts` | 3 个函数全部加 `<TResponded>` 泛型 |
| `api/system/user.ts` | 4 个函数全部加泛型 |
| `api/system/role.ts` | 6 个函数全部加泛型 |
| `api/system/menu.ts` | 3 个函数全部加泛型 |
| `api/system/permission.ts` | 2 个函数全部加泛型 |
| `api/system/audit.ts` | 1 个函数加泛型 |
| `api/system/log.ts` | 1 个函数加泛型 |
| `api/cluster.ts` | 14 个函数全部加泛型 |
| `api/transcode.ts` | 5 个函数全部加泛型 |
| `api/live.ts` | 8 个函数全部加泛型 |
| `api/config.ts` | 16 个函数全部加泛型 |

**阶段 合计**：新建 13 个类型文件（~535行）+ 修改 11 个 API 文件

### 阶段 2：封装组件类型修复

**目标**：消除 Table/Form/Modal/Upload 封装组件中的所有 `any`。

#### 2.1 Table 组件（20 any → 0）

| 文件 | 当前 `any` | 修复方案 |
|------|-----------|---------|
| `types/table.ts` | `columns: any[]`, `actionColumn: any[]`, `emit?: any`, `editValueMap?: (value: any) => string` | 改为 `columns: BasicColumn[]`, `actionColumn: BasicColumn`, `emit: TableEmit` |
| `types/tableAction.ts` | - | 添加 `TableActionEmit` 类型 |
| `types/pagination.ts` | `prefix?: any` | `prefix?: (info: PaginationInfo) => string` |
| `types/componentType.ts` | - | 使用字面量联合类型替代字符串 |
| `hooks/useDataSource.ts` | `const { ... }: any` 多处 | 使用 `propsRef.value` 的类型，添加 `FetchParams` 接口 |
| `hooks/useColumns.ts` | `const title: any`, `columns: any[]` 多处 | 添加 `ColumnConfig` 接口 |
| `editable/EditableCell.vue` | `handleChange(e: any)`, `onEvent: any` | `handleChange(e: ChangeEvent)`, `onEvent: EventEnum` |
| `Table.vue` | `tableEl: any` | `tableEl: Ref<NDataTableInst | null>` |
| `settings/ColumnSetting.vue` | 5 处 any | 使用 `BasicColumn[]` 替代 `any[]` |

#### 2.2 Form 组件（6 any → 0）

| 文件 | 当前 `any` | 修复方案 |
|------|-----------|---------|
| `types/form.ts` | `defaultValue?: any`, `validate: (...) => Promise<any>` | `defaultValue?: unknown`, `validate: () => Promise<Record<string, unknown>>` |
| `hooks/useForm.ts` | `validate(nameList?: any[])` | `validate(nameList?: string[])` |
| `hooks/useFormEvents.ts` | `declare type EmitType`, `catch (error: any)` | `type EmitType<T>`, `catch (error: unknown)` |

#### 2.3 Modal 组件（0 any，已规范）

Modal 的 `useModal` 返回类型通过 `ModalMethods` 和 `UseModalReturnType` 已定义，无需改动。

#### 2.4 Upload 组件（1 any → 0）

| 文件 | 当前 `any` | 修复方案 |
|------|-----------|---------|
| `type/index.ts` | `columns: any[]` | 删除废弃字段 |

**阶段 合计**：修改 ~12 个组件文件

### 阶段 3：Store + Router 类型修复

| 文件 | 当前 `any` | 修复方案 |
|------|-----------|---------|
| `store/modules/asyncRoute.ts` | `routers: any[]`, `routersAdded: any[]`, `node: any[]` 等 6 处 | 使用 `RouteRecordRaw[]`，定义 `RouteNode` 类型 |
| `store/modules/tabsView.ts` | `list: any[]` | 使用 `RouteItem[]` |
| `store/types.ts` | - | 补全 `IStore` 中缺失的 module 类型 |
| `router/generator.ts` | `menuNodes: any[]`, `parent?: any`, `currentRoute: any`, `c: any` 等 5 处 | 使用 `AdminMenuNode[]`, `RouteNode`, `CurrentRoute` 类型 |
| `router/guards.ts` | `currentComName: any` | `currentComName: string \| undefined` |
| `router/types.ts` | `icon?: any` | `icon?: () => VNode` |

**阶段 合计**：修改 6 个文件

### 阶段 4：Utils 类型修复

| 文件 | 当前 `any` | 修复方案 |
|------|-----------|---------|
| `utils/index.ts` | `firstRouter: any[]`, `data: any[]`, `treeAll: any[]` 等 8 处 | 添加泛型约束 |
| `utils/is/index.ts` | 3 处类型守卫 | 保持 `any` 作为宽类型守卫（这是合理的） |
| `utils/Storage.ts` | `value: any`, `def: any` | `value: unknown`, `def: T` |
| `utils/domUtils.ts` | `styleObj: any`, `this: any` | 具体类型 |
| `utils/Drag.ts` | `e: any` | `e: MouseEvent` |

**阶段 合计**：修改 5 个文件

### 阶段 5：Views 类型修复 + 功能补全

#### 5.1 消除 any（18 处 → 0）

当前剩余 18 处 `any` 分布：
- `user.vue`: 4 处 — `AdminUserRow` 接口
- `role.vue`: 5 处 — `AdminRoleRow` 接口
- `role/CreateModal.vue`: 1 处 — catch
- `menu.vue`: 4 处 — `AdminMenuNode` 接口
- `menu/CreateDrawer.vue`: 3 处 — formRef + payload + catch

全部替换为阶段 1 定义的具体类型。

#### 5.2 树状表格改造（menu + permission）

**方案**：左上角树 → 全宽可展开行表格

```
改造前:
┌──────┬───────────────────┐
│ Tree │ 编辑表单           │
│      │                   │
└──────┴───────────────────┘

改造后:
┌────────────────────────────────────────────────────┐
│ [+ 添加] [全部展开/收起]                             │
│ ID │ 名称 │ 类型 │ 路由 │ 组件 │ 图标 │ 排序 │ 操作 │
│ ▶1 │ 仪表盘│menu │/dash…│/dash…│Dash…│  0  │ 编辑  │
│ ▼2 │ 系统  │menu │/sys  │      │Sett…│  1  │ 编辑  │
│   │ ▶3 │用户│menu │user  │/sys…│Team…│  0  │ 编辑  │
│   │ ▶4 │角色│menu │role  │/sys…│Safe…│  1  │ 编辑  │
└────────────────────────────────────────────────────┘
```

**实现**：写一个 `flattenTree` 工具函数，将 `AdminMenuNode[]` 树转为带 `depth` / `expanded` / `hasChildren` 的扁平行列表。BasicTable 第一列自定义渲染缩进 + 展开箭头。

#### 5.3 功能补全清单

| 页面 | 补全内容 |
|------|---------|
| `views/system/user/user.vue` | + 角色分配弹窗（`bindUserRole`） |
| `views/system/role/role.vue` | + 权限分配弹窗（`bindRolePermission`） |
| `views/cluster/nodes.vue` | + 节点详情弹窗、IP 排水、GPU 列表 |
| `views/transcode/jobs.vue` | + 详情/进度弹窗 |
| `views/live/channels.vue` | + 编辑频道、会话列表子页面 |
| `views/config/runtime.vue` | + Server/Scheduler/Worker/Storage/Callback/MQ/gRPC 7 组表单 schema |
| `views/dashboard/console/console.vue` | 替换假数据 → `cluster/overview` API |

### 阶段 6：注释补全

所有文件（API、类型、Views、组件修改部分）补充中文注释，格式：

```typescript
/**
 * [一句话描述此接口/函数/类型的作用]
 *
 * 对应后端: Go 结构体名（文件路径）
 * 字段映射: snake_case JSON → camelCase TS
 */
interface SomeType { ... }
```

---

## 五、类型文件完整示例

### `src/types/api/common.ts`

```typescript
/**
 * 后端统一响应信封。
 * 对应后端 model.Response (internal/model/system.go)
 *
 * JSON 格式: { "code": 0, "message": "ok", "data": { ... } }
 * code: 0=成功, 401=未登录, 400=参数错误, 500=服务端错误
 */
export interface ApiEnvelope<T = unknown> {
  code: number;
  message: string;
  data: T;
}

/**
 * 分页响应 data 结构。
 * 对应后端 writePageResponse (internal/interfaces/http/admin/page.go)
 *
 * 后端返回: { "page": 1, "page_size": 10, "total": 60, "items": [...] }
 * 经 HTTP 拦截器 toCamelCase 转换后:
 *   { page: number; pageSize: number; total: number; items: T[] }
 *
 * 【重要】total 是总条数，不是总页数。
 * 总页数由前端 useDataSource 计算: Math.ceil(total / pageSize)
 */
export interface PageData<T> {
  page: number;
  pageSize: number;
  total: number;
  items: T[];
}

/**
 * 非分页列表 data 结构。
 * 对应后端 writeItemsResponse (internal/interfaces/http/admin/page.go)
 *
 * 后端返回: { "items": [...], "tree": true, ...其他元数据 }
 * 经 toCamelCase 后: { items: T[]; [metaKey: string]: unknown }
 */
export interface ItemsData<T> {
  items: T[];
  [metaKey: string]: unknown;
}
```

### `src/types/api/auth.ts`

```typescript
import type { ApiEnvelope } from './common';

/**
 * 登录请求参数。
 * 后端 AuthHandler.Login 通过 r.ParseForm() 解析 application/x-www-form-urlencoded。
 */
export interface LoginRequest {
  username: string;
  password: string;
  otp_code?: string;
}

/**
 * 登录响应 data 字段。
 * 对应后端: AuthHandler.Login 返回的 data map
 */
export interface LoginResponseData {
  authenticated: boolean;
  token_transport: {
    type: 'cookie';
    cookie_name: 'admin_session';
    http_only: true;
  };
}

/**
 * 登录 API 的完整响应（isReturnNativeResponse 模式）。
 */
export type LoginResponse = ApiEnvelope<LoginResponseData>;

/**
 * 管理员用户脱敏信息。
 * 对应后端 adminUserView (internal/interfaces/http/admin/rbac_view.go)
 *
 * Go JSON tag → toCamelCase → TS 字段:
 *   admin_user_id → adminUserId
 *   username → username
 *   display_name → displayName
 *   status → status
 *   last_login_at → lastLoginAt
 *   last_login_ip → lastLoginIp
 *   created_at → createdAt
 *   updated_at → updatedAt
 */
export interface AdminUserView {
  adminUserId: number;
  username: string;
  displayName: string;
  status: number;        // 1=启用, 0=禁用
  lastLoginAt: string | null;
  lastLoginIp: string;
  createdAt: string;
  updatedAt: string;
}

/**
 * WhoAmI 接口返回的 data 字段。
 * 对应后端 AuthHandler.WhoAmI
 *
 * Go: data := map[string]any{
 *   "authenticated":   bool,
 *   "admin_user_id":   uint64,
 *   "user":            adminUserView,
 *   "permission_keys": []string,
 *   "menu_tree":       []adminMenuNode,
 *   "tree":            true,
 *   "items":           []adminMenuNode,
 * }
 */
export interface WhoAmIData {
  authenticated: boolean;
  adminUserId: number;
  user: AdminUserView | null;
  permissionKeys: string[];
  menuTree: import('./menu').AdminMenuNode[];
  tree: boolean;
  items: import('./menu').AdminMenuNode[];
}

/** WhoAmI API 的完整响应（isReturnNativeResponse 模式） */
export type WhoAmIResponse = ApiEnvelope<WhoAmIData>;
```

### `src/types/api/user.ts`

```typescript
import type { PageData } from './common';

/**
 * 管理员用户行数据（列表展示用）。
 * 与 AdminUserView 字段一致，增加前端列表所需的额外字段。
 */
export interface AdminUserRow {
  adminUserId: number;
  username: string;
  displayName: string;
  status: number;
  lastLoginAt: string | null;
  lastLoginIp: string;
  createdAt: string;
  updatedAt: string;
}

/** 用户列表查询参数。对应后端 GET /v1/admin/system/user/list 的 query params */
export interface ListUsersParams {
  page: number;
  pageSize: number;
  username?: string;
  status?: number;
}

/** 用户列表 API 返回的 data（分页模式，默认拦截器解包后）。*/
export type ListUsersResponse = PageData<AdminUserRow>;

/**
 * 创建/更新用户请求体。
 * 对应后端 WriteRBAC.UpsertUser (internal/interfaces/http/admin/rbac_write.go)
 *
 * Go JSON tag → toSnakeCase 转换:
 *   username → username
 *   password → password
 *   displayName → display_name
 *   status → status
 */
export interface UpsertUserRequest {
  username: string;
  password?: string;
  displayName?: string;
  status?: number;
}

/** 设置用户状态请求体。对应后端 WriteRBAC.SetUserStatus */
export interface SetUserStatusRequest {
  adminUserId: number;
  status: number;          // 1=启用, 0=禁用
}

/** 用户-角色绑定请求体。对应后端 WriteRBAC.BindUserRole */
export interface BindUserRoleRequest {
  adminUserId: number;
  roleId: number;
}
```

---

## 六、修改优先级与依赖关系

```
阶段 1 (类型基础 + API 泛型)  ← 最高优先，无依赖
    ↓
阶段 2 (组件类型修复)         ← 依赖阶段 1 的类型
    ↓
阶段 3 (Store + Router)       ← 依赖阶段 1、2 的类型
    ↓
阶段 4 (Utils 工具)           ← 独立，可并行
    ↓
阶段 5 (Views 视图)           ← 依赖阶段 1~4 全部完成
    ↓
阶段 6 (注释补全)             ← 贯穿全过程，每阶段完成后立即补注释
```

## 七、文件变更总表

```
文件路径                                      阶段  改动类型  预计行数
════════════════════════════════════════════════════════════════════════════
【新建 - 类型文件】
src/types/api/common.ts                        1     新建      30
src/types/api/auth.ts                          1     新建      50
src/types/api/user.ts                          1     新建      40
src/types/api/role.ts                          1     新建      40
src/types/api/menu.ts                          1     新建      35
src/types/api/permission.ts                    1     新建      35
src/types/api/audit.ts                         1     新建      25
src/types/api/log.ts                           1     新建      25
src/types/api/cluster.ts                       1     新建      80
src/types/api/transcode.ts                     1     新建      40
src/types/api/live.ts                          1     新建      50
src/types/api/config.ts                        1     新建      70
src/types/api/index.ts                         1     新建      15

【修改 - API 文件】
src/api/auth.ts                                1     加泛型     15
src/api/system/user.ts                         1     加泛型     15
src/api/system/role.ts                         1     加泛型     20
src/api/system/menu.ts                         1     加泛型     10
src/api/system/permission.ts                   1     加泛型     10
src/api/system/audit.ts                        1     加泛型      5
src/api/system/log.ts                          1     加泛型      5
src/api/cluster.ts                             1     加泛型     40
src/api/transcode.ts                           1     加泛型     15
src/api/live.ts                                1     加泛型     25
src/api/config.ts                              1     加泛型     45

【修改 - 组件】
src/components/Table/src/types/table.ts         2     去any      15
src/components/Table/src/types/tableAction.ts   2     去any       5
src/components/Table/src/types/pagination.ts    2     去any       3
src/components/Table/src/types/componentType.ts 2     去any       5
src/components/Table/src/Table.vue              2     去any       8
src/components/Table/src/props.ts               2     去any       5
src/components/Table/src/const.ts               2     去any       2
src/components/Table/src/hooks/useColumns.ts    2     去any      10
src/components/Table/src/hooks/useDataSource.ts 2     去any      10
src/components/Table/src/hooks/useLoading.ts    2     去any       2
src/components/Table/src/hooks/usePagination.ts 2     去any       3
src/components/Table/src/components/editable/*  2     去any       5
src/components/Table/src/components/settings/*  2     去any       8
src/components/Form/src/types/form.ts           2     去any       5
src/components/Form/src/hooks/useForm.ts        2     去any       3
src/components/Form/src/hooks/useFormEvents.ts  2     去any       3
src/components/Form/src/BasicForm.vue           2     去any       5
src/components/Upload/src/type/index.ts         2     去any       1

【修改 - Store + Router】
src/store/modules/asyncRoute.ts                3     去any      10
src/store/modules/tabsView.ts                  3     去any       2
src/router/generator.ts                        3     去any      15
src/router/guards.ts                           3     去any       2
src/router/types.ts                            3     去any       1

【修改 - Utils】
src/utils/index.ts                             4     去any      15
src/utils/Storage.ts                           4     去any       5
src/utils/domUtils.ts                          4     去any       3
src/utils/Drag.ts                              4     去any       1

【修改 - Views】
src/views/system/user/user.vue                 5     去any+补全  30
src/views/system/role/role.vue                 5     去any+补全  20
src/views/system/role/CreateModal.vue          5     去any       2
src/views/system/menu/menu.vue                 5     去any+树表格 60
src/views/system/menu/CreateDrawer.vue         5     去any       5
src/views/system/permission/permission.vue     5     去any+树表格 40
src/views/system/audit/audit.vue               5     注释       10
src/views/system/log/log.vue                   5     注释       10
src/views/cluster/nodes.vue                    5     补全       30
src/views/transcode/jobs.vue                   5     补全       25
src/views/live/channels.vue                    5     补全       30
src/views/config/runtime.vue                   5     补全表单   80
src/views/config/versions.vue                  5     补全       10
src/views/dashboard/console/console.vue        5     适配API    20
════════════════════════════════════════════════════════════════════════════
合计: 新建 13 文件 + 修改 45+ 文件，预计新增/修改 ~1200 行
```

---

## 八、注释规范

每个 interface / type / function 必须包含：

```typescript
/**
 * [一句话描述]
 *
 * 对应后端: [Go 结构体名] ([文件路径])
 * JSON 字段映射: snake_case → camelCase
 */
```

每个字段必须包含：

```typescript
interface SomeType {
  /** 字段说明。Go: original_field → TS: tsField */
  tsField: string;
}
```

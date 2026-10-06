# HVC Web 管理后台 — 完整开发计划

> **核心原则**：不改 UI 风格、不改组件封装方式、只做数据对接。
> 所有页面复用 `BasicTable` / `BasicForm` / `BasicModal` / `useForm` / `useModal` 等现有封装。

---

## 一、范围总览

### 1.1 改造规模

| 类别 | 数量 |
|------|------|
| 后端 API 模块 | 11 个 |
| 后端接口端点 | 64 个 |
| 需新建页面 | 14 个页面 |
| 需适配页面 | 4 个页面 (登录/仪表盘/菜单管理/角色管理) |
| 需删除页面 | 18 个演示页 (comp/form/list/result 等) |
| 修改基础设施文件 | 6 个 |
| 新建 API 文件 | 9 个 |
| 新建路由模块 | 11 个 |

### 1.2 保留的现有页面

| 页面 | 路径 | 处理方式 |
|------|------|---------|
| 登录 | `views/login/index.vue` | 适配：表单提交方式改为 x-www-form-urlencoded，移除 token 提取逻辑 |
| 异常页 | `views/exception/*.vue` | 保留不变 |
| 重定向 | `views/redirect/index.vue` | 保留不变 |
| 菜单管理 | `views/system/menu/menu.vue` | 适配：Tree 数据字段映射后端 `adminMenuNode` |
| 角色管理 | `views/system/role/role.vue` | 适配：列表字段改为后端 `adminRoleView` 字段 |

### 1.3 删除的演示页面（18个）

```
views/comp/        (drag/form/modal/richtext/table/upload)  — 组件演示
views/dashboard/   (monitor/monitor.vue, workplace/workplace.vue) — 假仪表盘
views/directive/   (index.vue)                              — 指令演示
views/form/        (basicForm/detail/stepForm)               — 表单演示
views/frame/       (docs.vue)                               — iframe 演示
views/iframe/      (index.vue)                              — iframe 演示
views/list/        (basicList)                              — 列表演示
views/result/      (fail/info/success)                      — 结果页演示
views/about/       (index.vue)                              — 关于页
views/setting/     (account/system)                         — 假设置页
```

---

## 二、基础设施层改造（6 个文件）

### 2.1 HTTP 拦截器 → `src/utils/http/alova/index.ts`

**改动类型**：重写 `responded.onSuccess`，简化 `beforeRequest`

```
变更点                       当前值                          目标值
─────────────────────────────────────────────────────────────────────
成功状态码                    ResultEnum.SUCCESS = 200        ResultEnum.SUCCESS = 0
响应数据字段                  const { code, message, result }  const { code, message, data: result }
Token 注入 (beforeRequest)    headers['token'] = token        删除（Cookie 自动携带）
登录失效码                    code === 912                    code === 401
登录失效处理                  弹窗→清缓存→跳转                 弹窗→跳转（Cookie 已被清除）
```

**改造后的 `onSuccess` 核心逻辑**：

```typescript
responded: {
  onSuccess: async (response, method) => {
    const res = await response.json();

    // 原生响应：login / WhoAmI 自行处理
    if (method.meta?.isReturnNativeResponse) {
      if (res.data && typeof res.data === 'object') {
        res.data = toCamelCase(res.data);
      }
      return res;
    }

    const { message, code, data: result } = res;  // ← result → data

    if (method.meta?.isTransformResponse === false) {
      return res.data;
    }

    if (code === 0) {                               // ← 200 → 0
      if (result && typeof result === 'object') {
        return toCamelCase(result);
      }
      return result;
    }

    if (code === 401) {                             // ← 912 → 401
      Modal?.warning({
        title: '提示',
        content: '登录身份已失效，请重新登录!',
        onOk: () => { window.location.href = PageEnum.BASE_LOGIN; },
      });
    } else {
      Message?.error(message);
      throw new Error(message);
    }
  },
}
```

### 2.2 HTTP 状态枚举 → `src/enums/httpEnum.ts`

```diff
- SUCCESS = 200,
+ SUCCESS = 0,
  ERROR = -1,
- TIMEOUT = 10042,
```

### 2.3 路由生成器 → `src/router/generator.ts`

**改动类型**：重写 `generateRoutes`，直接消费后端 `adminMenuNode` 字段

```typescript
// 重写前：消费前端自定格式 { path, name, component, meta: { title, icon, permissions } }
// 重写后：直接消费后端 adminMenuNode（经 toCamelCase 转换后）

export const generateRoutes = (menuNodes: any[], parent?: any): any[] => {
  return menuNodes
    .filter((node) => node.status === 1)   // 只处理启用的菜单
    .sort((a, b) => (a.sortNo ?? 0) - (b.sortNo ?? 0))
    .map((node) => {
      // menuType: 'menu' | 'button' | 'iframe'
      if (node.menuType === 'button') return null;  // 按钮不生成路由

      const routePath = node.routePath || node.menuKey;
      const fullPath = parent
        ? `${parent.path}/${routePath}`.replace('//', '/')
        : `/${routePath}`.replace('//', '/');

      const currentRoute: any = {
        path: fullPath,
        name: node.menuKey,
        component: undefined,  // 由 asyncImportRoute 动态匹配 views/
        meta: {
          title: node.menuName,
          label: node.menuName,
          icon: constantRouterIcon[node.iconName] || null,
          permissions: node.permissionKey ? [node.permissionKey] : null,
          sort: node.sortNo ?? 0,
          hidden: node.hidden ?? false,
          keepAlive: true,
        },
      };

      // 处理 component_name：后端返回如 "/dashboard/console/console"
      if (node.componentName) {
        currentRoute.component = node.componentName;
      } else if (node.menuType === 'iframe' && node.routePath) {
        currentRoute.component = 'IFRAME';
        currentRoute.meta.frameSrc = node.routePath;
      }

      if (node.children?.length) {
        if (!node.redirect) {
          // 取第一个有效子路由作为 redirect
          const firstChild = node.children.find((c) => c.menuType !== 'button');
          if (firstChild) {
            currentRoute.redirect = `${fullPath}/${firstChild.routePath || firstChild.menuKey}`;
          }
        }
        currentRoute.children = generateRoutes(node.children, currentRoute).filter(Boolean);
      }

      // 如果只有 path 和 name，没有 component，设为 ParentLayout
      if (!currentRoute.component && !currentRoute.children?.length) {
        return null;
      }

      return currentRoute;
    })
    .filter(Boolean);
};
```

`asyncImportRoute` 不变——它继续通过 `import.meta.glob` 匹配 `componentName`（如 `/dashboard/console/console`）到 `views/dashboard/console/console.vue`。

### 2.4 分页字段配置 → `src/settings/componentSetting.ts`

```diff
  table: {
    apiSetting: {
-     pageField: 'page',
+     pageField: 'page',
-     sizeField: 'pageSize',
+     sizeField: 'pageSize',    // toCamelCase 已自动将 page_size → pageSize
-     listField: 'list',
+     listField: 'items',       // 后端返回 "items"
-     totalField: 'pageCount',
+     totalField: 'total',      // 后端返回 "total"（总条数）
-     countField: 'itemCount',
+     countField: 'total',      // 复用 total 作为总数
    },
```

**额外处理**：后端不返回 `pageCount`（总页数），在 `useDataSource.ts` 中新增一行计算：

```typescript
// useDataSource.ts fetch() 中，设置分页时：
const totalPages = Math.ceil(total / pageSize);
setPagination({
  page: currentPage,
  pageCount: totalPages,   // 前端计算
  itemCount: total,
});
```

### 2.5 路由常量 → `src/router/constant.ts`

```diff
- export const ErrorPage = () => import('@/views/exception/404.vue');
+ export const ErrorPage = () => import('@/views/exception/404.vue'); // 不变
```

保留 `Layout`、`ParentLayout`、`ErrorPage` 三个常量。

### 2.6 鉴权 Store → `src/store/modules/user.ts`

**改动类型**：重写 `login` / `getInfo` / `logout`

```typescript
// 改造后的 user store

export const useUserStore = defineStore({
  id: 'app-user',
  state: (): IUserState => ({
    token: '',            // 不再使用，鉴权走 Cookie
    username: '',
    avatar: '',
    permissions: [],      // string[] ← permission_keys
    info: {} as UserInfoType,
  }),
  actions: {
    // 登录
    async login(params: { username: string; password: string }) {
      // 后端接受 x-www-form-urlencoded，Alova 自动处理
      const res = await loginApi(params);
      const { code, data } = res;
      if (code === 0) {
        // Cookie 已由浏览器自动存储，不手动存 token
        // 立即获取用户信息加载权限和菜单
        await this.getInfo();
      }
      return res;
    },

    // 获取当前用户（WhoAmI）
    async getInfo() {
      const res = await whoAmI();  // GET /v1/admin/auth/me
      const { code, data } = res;
      if (code === 0) {
        const { user, permissionKeys } = data;
        this.username = user?.displayName || user?.username || '';
        this.setPermissions(permissionKeys || []);
        this.setUserInfo({
          username: user?.username || '',
          email: '',
        });
        // 返回给路由守卫，包含 menuTree 用于动态路由生成
        return {
          permissions: (permissionKeys || []).map((k: string) => ({ value: k, label: k })),
          menuTree: data.menuTree || data.items || [],
          user,
        };
      }
      throw new Error('获取用户信息失败');
    },

    // 登出
    async logout() {
      await logoutApi();
      this.setPermissions([]);
      this.setUserInfo({ username: '', email: '' });
    },
  },
});
```

---

## 三、后端 API → 前端页面完整映射

### 3.1 认证模块 (Auth)

| 后端端点 | 前端文件 | 操作 |
|---------|---------|------|
| `POST /v1/admin/auth/login` | `api/auth.ts` (新建) | 登录 |
| `POST /v1/admin/auth/logout` | `api/auth.ts` | 登出 |
| `GET /v1/admin/auth/me` | `api/auth.ts` | 获取当前用户+权限+菜单树 |

### 3.2 系统管理 — 用户 (System Users)

| 后端端点 | 权限 | 前端组件 | 操作 |
|---------|------|---------|------|
| `GET /v1/admin/system/user/list` | `system.user.read` | `BasicTable` | 用户列表 |
| `PUT /v1/admin/system/user/upsert` | `system.user.create` | `BasicModal` + `BasicForm` | 新建/编辑用户 |
| `PUT /v1/admin/system/user/status` | `system.user.update` | `TableAction` | 启用/禁用 |
| `PUT /v1/admin/system/user-role/bind` | `system.user.role_bind` | `BasicModal` + Checkbox | 分配角色 |

**新建文件**：`views/system/user/user.vue`、`api/system/user.ts`、`router/modules/system.ts`（含新增路由）

### 3.3 系统管理 — 角色 (System Roles)

| 后端端点 | 权限 | 前端组件 | 操作 |
|---------|------|---------|------|
| `GET /v1/admin/system/role/list` | `system.role.read` | `BasicTable` | 角色列表（**适配现有 2 个 Modal**） |
| `GET /v1/admin/system/role/all` | `system.role.read` | `NSelect options` | 角色下拉（绑定用户时） |
| `PUT /v1/admin/system/role/upsert` | `system.role.update` | `BasicModal` + `BasicForm` | 新建/编辑角色 |
| `PUT /v1/admin/system/role-permission/bind` | `system.role.permission_bind` | `BasicModal` + `NCheckbox` | 绑定权限 |
| `PUT /v1/admin/system/role-menu/assign` | `system.role.menu_bind` | `BasicModal` + `NTree` | 分配菜单 |
| `GET /v1/admin/system/role/menu/tree` | `system.menu.read` | `NTree` 数据源 | 获取角色菜单树 |

**适配文件**：`views/system/role/role.vue` — 重写 `CreateModal` 和 `EditModal` 的 schema

### 3.4 系统管理 — 权限 (System Permissions)

| 后端端点 | 权限 | 前端组件 | 操作 |
|---------|------|---------|------|
| `GET /v1/admin/system/permission/tree` | `system.permission.read` | `NTree` | 权限树展示 |
| `PUT /v1/admin/system/permission/upsert` | `system.role.permission_bind` | `BasicModal` + `BasicForm` | 新建/编辑权限 |

**新建文件**：`views/system/permission/permission.vue`、`api/system/permission.ts`

### 3.5 系统管理 — 菜单 (System Menus)

| 后端端点 | 权限 | 前端组件 | 操作 |
|---------|------|---------|------|
| `GET /v1/admin/system/menu/tree` | `system.menu.read` | `NTree` | 菜单树（管理页） |
| `PUT /v1/admin/system/menu/upsert` | `system.menu.update` | `BasicDrawer` + `BasicForm` | 新建/编辑菜单 |
| `DELETE /v1/admin/system/menu/delete` | `system.menu.delete` | `PopConfirm` | 删除菜单 |

**适配文件**：`views/system/menu/menu.vue` — 字段映射为后端 `adminMenuNode`

### 3.6 系统管理 — 审计日志 (Audit Logs)

| 后端端点 | 权限 | 前端组件 | 操作 |
|---------|------|---------|------|
| `GET /v1/admin/audit/list` | `audit.read` | `BasicTable` | 审计日志列表 |

**新建文件**：`views/system/audit/audit.vue`、`api/system/audit.ts`

### 3.7 系统管理 — 运行日志 (Runtime Logs)

| 后端端点 | 权限 | 前端组件 | 操作 |
|---------|------|---------|------|
| `GET /v1/admin/system/log/list` | `system.log.read` | `BasicTable` | 运行日志列表 |

**新建文件**：`views/system/log/log.vue`、`api/system/log.ts`

### 3.8 集群管理 (Cluster)

| 后端端点 | 权限 | 前端组件 | 操作 |
|---------|------|---------|------|
| `GET /v1/admin/cluster/overview` | `cluster.read` | 自定义 Cards + ECharts | 集群总览 |
| `GET /v1/admin/cluster/realtime` | `cluster.read` | 自定义 Cards | 实时指标 |
| `GET /v1/admin/cluster/topology` | `cluster.read` | 自定义 SVG/Canvas | 拓扑图 |
| `GET /v1/admin/cluster/node/list` | `cluster.node.read` | `BasicTable` | 节点列表 |
| `GET /v1/admin/cluster/node/detail` | `cluster.node.read` | 自定义 Detail 卡片 | 节点详情 |
| `GET /v1/admin/cluster/node/metrics` | `cluster.node.metrics.read` | ECharts | 节点指标 |
| `PUT /v1/admin/cluster/node/enabled` | `cluster.node.enable` | `TableAction` Switch | 启用/禁用节点 |
| `PUT /v1/admin/cluster/node/quarantined` | `cluster.node.quarantine` | `TableAction` Switch | 隔离/解除节点 |
| `PUT /v1/admin/cluster/node/draining` | `cluster.node.drain` | `TableAction` Button | 节点排水 |
| `GET /v1/admin/cluster/worker/list` | `cluster.read` | `BasicTable` | Worker 列表 |
| `PUT /v1/admin/cluster/worker/offline` | `cluster.worker.offline` | `TableAction` Button | Worker 下线 |
| `PUT /v1/admin/cluster/worker/exit` | `cluster.worker.exit` | `TableAction` Button | Worker 退出 |
| `GET /v1/admin/cluster/member/list` | `cluster.read` | `BasicTable` | 成员列表 |
| `GET /v1/admin/cluster/scheduler/insight` | `cluster.read` | 自定义 Card + Charts | 调度器洞察 |
| `PUT /v1/admin/cluster/job/takeover` | `cluster.job.takeover` | `BasicModal` + Form | 任务接管 |
| `GET /v1/admin/cluster/resource/distribution` | `cluster.read` | ECharts | 资源分布 |

**新建文件**：
- `views/cluster/overview.vue` — 集群总览仪表盘
- `views/cluster/nodes.vue` — 节点管理
- `views/cluster/workers.vue` — Worker 管理
- `views/cluster/members.vue` — 成员管理
- `views/cluster/scheduler.vue` — 调度器洞察
- `views/cluster/topology.vue` — 拓扑图
- `api/cluster.ts`
- `router/modules/cluster.ts`

### 3.9 转码管理 (Transcode)

| 后端端点 | 权限 | 前端组件 | 操作 |
|---------|------|---------|------|
| `GET /v1/admin/transcode/job/list` | `transcode.job.read` | `BasicTable` | 转码任务列表 |
| `GET /v1/admin/transcode/job/detail` | `transcode.job.detail.read` | 自定义 Detail 面板 | 任务详情 |
| `GET /v1/admin/transcode/job/progress` | `transcode.job.read` | `NProgress` + 时间线 | 任务进度 |
| `PUT /v1/admin/transcode/job/retry` | `transcode.job.retry` | `TableAction` Button | 重试 |
| `PUT /v1/admin/transcode/job/cancel` | `transcode.job.cancel` | `TableAction` Button + PopConfirm | 取消 |

**新建文件**：`views/transcode/jobs.vue`、`api/transcode.ts`、`router/modules/transcode.ts`

### 3.10 直播管理 (Live)

| 后端端点 | 权限 | 前端组件 | 操作 |
|---------|------|---------|------|
| `GET /v1/admin/live/channel/list` | `live.channel.read` | `BasicTable` | 频道列表 |
| `GET /v1/admin/live/channel/detail` | `live.channel.read` | 自定义 Detail 面板 | 频道详情 |
| `PUT /v1/admin/live/channel/create` | `live.channel.create` | `BasicModal` + `BasicForm` | 创建频道 |
| `PUT /v1/admin/live/channel/update` | `live.channel.update` | `BasicModal` + `BasicForm` | 更新频道 |
| `PUT /v1/admin/live/channel/start` | `live.channel.start` | `TableAction` Button | 开播 |
| `PUT /v1/admin/live/channel/stop` | `live.channel.stop` | `TableAction` Button + PopConfirm | 停播 |
| `DELETE /v1/admin/live/channel/delete` | `live.channel.delete` | `TableAction` Button + PopConfirm | 删除 |
| `GET /v1/admin/live/session/list` | `live.session.read` | `BasicTable` | 会话列表 |

**新建文件**：`views/live/channels.vue`、`api/live.ts`、`router/modules/live.ts`

### 3.11 配置管理 (Config)

#### 运行配置 (Runtime Config)

| 后端端点 | 权限 | 前端组件 | 说明 |
|---------|------|---------|------|
| `GET /v1/admin/config/runtime/versions` | `config.version.read` | `BasicTable` | 版本列表 |
| `PUT /v1/admin/config/publish` | `config.version.publish` | `BasicModal` + Confirm | 发布配置 |
| `PUT /v1/admin/config/runtime/server/update` | `config.runtime.update` | `BasicForm` | Server 配置 |
| `PUT /v1/admin/config/runtime/scheduler/update` | `config.runtime.update` | `BasicForm` | Scheduler 配置 |
| `PUT /v1/admin/config/runtime/worker/update` | `config.runtime.update` | `BasicForm` | Worker 配置 |
| `PUT /v1/admin/config/runtime/storage/update` | `config.runtime.update` | `BasicForm` | Storage 配置 |
| `PUT /v1/admin/config/runtime/callback/update` | `config.runtime.update` | `BasicForm` | Callback 配置 |
| `PUT /v1/admin/config/runtime/mq/update` | `config.runtime.update` | `BasicForm` | MQ 配置 |
| `PUT /v1/admin/config/runtime/grpc/update` | `config.runtime.update` | `BasicForm` | gRPC 配置 |

#### 回调配置 (Callback Config)

| 后端端点 | 权限 | 前端组件 |
|---------|------|---------|
| `GET /v1/admin/config/callback/list` | `config.callback.read` | `BasicTable` |
| `PUT /v1/admin/config/callback/upsert` | `config.callback.update` | `BasicModal` + `BasicForm` |
| `PUT /v1/admin/config/callback/enabled` | `config.callback.update` | `TableAction` Switch |

#### 命名模板 (Naming Template)

| 后端端点 | 权限 | 前端组件 |
|---------|------|---------|
| `GET /v1/admin/config/naming-template/list` | `config.naming_template.read` | `BasicTable` |
| `PUT /v1/admin/config/naming-template/configure` | `config.naming_template.update` | `BasicModal` + `BasicForm` |
| `PUT /v1/admin/config/naming-template/activate` | `config.naming_template.update` | `TableAction` Button |

#### 配置中心 (Config Center)

| 后端端点 | 权限 | 前端组件 |
|---------|------|---------|
| `GET /v1/admin/config-center/list` | `config.version.read` | `BasicTable` |
| `PUT /v1/admin/config-center/upsert` | `config.version.publish` | `BasicModal` + `BasicForm` |
| `PUT /v1/admin/config-center/enabled` | `config.version.publish` | `TableAction` Switch |

#### Etcd 注册中心

| 后端端点 | 权限 | 前端组件 |
|---------|------|---------|
| `GET /v1/admin/registry/etcd/list` | `config.version.read` | `BasicTable` |
| `PUT /v1/admin/registry/etcd/upsert` | `config.runtime.update` | `BasicModal` + `BasicForm` |
| `PUT /v1/admin/registry/etcd/enabled` | `config.runtime.update` | `TableAction` Switch |

**新建文件**：
- `views/config/runtime.vue` — 运行配置 Tab 页（子 Tab: Server/Scheduler/Worker/Storage/Callback/MQ/gRPC）
- `views/config/versions.vue` — 版本管理 + 发布
- `views/config/callback.vue` — 回调配置
- `views/config/namingTemplate.vue` — 命名模板
- `views/config/configCenter.vue` — 配置中心
- `views/config/registryEtcd.vue` — Etcd 注册中心
- `api/config.ts`
- `router/modules/config.ts`

### 3.12 仪表盘

| 用途 | 后端端点 | 前端组件 | 说明 |
|------|---------|---------|------|
| 首页总览 | `GET /v1/admin/cluster/overview` | Cards + ECharts | 集群概览卡 + 实时指标 |

**适配文件**：`views/dashboard/console/console.vue` — 替换假数据为真实 API

### 3.13 路由模块规划

| 菜单 | 路由模块文件 | 对应 views |
|------|------------|-----------|
| 仪表盘 | `router/modules/dashboard.ts` | dashboard/console |
| 系统管理 | `router/modules/system.ts` | system/user, system/role, system/menu, system/permission, system/audit, system/log |
| 集群管理 | `router/modules/cluster.ts` | cluster/overview, cluster/nodes, cluster/workers, cluster/members, cluster/scheduler, cluster/topology |
| 转码管理 | `router/modules/transcode.ts` | transcode/jobs |
| 直播管理 | `router/modules/live.ts` | live/channels |
| 配置管理 | `router/modules/config.ts` | config/runtime, config/versions, config/callback, config/namingTemplate, config/configCenter, config/registryEtcd |

---

## 四、页面详细设计

### 4.1 登录页 → 适配 `views/login/index.vue`

**现状**：表单提交 JSON `{ username, password }`，从响应取 `result.token` 存 localStorage。

**改造**：
```typescript
// api/auth.ts
export function loginApi(params: { username: string; password: string }) {
  return Alova.Post('/v1/admin/auth/login', params, {
    meta: {
      isReturnNativeResponse: true,      // 自行处理 { code, data }
      isTransformResponse: false,
    },
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded',
    },
    // Alova 需配置将 params 转为 form body
  });
}
```

**UI 保持不变**：Naive UI 登录卡片 + 背景图。

### 4.2 仪表盘 → 适配 `views/dashboard/console/console.vue`

**现状**：假数据 Cards（访问量/销售额/下载量）+ ECharts 趋势图。

**改造**：调用 `GET /v1/admin/cluster/overview` 获取真实数据，展示：
- 节点总数 / 在线节点 / Worker 数 / 任务数 卡片
- 集群 CPU/内存资源使用趋势（ECharts，复用 `useECharts`）
- 保留现有 VisiTab / FluxTrend 子组件结构，只替换数据源

### 4.3 用户管理 → 新建 `views/system/user/user.vue`

**使用组件**：`BasicTable` + `TableAction` + `BasicModal` + `BasicForm`

**页面布局**：
```
┌──────────────────────────────────────────────┐
│  [ 搜索栏 (BasicForm, inline)               ]│
│  [ 用户名 ] [ 状态 ]  [ 查询 ] [ 重置 ]      │
├──────────────────────────────────────────────┤
│  [ + 新建用户 ]                               │
│  BasicTable                                  │
│  ┌──────┬───────┬──────┬──────┬──────┐      │
│  │ 用户名 │ 显示名 │ 状态  │ 上次登录│ 操作  │      │
│  │ admin │ Admin │ 启用  │ 05-12 │ [编辑]│      │
│  │       │       │       │       │ [禁用]│      │
│  │       │       │       │       │ [分配角色]│  │
│  └──────┴───────┴──────┴──────┴──────┘      │
└──────────────────────────────────────────────┘
```

**新建用户 Modal**：
```typescript
const schemas = [
  { field: 'username', label: '用户名', component: 'NInput', rules: [{ required: true }] },
  { field: 'password', label: '密码', component: 'NInput', componentProps: { type: 'password' } },
  { field: 'display_name', label: '显示名', component: 'NInput' },
  { field: 'status', label: '状态', component: 'NSwitch', defaultValue: 1 },
];
```

### 4.4 角色管理 → 适配 `views/system/role/role.vue`

**现有效果保留**：Table + 新建 Modal + 编辑 Modal + 菜单权限分配 Modal。

**字段映射**：
```
create_date → createdAt（toCamelCase 自动转换）
isDefault → isDefault
menu_keys → 改为调 GET /v1/admin/system/role/menu/tree?role_id=xxx
```

### 4.5 菜单管理 → 适配 `views/system/menu/menu.vue`

**保留**：Tree 展示 + Drawer 编辑。

**字段映射**：
```typescript
// 后端 adminMenuNode（toCamelCase 后）
{
  menuId, parentId, menuKey, menuName, routePath, componentName,
  iconName, menuType, permissionKey, sortNo, hidden, status, children
}
```

**Drawer 表单 schema**：
```typescript
const schemas = [
  { field: 'menuKey', label: '菜单标识', component: 'NInput', rules: [{ required: true }] },
  { field: 'menuName', label: '菜单名称', component: 'NInput', rules: [{ required: true }] },
  { field: 'parentId', label: '父级菜单', component: 'NTreeSelect' },
  { field: 'menuType', label: '类型', component: 'NSelect',
    componentProps: { options: [{ label: '菜单', value: 'menu' }, { label: '按钮', value: 'button' }, { label: '外链', value: 'iframe' }] }
  },
  { field: 'routePath', label: '路由路径', component: 'NInput' },
  { field: 'componentName', label: '组件路径', component: 'NInput', ifShow: (s) => s.menuType === 'menu' },
  { field: 'iconName', label: '图标名', component: 'NInput' },
  { field: 'permissionKey', label: '权限标识', component: 'NInput' },
  { field: 'sortNo', label: '排序', component: 'NInputNumber' },
  { field: 'hidden', label: '隐藏', component: 'NSwitch' },
  { field: 'status', label: '启用', component: 'NSwitch', defaultValue: 1 },
];
```

### 4.6 权限管理 → 新建 `views/system/permission/permission.vue`

**使用组件**：`NTree` + `BasicModal` + `BasicForm`

**页面布局**：
```
┌──────────────────────────────────────────────┐
│  权限树（NTree）          │  新建权限 (Modal)  │
│  ├── auth                │                    │
│  │   ├── auth.session    │                    │
│  │   └── ...             │                    │
│  ├── system              │                    │
│  │   ├── system.user     │                    │
│  │   └── ...             │                    │
│  └── cluster             │                    │
│      └── ...             │                    │
└──────────────────────────────────────────────┘
```

### 4.7 集群节点 → 新建 `views/cluster/nodes.vue`

**使用组件**：`BasicTable` + `TableAction`

**列定义**：
```typescript
columns: [
  { title: '节点ID', key: 'nodeId', width: 80 },
  { title: '节点名称', key: 'nodeName' },
  { title: '角色', key: 'role' },
  { title: '状态', key: 'status', render: (row) => h(NTag, { type: statusColor(row.status) }) },
  { title: 'IP', key: 'ip' },
  { title: 'CPU', key: 'cpuUsage' },
  { title: '内存', key: 'memoryUsage' },
  { title: 'Worker数', key: 'workerCount' },
  { title: '操作', key: 'action', render: renderNodeActions },
]
```

### 4.8 转码任务 → 新建 `views/transcode/jobs.vue`

**使用组件**：`BasicTable` + `TableAction`

**操作按钮**：
```typescript
actions: [
  { label: '详情', onClick: handleDetail },
  { label: '进度', onClick: handleProgress },
  { label: '重试', auth: ['transcode.job.retry'], onClick: handleRetry },
  { label: '取消', auth: ['transcode.job.cancel'], popConfirm: { ... } },
]
```

### 4.9 直播频道 → 新建 `views/live/channels.vue`

**使用组件**：`BasicTable` + `TableAction` + `BasicModal` + `BasicForm`

**操作按钮**：
```typescript
actions: [
  { label: '详情', onClick: handleDetail },
  { label: '开始', auth: ['live.channel.start'], onClick: handleStart },
  { label: '停止', auth: ['live.channel.stop'], popConfirm: { ... } },
  { label: '删除', auth: ['live.channel.delete'], popConfirm: { ... } },
]
```

### 4.10 运行配置 → 新建 `views/config/runtime.vue`

**使用组件**：`NTabs` + `BasicForm`（每个 Tab 一个 Form）

**Tab 结构**：
```
[ Server | Scheduler | Worker | Storage | Callback | MQ | gRPC ]
```

每个 Tab 内的 Form 根据后端返回的配置结构动态生成 schema。首次加载调用 `GET /v1/admin/config/runtime/versions` 获取最新配置快照。

### 4.11 配置页面统一模式

配置管理（Runtime/Callback/NamingTemplate/ConfigCenter/Etcd）的 5 个子页全部遵循：

```
┌──────────────────────────────────────────────┐
│  [ 新建 ]                                     │
│  BasicTable (带 Switch 开关列)                 │
│  ┌────┬───────┬──────┬──────┬──────┐        │
│  │ ID │ 名称   │ 状态  │ 更新时间│ 操作  │        │
│  │    │       │ ✓    │ ...   │ [编辑]│        │
│  └────┴───────┴──────┴──────┴──────┘        │
└──────────────────────────────────────────────┘
```

---

## 五、Store 改造设计

### 5.1 修改的 Store

| Store | 改动 |
|-------|------|
| `user.ts` | 重写 `login`/`getInfo`/`logout`，Token 改 Cookie，`getInfo` 返回 `menuTree` |
| `asyncRoute.ts` | `generateRoutes` 改为直接调 `generator.ts` 的 `generateRoutes(menuTree)` |
| `projectSetting.ts` | `permissionMode` 默认改为 `'BACK'` |
| `screenLock.ts` | 不变 |
| `tabsView.ts` | 不变 |
| `designSetting.ts` | 不变 |

### 5.2 `asyncRoute.ts` 改造

```typescript
async generateRoutes(data) {
  const { permissionMode } = useProjectSetting();
  let accessedRouters;

  if (unref(permissionMode) === 'BACK') {
    // 直接消费后端返回的 menuTree
    const menuTree = data.menuTree || [];
    accessedRouters = generateRoutes(menuTree);
    asyncImportRoute(accessedRouters);
  } else {
    // FIXED 模式保留（开发期降级）
    const permissionsList = data.permissions ?? [];
    accessedRouters = filter(asyncRoutes, (route) => {
      const { permissions } = route.meta || {};
      if (!permissions) return true;
      return permissionsList.some((item) => permissions.includes(item));
    });
  }

  this.setRouters(accessedRouters);
  this.setMenus(accessedRouters);
  return toRaw(accessedRouters);
}
```

---

## 六、路由菜单设计（后端菜单种子数据）

后端 `CurrentMenuTree` 返回的菜单树结构，对应前端导航。建议在数据库初始化时写入以下菜单数据作为种子：

```yaml
# 一级菜单
- menu_key: dashboard
  menu_name: 仪表盘
  route_path: dashboard
  component_name: /dashboard/console/console
  icon_name: DashboardOutlined
  menu_type: menu
  sort_no: 0

- menu_key: system
  menu_name: 系统管理
  route_path: system
  icon_name: SettingOutlined
  menu_type: menu
  sort_no: 1
  children:
    - menu_key: system_user
      menu_name: 用户管理
      route_path: user
      component_name: /system/user/user
      permission_key: system.user.read
    - menu_key: system_role
      menu_name: 角色管理
      route_path: role
      component_name: /system/role/role
      permission_key: system.role.read
    - menu_key: system_menu
      menu_name: 菜单管理
      route_path: menu
      component_name: /system/menu/menu
      permission_key: system.menu.read
    - menu_key: system_permission
      menu_name: 权限管理
      route_path: permission
      component_name: /system/permission/permission
      permission_key: system.permission.read
    - menu_key: system_audit
      menu_name: 审计日志
      route_path: audit
      component_name: /system/audit/audit
      permission_key: audit.read
    - menu_key: system_log
      menu_name: 运行日志
      route_path: log
      component_name: /system/log/log
      permission_key: system.log.read

- menu_key: cluster
  menu_name: 集群管理
  route_path: cluster
  icon_name: CloudServerOutlined
  menu_type: menu
  sort_no: 2
  children:
    - menu_key: cluster_overview
      menu_name: 集群总览
      route_path: overview
      component_name: /cluster/overview
      permission_key: cluster.read
    - menu_key: cluster_nodes
      menu_name: 节点管理
      route_path: nodes
      component_name: /cluster/nodes
      permission_key: cluster.node.read
    - menu_key: cluster_workers
      menu_name: Worker 管理
      route_path: workers
      component_name: /cluster/workers
      permission_key: cluster.read
    - menu_key: cluster_members
      menu_name: 成员管理
      route_path: members
      component_name: /cluster/members
      permission_key: cluster.read
    - menu_key: cluster_scheduler
      menu_name: 调度洞察
      route_path: scheduler
      component_name: /cluster/scheduler
      permission_key: cluster.read
    - menu_key: cluster_topology
      menu_name: 拓扑图
      route_path: topology
      component_name: /cluster/topology
      permission_key: cluster.read

- menu_key: transcode
  menu_name: 转码管理
  route_path: transcode
  icon_name: VideoCameraOutlined
  menu_type: menu
  sort_no: 3
  children:
    - menu_key: transcode_jobs
      menu_name: 转码任务
      route_path: jobs
      component_name: /transcode/jobs
      permission_key: transcode.job.read

- menu_key: live
  menu_name: 直播管理
  route_path: live
  icon_name: LiveStreamOutlined
  menu_type: menu
  sort_no: 4
  children:
    - menu_key: live_channels
      menu_name: 频道管理
      route_path: channels
      component_name: /live/channels
      permission_key: live.channel.read

- menu_key: config
  menu_name: 配置管理
  route_path: config
  icon_name: SettingsOutlined
  menu_type: menu
  sort_no: 5
  children:
    - menu_key: config_runtime
      menu_name: 运行配置
      route_path: runtime
      component_name: /config/runtime
      permission_key: config.version.read
    - menu_key: config_versions
      menu_name: 版本管理
      route_path: versions
      component_name: /config/versions
      permission_key: config.version.read
    - menu_key: config_callback
      menu_name: 回调配置
      route_path: callback
      component_name: /config/callback
      permission_key: config.callback.read
    - menu_key: config_naming_template
      menu_name: 命名模板
      route_path: naming-template
      component_name: /config/namingTemplate
      permission_key: config.naming_template.read
    - menu_key: config_center
      menu_name: 配置中心
      route_path: config-center
      component_name: /config/configCenter
      permission_key: config.version.read
    - menu_key: registry_etcd
      menu_name: Etcd 注册中心
      route_path: etcd
      component_name: /config/registryEtcd
      permission_key: config.version.read
```

---

## 七、文件变更清单

### 7.1 修改的现有文件（6 个）

```
src/utils/http/alova/index.ts              # 重写拦截器（code: 0, data 字段, Cookie 鉴权）
src/enums/httpEnum.ts                       # SUCCESS = 0
src/router/generator.ts                     # 重写 generateRoutes（直接消费 adminMenuNode）
src/settings/componentSetting.ts            # 分页字段：list→items, total→total
src/settings/projectSetting.ts              # permissionMode → 'BACK'
src/store/modules/user.ts                   # 重写 login/getInfo/logout
```

### 7.2 适配的现有页面（4 个）

```
src/views/login/index.vue                   # 表单提交方式，移除 token 逻辑
src/views/dashboard/console/console.vue     # 假数据 → 真实 API
src/views/system/role/role.vue              # 字段映射 + API URL
src/views/system/menu/menu.vue              # 字段映射 + API URL
```

### 7.3 新建的 API 文件（9 个）

```
src/api/auth.ts                             # login, logout, whoAmI
src/api/system/user.ts                      # listUsers, upsertUser, setUserStatus, bindUserRole
src/api/system/permission.ts                # listPermissions, permissionTree, upsertPermission
src/api/system/audit.ts                     # listAuditLogs
src/api/system/log.ts                       # listRuntimeLogs
src/api/cluster.ts                          # overview, nodes, workers, members, scheduler, topology...
src/api/transcode.ts                        # listJobs, jobDetail, jobProgress, retryJob, cancelJob
src/api/live.ts                             # channel CRUD + sessions
src/api/config.ts                           # runtime, callback, namingTemplate, configCenter, etcd
```

### 7.4 新建的路由模块（11 个）

```
src/router/modules/dashboard.ts             # 重写（仅保留 console）
src/router/modules/system.ts                # 重写（user/role/menu/permission/audit/log）
src/router/modules/cluster.ts               # 新建（overview/nodes/workers/members/scheduler/topology）
src/router/modules/transcode.ts             # 新建（jobs）
src/router/modules/live.ts                  # 新建（channels）
src/router/modules/config.ts                # 新建（runtime/versions/callback/namingTemplate/configCenter/etcd）
```

### 7.5 新建的页面文件（14 个）

```
src/views/system/user/user.vue
src/views/system/permission/permission.vue
src/views/system/audit/audit.vue
src/views/system/log/log.vue
src/views/cluster/overview.vue
src/views/cluster/nodes.vue
src/views/cluster/workers.vue
src/views/cluster/members.vue
src/views/cluster/scheduler.vue
src/views/cluster/topology.vue
src/views/transcode/jobs.vue
src/views/live/channels.vue
src/views/config/runtime.vue
src/views/config/versions.vue
src/views/config/callback.vue
src/views/config/namingTemplate.vue
src/views/config/configCenter.vue
src/views/config/registryEtcd.vue
```

### 7.6 删除的文件（约 30 个）

```
src/router/modules/about.ts          # 演示页路由
src/router/modules/comp.ts           # 演示页路由
src/router/modules/directive.ts      # 演示页路由
src/router/modules/docs.ts           # 演示页路由
src/router/modules/form.ts           # 演示页路由
src/router/modules/frame.ts          # 演示页路由
src/router/modules/list.ts           # 演示页路由
src/router/modules/result.ts         # 演示页路由
src/router/modules/setting.ts        # 演示页路由
src/router/modules/newVersion.ts     # 演示页路由
src/views/about/**                   # 关于页
src/views/comp/**                    # 组件演示
src/views/directive/**               # 指令演示
src/views/form/**                    # 表单演示
src/views/frame/**                   # iframe 演示
src/views/iframe/**                  # iframe 演示
src/views/list/**                    # 列表演示
src/views/result/**                  # 结果演示
src/views/setting/**                 # 设置演示
src/views/dashboard/monitor/**       # 假监控页
src/views/dashboard/workplace/**     # 假工作台
src/mock/**                          # 全部 mock 文件
src/utils/http/alova/mocks.ts        # mock 入口
```

---

## 八、开发顺序

按依赖关系和主链路优先级，分 4 个阶段：

### 阶段 0：环境准备（0.5 天）

1. 关闭 Mock：`.env.development` → `VITE_USE_MOCK = false`
2. 配置代理：`.env.development` → `VITE_PROXY = [["/v1","http://localhost:8888"]]`
3. 配置 API 前缀：`.env.development` → `VITE_GLOB_API_URL_PREFIX = /v1/admin`
4. 验证后端可连通：`GET /v1/admin/ping`

### 阶段 1：基础设施（1 天）

1. 改 `enums/httpEnum.ts` → SUCCESS = 0
2. 改 `utils/http/alova/index.ts` → 拦截器（code/data/Cookie）
3. 改 `settings/componentSetting.ts` → 分页字段
4. 改 `settings/projectSetting.ts` → permissionMode = 'BACK'
5. 改 `store/modules/user.ts` → login/getInfo/logout
6. 改 `store/modules/asyncRoute.ts` → generateRoutes
7. 改 `router/generator.ts` → 重写 generateRoutes
8. 新建 `api/auth.ts` → login/logout/whoAmI

**验证**：登录 → 自动获取菜单 → 侧边栏出现菜单 → 跳转到仪表盘

### 阶段 2：系统管理（1.5 天）

1. 适配 `views/login/index.vue`
2. 新建 `views/system/user/user.vue` + `api/system/user.ts`
3. 适配 `views/system/role/role.vue`
4. 适配 `views/system/menu/menu.vue`
5. 新建 `views/system/permission/permission.vue` + `api/system/permission.ts`
6. 新建 `views/system/audit/audit.vue` + `api/system/audit.ts`
7. 新建 `views/system/log/log.vue` + `api/system/log.ts`
8. 重写 `router/modules/system.ts`

### 阶段 3：核心业务（2 天）

1. 适配 `views/dashboard/console/console.vue` → 集群概览数据
2. 新建集群模块：overview / nodes / workers / members / scheduler / topology
3. 新建转码模块：jobs
4. 新建直播模块：channels
5. 新建配置模块：runtime / versions / callback / namingTemplate / configCenter / etcd

### 阶段 4：清理 & 验证（0.5 天）

1. 删除 30 个演示文件
2. 删除全部 mock 文件
3. 统一验证登录→菜单→各页面数据加载
4. 验证权限控制（菜单可见性 + 按钮权限 + 接口权限）

---

## 九、开发规范

1. **所有新页面必须使用现有封装组件**：`BasicTable`（列表）、`BasicForm`（表单）、`BasicModal`（弹窗）、`TableAction`（操作栏）
2. **API 文件命名**：`src/api/业务模块/功能.ts`
3. **views 文件命名**：`src/views/业务模块/功能/功能.vue`，文件名用 camelCase 以匹配 component_name
4. **路由 name**：使用 `menuKey` 作为路由 name（全局唯一）
5. **权限判断**：使用 `v-permission` 指令 + `usePermission().hasPermission()`
6. **toCamelCase 已自动转换**：所有响应数据的键名已是驼峰格式，无需手动写 `menu_key` → `menuKey`
7. **图标映射**：在 `router/icons.ts` 中新增 `CloudServerOutlined`、`VideoCameraOutlined`、`LiveStreamOutlined` 等

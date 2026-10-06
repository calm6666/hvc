# HVC-Web 项目架构分析与二次开发指南

## 一、项目概述

`hvc-web` 是基于开源项目 **Naive Ui Admin v2.1.0** 二次开发的企业级中后台前端解决方案，底层技术栈为 **Vue 3.x + Vite + Naive UI + TypeScript**。

- **开源上游**: https://github.com/jekip/naive-ui-admin
- **UI 框架**: [Naive UI](https://www.naiveui.com/) (Vue 3 原生组件库)
- **定位**: 中后台管理系统前端模板，包含二次封装组件、动态菜单、权限校验、多主题、响应式布局等开箱即用功能

---

## 二、技术栈

| 类别 | 核心依赖 | 版本 |
|------|---------|------|
| 框架 | Vue 3 (Composition API) | ^3.5.21 |
| 构建 | Vite | ^5.4.20 |
| 语言 | TypeScript | ^4.9.5 |
| UI | Naive UI | ^2.43.1 |
| 状态管理 | Pinia | ^2.3.1 |
| 路由 | Vue Router | ^4.5.1 |
| HTTP | Alova (带 mock 适配器) | ^3.3.4 |
| 图表 | ECharts | ^5.6.0 |
| 工具 | @vueuse/core, lodash-es, dayjs, date-fns | - |
| CSS | Tailwind CSS + Less + PostCSS | - |
| 富文本 | @vueup/vue-quill | ^1.2.0 |
| 拖拽 | vuedraggable | ^4.1.0 |

---

## 三、项目目录结构

```
web/
├── build/                       # 构建脚本
│   ├── constant.ts              # 构建常量 (OUTPUT_DIR等)
│   ├── getConfigFileName.ts     # 获取配置文件名
│   ├── utils.ts                 # env 解析、环境判断工具
│   ├── script/
│   │   ├── buildConf.ts         # 构建配置生成
│   │   └── postBuild.ts         # 构建后处理
│   └── vite/
│       ├── plugin/              # Vite 插件集
│       │   ├── index.ts         # 插件入口(注册vue/jsx/naive/compress/html)
│       │   ├── html.ts          # HTML 注入插件
│       │   └── compress.ts      # 压缩插件(gzip/brotli)
│       └── proxy.ts             # 开发代理配置
├── mock/                        # Mock 数据 (开发期)
│   ├── _util.ts                 # Mock 工具函数
│   ├── dashboard/console.ts     # 仪表盘 mock
│   ├── system/menu.ts           # 菜单 mock
│   ├── system/role.ts           # 角色 mock
│   ├── table/list.ts            # 表格列表 mock
│   └── user/                    # 用户 mock
├── public/                      # 静态资源 (直接复制)
├── src/
│   ├── api/                     # API 接口层
│   │   ├── dashboard/console.ts
│   │   ├── system/
│   │   │   ├── menu.ts          # 菜单 API
│   │   │   ├── role.ts          # 角色 API
│   │   │   └── user.ts          # 用户 API (登录/信息/登出)
│   │   └── table/list.ts
│   ├── assets/                  # 静态资源 (SVG/PNG)
│   ├── components/              # 二次封装组件 ★核心★
│   │   ├── Application/         # 全局应用包裹组件(Provider层)
│   │   ├── CountTo/             # 数字滚动组件
│   │   ├── Form/                # 动态表单组件 ★重要★
│   │   ├── Lockscreen/          # 锁屏组件
│   │   ├── Modal/               # 模态框封装
│   │   ├── Table/               # 表格封装 ★核心★
│   │   └── Upload/              # 上传封装
│   ├── config/
│   │   └── website.config.ts    # 网站静态配置(Logo/标题)
│   ├── directives/              # 自定义指令
│   │   ├── clickOutside.ts      # 点击外部
│   │   ├── copy.ts              # 剪贴板复制
│   │   ├── debounce.ts          # 防抖
│   │   ├── draggable.ts         # 拖拽
│   │   ├── longpress.ts         # 长按
│   │   ├── permission.ts        # 权限控制指令
│   │   └── throttle.ts          # 节流
│   ├── enums/                   # 枚举常量
│   │   ├── breakpointEnum.ts    # 响应式断点
│   │   ├── cacheEnum.ts         # 缓存键
│   │   ├── httpEnum.ts          # HTTP 状态码/方法
│   │   ├── pageEnum.ts          # 页面路径枚举
│   │   ├── permissionsEnum.ts   # 权限枚举
│   │   └── roleEnum.ts          # 角色枚举
│   ├── hooks/                   # 组合式函数 (Composables)
│   │   ├── core/useTimeout.ts   # 超时 hook
│   │   ├── event/               # 事件类 hook
│   │   │   ├── useBreakpoint.ts
│   │   │   ├── useEventListener.ts
│   │   │   └── useWindowSizeFn.ts
│   │   ├── setting/             # 配置读取 hook
│   │   │   ├── index.ts         # useGlobSetting / useLocalSetting
│   │   │   ├── useDesignSetting.ts
│   │   │   └── useProjectSetting.ts
│   │   ├── web/                 # 业务 hook
│   │   │   ├── useECharts.ts    # 图表
│   │   │   ├── usePage.ts       # 页面级逻辑
│   │   │   └── usePermission.ts # 权限判断
│   │   ├── useAsync.ts
│   │   ├── useBattery.ts        # 电池状态
│   │   ├── useDomWidth.ts
│   │   ├── useOnline.ts         # 网络状态
│   │   └── useTime.ts           # 时间展示
│   ├── layout/                  # 布局系统 ★核心★
│   │   ├── index.vue            # 主布局(侧边栏/Header/内容区)
│   │   ├── parentLayout.vue     # 父级路由占位
│   │   └── components/
│   │       ├── Footer/          # 页脚
│   │       ├── Header/          # 顶栏(项目配置/用户信息)
│   │       ├── Logo/            # Logo
│   │       ├── Main/            # 主内容区(含路由动画)
│   │       ├── Menu/            # 菜单(递归渲染/动态菜单)
│   │       └── TagsView/        # 标签页
│   ├── plugins/                 # 应用插件
│   │   ├── index.ts             # 统一导出
│   │   ├── naive.ts             # Naive UI 全局注册
│   │   ├── naiveDiscreteApi.ts  # Naive 脱离上下文的API(Message等)
│   │   ├── directives.ts        # 全局指令注册
│   │   ├── customComponents.ts  # 全局组件注册(预留)
│   │   └── globalMethods.ts     # 全局方法注册(预留)
│   ├── router/                  # 路由 ★核心★
│   │   ├── index.ts             # 路由实例创建+动态导入模块
│   │   ├── base.ts              # 基础路由(404/重定向)
│   │   ├── constant.ts          # 路由常量(Layout/ParentLayout引入)
│   │   ├── generator.ts         # 动态路由生成器(后端菜单→路由表)
│   │   ├── guards.ts            # 路由守卫(鉴权+动态路由注册)
│   │   ├── icons.ts             # 路由图标映射表
│   │   ├── types.ts             # 路由类型定义
│   │   └── modules/             # 路由模块(按业务分文件)
│   ├── settings/                # 系统预设配置
│   │   ├── animateSetting.ts    # 动画预设
│   │   ├── componentSetting.ts  # 组件预设(表格分页/上传限制)
│   │   ├── designSetting.ts     # 主题预设(主题色列表)
│   │   └── projectSetting.ts    # 项目预设(导航模式/权限模式等)
│   ├── store/                   # Pinia 状态管理
│   │   ├── index.ts             # store 创建
│   │   ├── mutation-types.ts    # localStorage 键名常量
│   │   ├── types.ts             # 全局 store 类型
│   │   └── modules/
│   │       ├── asyncRoute.ts    # 动态路由 store
│   │       ├── designSetting.ts # 主题状态
│   │       ├── projectSetting.ts# 项目配置状态
│   │       ├── screenLock.ts    # 锁屏状态
│   │       ├── tabsView.ts      # 标签页状态
│   │       └── user.ts          # 用户/鉴权状态
│   ├── styles/                  # 全局样式
│   │   ├── index.less
│   │   ├── tailwind.css
│   │   └── transition/          # 过渡动画(CSS)
│   ├── utils/                   # 工具函数
│   │   ├── index.ts             # renderIcon/deepMerge/lighten等
│   │   ├── Storage.ts           # localStorage/sessionStorage封装(含过期)
│   │   ├── caseConverter.ts     # 驼峰/下划线键名互转工具 ★新增★
│   │   ├── Drag.ts              # 模态框拖拽
│   │   ├── browser-type.ts
│   │   ├── dateUtil.ts
│   │   ├── domUtils.ts
│   │   ├── downloadFile.ts
│   │   ├── env.ts               # 环境变量读取
│   │   ├── http/alova/          # Alova HTTP 实例
│   │   │   ├── index.ts         # 核心实例(拦截器/响应/错误处理)
│   │   │   └── mocks.ts         # Mock 接口定义
│   │   ├── is/                  # 类型判断工具
│   │   ├── lib/echarts.ts
│   │   ├── lodashChunk.ts
│   │   ├── log.ts
│   │   ├── propTypes.ts         # Vue 增强的 propTypes
│   │   └── urlUtils.ts
│   └── views/                   # 页面视图 ★业务开发核心区★
│       ├── about/
│       ├── comp/                # 组件示例页
│       ├── dashboard/           # 仪表盘
│       ├── directive/           # 指令示例
│       ├── exception/           # 异常页(403/404/500)
│       ├── form/                # 表单示例
│       ├── frame/               # 外链 iframe
│       ├── iframe/
│       ├── list/                # 列表示例
│       ├── login/               # 登录页
│       ├── redirect/            # 重定向页
│       ├── result/              # 结果页
│       ├── setting/             # 设置页(账户/系统)
│       └── system/              # 系统管理(菜单/角色)
├── types/                       # 全局类型定义 (.d.ts)
├── .env / .env.development / .env.production  # 环境变量
├── vite.config.ts               # Vite 配置
├── tsconfig.json
├── tailwind.config.js
└── package.json
```

---

## 四、核心模块深度分析

### 4.1 应用启动流程 (main.ts)

```
main.ts → bootstrap()
  ├── createApp(App)
  ├── setupStore(app)          # Pinia 状态管理挂载
  ├── setupNaive(app)          # 注册全局 Naive UI 组件
  ├── setupNaiveDiscreteApi()  # 注册脱离上下文的API(Message/Dialog/Notification/LoadingBar)
  ├── setupDirectives(app)     # 注册全局指令(v-permission/v-copy等)
  ├── setupRouter(app)         # 路由挂载+守卫注册
  └── await router.isReady()   # 路由就绪后 mount
```

**App.vue** 是根组件，核心职责：
- 提供 `NConfigProvider` 主题配置（高亮色/暗色模式）
- 嵌套 `AppProvider`（Message/Dialog/Notification 的 Provider 层）
- 实现屏幕锁屏倒计时逻辑（监听全局 mouse 事件）

### 4.2 路由系统

路由系统是这个项目最精妙的部分，实现了**前端固定路由**和**后端动态路由**两种模式。

#### 4.2.1 路由注册流程

```
router/index.ts
  ├── import.meta.glob('./modules/**/*.ts')    # 动态导入所有路由模块
  ├── routeModuleList (合并并排序所有模块路由)
  ├── constantRouter = [LoginRoute, RootRoute, RedirectRoute]  # 基础路由
  └── asyncRoutes = routeModuleList                             # 需要鉴权的路由

router/guards.ts
  router.beforeEach:
    1. 白名单检查 (login)
    2. Token 检查 (无 token → redirect login)
    3. 动态路由是否已添加
       - 未添加: 调 userStore.getInfo() → asyncRouteStore.generateRoutes()
       - router.addRoute() 逐个注册
       - 最后注册 404 捕获路由

router/generator.ts
  generateDynamicRoutes():
    1. 调后端接口 adminMenus() 获取菜单树
    2. generateRoutes() 递归生成路由配置
    3. asyncImportRoute() 通过 import.meta.glob 匹配views下的组件文件
```

#### 4.2.2 路由模块写法

每个路由模块在 `router/modules/` 下按业务分文件:

```typescript
// router/modules/system.ts
export default [
  {
    path: '/system',
    name: 'System',
    component: 'LAYOUT',        // 特殊字符串，映射到 Layout 组件
    meta: { title: '系统管理', icon: 'DashboardOutlined', sort: 0 },
    children: [
      {
        path: 'menu',
        name: 'MenuManagement',
        component: '/system/menu/menu',  // 对应 views/system/menu/menu.vue
        meta: { title: '菜单管理', permissions: ['MenuManagement'] }
      }
    ]
  }
]
```

#### 4.2.3 关键设计模式

- **component 映射**: 通过 import.meta.glob 动态匹配 views 目录下的 .vue/.tsx 文件，字符串 `"/system/menu/menu"` 自动匹配到 `views/system/menu/menu.vue`
- **LayoutMap**: `{ LAYOUT → Layout组件, IFRAME → Iframe组件 }`，通过特殊字符串区分布局组件
- **权限过滤**: `usePermission.hasPermission()` 在路由生成期和组件渲染期双重守卫

### 4.3 状态管理 (Pinia)

#### 4.3.1 User Store

```typescript
// store/modules/user.ts
state: { token, username, avatar, permissions[], info }
actions: {
  login(params)    // 登录 → 存 token/用户信息到 localStorage(7天过期)
  getInfo()        // 获取用户信息和权限列表
  logout()         // 清空 token/权限
}
```

#### 4.3.2 AsyncRoute Store (核心)

```typescript
// store/modules/asyncRoute.ts
state: { menus[], routers[], keepAliveComponents[], isDynamicRouteAdded }
actions: {
  generateRoutes(data):
    - permissionMode === 'BACK'  → generateDynamicRoutes() 调后端接口获取菜单
    - permissionMode === 'FIXED' → filter(asyncRoutes) 从前端路由筛选权限
    - setRouters / setMenus
}
```

两种权限模式通过 `projectSetting.permissionMode` 切换：

| 模式 | 说明 | 使用场景 |
|------|------|---------|
| `FIXED` | 前端固定路由 + 权限过滤 | 纯前端开发/演示 |
| `BACK` | 后端动态返回菜单树 | 对接真实后端 |

#### 4.3.3 TabsView Store

管理多标签页状态：`tabsList` 数组，支持关闭左侧/右侧/其他/全部，保留 `affix` 固定标签。

### 4.4 HTTP 请求层 (Alova)

项目使用 **Alova** (而非 Axios) 作为 HTTP 客户端：

```
utils/http/alova/index.ts
  └── createAlova({
        baseURL, statesHook: VueHook,
        requestAdapter: mockAdapter,      # mock 适配器(开发模式)
        beforeRequest: 添加 token + url 前缀
        responded.onSuccess: 统一响应处理
          - code === 200 → return result
          - code === 912 → 弹窗提示登录失效 → 清除缓存 → 跳转 login
          - 其他 → message.error → throw Error
      })
```

**Alova vs Axios 的区别**: Alova 是请求策略库，自动管理请求状态（loading/error/data），支持缓存、分页等策略。但这个项目主要把它当作普通 HTTP 客户端使用。

#### API 层调用范式

```typescript
// api/system/user.ts
import { Alova } from '@/utils/http/alova/index';

export function getUserInfo() {
  return Alova.Get<InResult>('/admin_info', {
    meta: { isReturnNativeResponse: true }  // 返回原始响应(不自动解包)
  });
}
```

### 4.5 驼峰/下划线键名互转工具 (caseConverter.ts) ★新增★

**文件位置**: `src/utils/caseConverter.ts`

这是解决前后端字段命名风格不一致（前端驼峰 `userId` ↔ 后端下划线 `user_id`）的统一转换工具。设计目标：高性能、零内存泄漏、完整 TypeScript 类型推断。

#### 分层架构

```
第一部分: 核心字符串转换（纯函数）
  snakeToCamel("user_id")   → "userId"
  camelToSnake("userId")    → "user_id"

第二部分: 递归对象转换核心
  transformKeys(obj, fn) → 深度遍历对象/数组/基本类型

第三部分: TypeScript 类型工具
  CamelCaseKeys<T> / SnakeCaseKeys<T> → 编译期类型推断

第四部分: 公开 API
  toCamelCase(obj)  → 下划线 → 驼峰（返回强类型新对象）
  toSnakeCase(obj)  → 驼峰 → 下划线（返回强类型新对象）

第五部分: 请求级缓存（可选优化）
  KeyConverter 类 → 单次请求内字段名缓存，请求结束自动 GC
```

#### 核心实现要点

**1. 预编译正则（性能优化）**
```typescript
const SNAKE_TO_CAMEL_RE = /_([a-z])/g;    // 匹配 "_小写字母"
const CAMEL_TO_SNAKE_RE = /([A-Z])/g;     // 匹配任意大写字母
const LEADING_UNDERSCORE_RE = /^_/;       // 处理 "_id" 开头下划线
```
正则对象在模块加载时只创建一次，避免每次调用重新编译。

**2. 递归转换核心（无内存泄漏）**
```typescript
function transformKeys<T>(obj: T, transformKey: KeyTransformer): unknown {
  if (obj === null || typeof obj !== 'object') return obj;   // 基本类型直接返回
  if (Array.isArray(obj)) {                                   // 数组：预分配长度 + for 循环
    const newArr = new Array(obj.length);
    for (let i = 0; i < obj.length; i++) {
      newArr[i] = transformKeys(obj[i], transformKey);
    }
    return newArr;
  }
  // 普通对象：for 循环遍历自有属性
  const newObj: Record<string, unknown> = {};
  const keys = Object.keys(obj);
  for (let i = 0; i < keys.length; i++) {
    newObj[transformKey(keys[i])] = transformKeys(obj[keys[i]], transformKey);
  }
  return newObj;
}
```
- 使用 `for` 循环而非 `forEach/map`，减少函数调用栈开销
- 无任何全局缓存，保证内存零增长
- 不改变原对象（不可变数据）

**3. TypeScript 类型推断（递归模板字面量）**
```typescript
type CamelCase<S extends string> = S extends `${infer Head}_${infer Tail}`
  ? `${Head}${Capitalize<CamelCase<Tail>>}`
  : S;

type SnakeCase<S extends string> = S extends `${infer Head}${infer Tail}`
  ? Head extends Uppercase<Head>
    ? `_${Lowercase<Head>}${SnakeCase<Tail>}`
    : `${Head}${SnakeCase<Tail>}`
  : S;
```

这意味着转换后 IDE 能自动推断出正确的字段名：
```typescript
const apiData = { user_id: 1, created_at: "2024" };
const result = toCamelCase(apiData);
// result 类型为: { userId: number; createdAt: string }
// IDE 会自动补全 result.userId ✓
```

#### 使用场景

**场景 1：API 响应数据 → 前端数据（下划线 → 驼峰）**
```typescript
import { toCamelCase } from '@/utils/caseConverter';

// 在 HTTP 拦截器中使用
responded: {
  onSuccess: async (response) => {
    const res = await response.json();
    return toCamelCase(res.result);  // 后端下划线 → 前端驼峰
  }
}
```

**场景 2：前端数据 → API 请求参数（驼峰 → 下划线）**
```typescript
import { toSnakeCase } from '@/utils/caseConverter';

// 在 HTTP 拦截器 beforeRequest 中使用
beforeRequest(method) {
  if (method.data) {
    method.data = toSnakeCase(method.data);  // 前端驼峰 → 后端下划线
  }
}
```

**场景 3：批量数据处理（使用 KeyConverter 缓存加速）**
```typescript
import { KeyConverter } from '@/utils/caseConverter';

const converter = new KeyConverter();
const result = largeDataList.map(item => converter.toCamelCase(item));
// converter 使用完毕，失去引用后自动 GC，无需手动清理
```

#### 两种 API 对比

| 维度 | `toCamelCase` / `toSnakeCase` | `KeyConverter` 类 |
|------|-------------------------------|-------------------|
| 缓存 | 无（每次重新转换） | 实例级缓存 Map |
| 适用场景 | 小数据量、字段不重复 | 批量数据、字段高度重复 |
| 内存 | 零额外占用 | 实例存在期间持有缓存 |
| 类型推断 | ✅ 完整 | ✅ 完整 |

---

### 4.6 布局系统

#### 4.5.1 主布局 (layout/index.vue)

```
┌─────────────────────────────────────────┐
│  NLayout (has-sider)                    │
│  ┌──────────┬──────────────────────────┐│
│  │ Sider    │ Header (PageHeader)      ││
│  │ (Logo)   ├──────────────────────────┤│
│  │ (Menu)   │ TabsView (标签页)         ││
│  │          ├──────────────────────────┤│
│  │          │ MainView (router-view)   ││
│  │          │ + keep-alive + transition ││
│  └──────────┴──────────────────────────┘│
└─────────────────────────────────────────┘
```

关键特性：
- **响应式**: 通过监听 `resize` 事件，窗口宽度 < 800px 切换为移动端抽屉模式
- **多布局模式**: `navMode` 支持 `vertical`(侧边栏) / `horizontal`(顶栏) / `horizontal-mix`(混合)
- **菜单收缩**: 最小宽度 64px，展开宽度 200px
- **固定定位**: header/menu/tabs 均可通过配置 fixed
- **KeepAlive**: 路由 meta 中 `keepAlive: true` 的页面会被 `<keep-alive>` 缓存

#### 4.5.2 Menu 组件

支持垂直和水平两种模式。通过 `generatorMenu()` 函数将路由数据转换为 NMenu 的 options 格式，支持递归无限层级。

### 4.7 二次封装组件详解 ★最重要★

#### 4.6.1 BasicTable (表格封装)

**文件位置**: `src/components/Table/`

**架构设计**:
```
Table.vue (模板+脚本) ── 组合 hooks 实现逻辑
  ├── useLoading      # loading 状态
  ├── useColumns      # 列配置(过滤/排序/权限/编辑列)
  ├── useDataSource   # 数据加载(请求/分页/错误处理)
  ├── usePagination   # 分页状态
  └── useTableContext # provide/inject 上下文

子组件:
  ├── TableAction.vue       # 操作按钮组(含权限过滤/下拉菜单)
  ├── EditableCell.vue      # 可编辑单元格
  ├── ColumnSetting.vue     # 列设置(显示/隐藏/排序)
  └── CellComponent.ts      # 动态组件渲染器
```

**核心 Props**:
```typescript
{
  title: string,                 // 表格标题
  columns: BasicColumn[],        // 列配置(必填)
  request: Function,             // API 请求函数(必填)
  dataSource: [],                // 静态数据(与request互斥)
  rowKey: string | Function,     // 行唯一键
  pagination: Object | Boolean,  // 分页配置(false关闭)
  beforeRequest: Function,       // 请求前钩子(可修改参数)
  afterRequest: Function,        // 请求后钩子(可修改数据)
  canResize: Boolean,            // 是否自动计算高度
  striped: Boolean,              // 斑马纹
}
```

**核心 expose 方法** (通过 template ref 或 useTableContext):
```typescript
{
  reload(opt?),     // 重新加载数据
  setColumns,       // 设置列
  getColumns,       // 获取列
  setLoading,       // 控制 loading
  setProps,         // 动态设置 props
  getDataSource,    // 获取当前数据
  getPageColumns,   // 获取显示列
  getCacheColumns,  // 获取缓存列
  setCacheColumnsField,  // 更新缓存列字段
  emit              // 事件发射器
}
```

**数据加载流程**:
```
1. onMounted → setTimeout(fetch, 16)
2. fetch():
   ├── setLoading(true)
   ├── 组装分页参数 { page, pageSize }
   ├── beforeRequest(params) [可选钩子]
   ├── request(params) → await API
   ├── afterRequest(data) [可选钩子]
   ├── dataSourceRef.value = data
   ├── setPagination({ page, pageCount, itemCount })
   ├── emit('fetch-success', { items, resultTotal })
   ├── setLoading(false)
```

**可编辑表格特性**:
- `EditableCell.vue` 支持行内编辑，通过 `v-click-outside` 指令控制编辑态退出
- 支持多种组件: NInput, NSelect, NSwitch, NCheckbox, NDatePicker, NTimePicker 等
- 通过 `componentMap` 统一管理组件类型和事件映射
- 支持行编辑模式（整行同时编辑）和单元格编辑模式

**使用示例**:
```vue
<template>
  <BasicTable @register="registerTable" />
</template>
<script setup>
import { BasicTable } from '@/components/Table';
const [registerTable, tableMethods] = useTable({
  columns: [
    { title: '名称', key: 'name' },
    { title: '操作', key: 'action', render: (row) => h(TableAction, { actions: [...] }) }
  ],
  request: async (params) => await getList(params),
});
</script>
```

#### 4.6.2 BasicForm (表单封装)

**文件位置**: `src/components/Form/`

**架构设计**:
```
BasicForm.vue
  ├── 动态渲染: 根据 schema.component 渲染 NInput/NSelect/... 等
  ├── Grid 布局: NGrid + NGi 实现响应式栅格
  ├── 内联模式: layout='inline' 支持展开/收起
  ├── useFormEvents:  验证/提交/重置/获取值
  ├── useFormValues:  初始默认值/表单值处理
  └── useFormContext:  provide/inject 上下文
```

**核心 Schema 定义**:
```typescript
interface FormSchema {
  field: string;            // 字段名(对应 formModel 的 key)
  label: string;            // 标签
  component?: ComponentType; // 组件类型: 'NInput'|'NSelect'|'NSwitch'|...
  componentProps?: object;  // 组件属性
  defaultValue?: any;       // 默认值
  rules?: object[];         // 校验规则
  slot?: string;            // 自定义插槽名
  giProps?: GridItemProps;  // 栅格项属性(控制每行几个)
}
```

**useForm hook 范式**:
```typescript
// 在页面中使用
const [registerForm, formMethods] = useForm({
  schemas: [
    { field: 'name', label: '名称', component: 'NInput' },
    { field: 'status', label: '状态', component: 'NSelect',
      componentProps: { options: [...] } }
  ],
  labelWidth: 100,
  showSubmitButton: true,
  submitFunc: async () => { await formMethods.validate(); }
});
```

#### 4.6.3 BasicModal (模态框封装)

核心功能: 支持拖拽（通过 header 的 cursor-move + Drag.ts），通过 `useModal` hook 外部控制：

```typescript
const [registerModal, modalMethods] = useModal({ title: '编辑' });
// modalMethods: { openModal(), closeModal(), setProps(), setSubLoading() }
```

#### 4.6.4 BasicUpload (上传封装)

基于 Naive UI 的 `NUpload`，增加了: 图片预览/删除/类型校验/大小校验/图片列表管理。

#### 4.6.5 Application 组件

Provider 层级包裹：`NDialogProvider → NNotificationProvider → NMessageProvider`，确保全局消息弹窗能正常渲染。

#### 4.6.6 CountTo 组件

数字滚动动画组件，从 0 滚动到目标数字。

### 4.8 Hooks 系统

| Hook | 用途 |
|------|------|
| `useForm(props)` | 表单实例管理，返回 `[register, methods]` |
| `useModal(props)` | 模态框实例管理，返回 `[register, methods]` |
| `usePermission()` | 权限判断 `hasPermission / hasEveryPermission / hasSomePermission` |
| `useProjectSetting()` | 读取项目配置（导航模式/标签/面包屑等） |
| `useDesignSetting()` | 读取主题配置（暗色/主题色） |
| `useGlobSetting()` | 读取环境变量（API地址/上传地址等） |
| `useECharts()` | ECharts 图表实例管理 |
| `useBreakpoint()` | 响应式断点监听 |
| `useWindowSizeFn()` | 窗口大小变化防抖回调 |
| `useEventListener()` | 事件监听封装（自动清理） |
| `useTimeout()` | setTimeout 封装 |
| `useOnline()` | 网络在线状态 |
| `useBattery()` | 设备电池状态 |
| `useTime()` | 实时时间展示 |

### 4.9 自定义指令

| 指令 | 用法 | 功能 |
|------|------|------|
| `v-permission` | `v-permission="{ action: ['xxx'] }"` | 无权限时 disable 或 remove 元素 |
| `v-copy` | `v-copy="text"` | 点击复制到剪贴板 |
| `v-debounce` | `v-debounce="fn"` | 按钮 500ms 防抖 |
| `v-throttle` | `v-throttle="fn"` | 按钮 1s 节流 |
| `v-draggable` | `v-draggable` | 元素自由拖拽 |
| `v-clickOutside` | `v-clickOutside="fn"` | 点击外部触发回调 |
| `v-longpress` | `v-longpress="fn"` | 长按 1s 触发 |

### 4.10 插件系统

```
plugins/
  ├── naive.ts             # 按需注册 50+ Naive UI 组件
  ├── naiveDiscreteApi.ts  # 创建 Message/Dialog/Notification/LoadingBar 挂到 window
  ├── directives.ts        # 注册 5 个全局指令
  ├── customComponents.ts  # (预留) 全局组件注册
  └── globalMethods.ts     # (预留) 全局方法注册
```

### 4.11 构建系统

**Vite 配置要点**:
- 路径别名: `@ → src/`, `/# → types/`
- 分包策略: 每个大依赖独立 chunk (vue/pinia/echarts/naive-ui/lodash-es...)
- 开发代理: 通过 `.env.development` 的 `VITE_PROXY` 配置
- 构建压缩: 可选 gzip/brotli
- 包分析: `pnpm run report` 生成分析报告

**构建环境**:
- `.env` — 公共变量 (PORT, APP_TITLE)
- `.env.development` — 开发变量 (mock, proxy, API地址)
- `.env.production` — 生产变量 (压缩, API地址)

---

## 五、组件封装设计模式总结

### 5.1 register/useXxx 模式

这是项目最核心的组件调用范式，用于**外部控制组件实例**：

```typescript
// Form 组件
const [registerForm, formMethods] = useForm(props);
// registerForm: 传给 <BasicForm @register="registerForm" />
// formMethods: { submit(), validate(), resetFields(), getFieldsValue(), ... }

// Modal 组件
const [registerModal, modalMethods] = useModal(props);
// modalMethods: { openModal(), closeModal(), setProps() }

// Table 组件 (略有不同，通过 useTableContext)
const table = useTableContext();
```

**设计意图**: 将组件的控制权暴露给父组件，父组件可以在 JS 代码中操作组件，而不仅仅通过模板绑定。这是 Vue 3 Composition API 下的最佳实践。

### 5.2 Props 继承模式

组件通过 `...NDataTable.props` 等继承原 UI 组件的所有 props，然后扩展自己的：

```typescript
export const basicProps = {
  ...NDataTable.props,  // 继承所有 Naive UI Table 的属性
  title: { type: String, default: null },    // 扩展
  request: { type: Function, ... },           // 扩展
};
```

### 5.3 provide/inject 上下文模式

组件树跨层级通信用 `provide(key, instance)` / `inject(key)`:

```typescript
// Table 组件
createTableContext(instance);   // 提供
useTableContext();              // 子组件注入

// Form 组件
createFormContext(instance);
useFormContext();
```

### 5.4 动态组件映射模式 (Table 的 componentMap)

通过 `Map<ComponentType, Component>` 维护组件类型到 Vue 组件的映射，支持运行时动态渲染不同组件，也支持 `add/del` 扩展。

### 5.5 全局 API 挂载模式

Naive UI 的 Message/Dialog/Notification/LoadingBar 需要通过 `createDiscreteApi` 在 setup 外部创建。项目将它们挂到 `window` 对象上方便使用。

---

## 六、二次开发指南

### 6.1 添加新页面

**第一步**: 在 `views/` 下创建页面组件

```
src/views/live/room/room.vue
```

**第二步**: 在 `api/` 下创建对应 API

```typescript
// src/api/live/room.ts
import { Alova } from '@/utils/http/alova/index';

export function getRoomList(params) {
  return Alova.Get('/live/room/list', { params });
}
```

**第三步**: (FIXED 权限模式) 添加路由模块

```typescript
// src/router/modules/live.ts
export default [
  {
    path: '/live',
    name: 'Live',
    component: 'LAYOUT',
    meta: { title: '直播管理', icon: 'DashboardOutlined', sort: 2 },
    children: [
      {
        path: 'room',
        name: 'LiveRoom',
        component: '/live/room/room',  // 自动匹配 views/live/room/room.vue
        meta: { title: '直播间管理', permissions: ['LiveRoom'] }
      }
    ]
  }
];
```

注意: 文件命名必须是 `.vue` 且在 `views/` 目录下，动态导入才会自动匹配。

**第四步**: (可选) 配置 mock 数据

```typescript
// mock/live/room.ts
export default [
  {
    url: '/api/live/room/list',
    method: 'GET',
    response: () => ({ code: 200, result: { list: [...], pageCount: 1 } })
  }
];

// 在 utils/http/alova/mocks.ts 中注册
import liveRoomMock from '@/../mock/live/room';
```

### 6.2 使用 BasicTable 开发列表页

```vue
<template>
  <BasicTable @register="registerTable">
    <template #toolbar>
      <n-button type="primary" @click="handleAdd">新增</n-button>
    </template>
  </BasicTable>
</template>

<script lang="ts" setup>
import { BasicTable, TableAction } from '@/components/Table';
import { getRoomList } from '@/api/live/room';
import { h } from 'vue';

const columns = [
  { title: 'ID', key: 'id', width: 80 },
  { title: '房间名', key: 'name' },
  { title: '状态', key: 'status' },
  {
    title: '操作', key: 'action', width: 200,
    render: (row) => h(TableAction, {
      actions: [
        { label: '编辑', type: 'primary', onClick: () => handleEdit(row) },
        { label: '删除', type: 'error', popConfirm: { title: '确认删除?', confirm: () => handleDelete(row) } }
      ]
    })
  }
];

const [registerTable, { reload }] = useTable({
  columns,
  request: async (params) => await getRoomList(params),
});
</script>
```

### 6.3 使用 BasicForm 开发表单页

```vue
<template>
  <BasicForm @register="registerForm" @submit="handleSubmit" />
</template>

<script lang="ts" setup>
import { BasicForm, useForm } from '@/components/Form';

const schemas = [
  { field: 'name', label: '房间名', component: 'NInput', rules: [{ required: true }] },
  { field: 'type', label: '类型', component: 'NSelect',
    componentProps: { options: [{ label: '推流', value: 1 }, { label: '拉流', value: 2 }] }
  },
  { field: 'enabled', label: '启用', component: 'NSwitch', defaultValue: true },
];

const [registerForm, { getFieldsValue, validate }] = useForm({ schemas });

async function handleSubmit(values) {
  // 处理提交
}
</script>
```

### 6.4 权限控制

**按钮级**:
```vue
<n-button v-permission="{ action: ['LiveRoomEdit'] }">编辑</n-button>
```

**代码级**:
```typescript
import { usePermission } from '@/hooks/web/usePermission';
const { hasPermission } = usePermission();
if (hasPermission(['LiveRoomEdit'])) { ... }
```

**路由级**: 在路由 meta 中设置 `permissions: ['LiveRoom']`，生成动态路由时自动过滤。

### 6.5 对接真实后端需要修改的地方

按照内存中的规则，**启动只管必需项，业务开关全走后台动态配置**，需要修改以下地方：

1. **环境变量**: `.env.development` 中的 `VITE_USE_MOCK = false`
2. **API 地址**: 修改 `VITE_GLOB_API_URL` 指向真实后端
3. **登录接口**: 修改 `api/system/user.ts` 中 `login()` 和 `getUserInfo()` 的 URL 和响应结构
4. **菜单接口**: `api/system/menu.ts` 中 `adminMenus()` 的 URL
5. **Token 字段**: `store/modules/user.ts` 中 `response.result.token` 根据后端结构调整
6. **HTTP 拦截器**: `utils/http/alova/index.ts` 中的 `responded.onSuccess` 根据后端响应结构调整 code/message/result 字段
7. **权限模式**: `settings/projectSetting.ts` 中 `permissionMode: 'BACK'` 改为后端动态菜单模式
8. **代理配置**: `.env.development` 中 `VITE_PROXY` 配置跨域代理

### 6.6 初始化前端路由图标映射

当后端返回菜单数据中的 `icon` 字段需要映射到前端图标组件时，在 `src/router/icons.ts` 中添加：

```typescript
import { LiveStreamOutlined } from '@vicons/antd';  // 或 @vicons/ionicons5
export const constantRouterIcon = {
  DashboardOutlined: renderIcon(DashboardOutlined),
  LiveRoom: renderIcon(VideoCameraOutlined),  // 新增
};
```

---

## 七、数据流全链路图

```
用户操作 → 页面组件
              ↓
         useForm/useTable (hook 封装)
              ↓
         API 层 (api/xxx.ts)
              ↓
         Alova 实例 (beforeRequest 注入 token/前缀)
              ↓  ←── Mock 适配器 (开发阶段拦截)
         后端 API
              ↓
         responded.onSuccess (统一解析响应)
              ↓
         页面接收数据 → store 或 组件 state
              ↓
         UI 渲染 (Naive UI 组件)
```

**路由鉴权链路**:
```
页面加载
  → router.beforeEach
  → Token 检查 (storage.get)
  → 无 token → redirect login
  → 有 token → userStore.getInfo()
  → asyncRouteStore.generateRoutes()
    → FIXED 模式: 前端路由 filter(权限)
    → BACK 模式: 后端 API adminMenus() → 转路由表
  → router.addRoute() 逐个注册
  → isDynamicRouteAdded = true
  → 放行 next()
```

---

## 八、最佳实践建议

1. **页面组件命名**: 按照 `views/业务模块/功能/功能.vue` 的规范，保证动态路由组件匹配
2. **API 文件**: 按业务模块分文件，函数以 `get/post/delete/update` 前缀命名
3. **类型定义**: 在 `types/` 下定义全局类型，页面内类型就近定义
4. **async await**: 异步请求使用 async/await 而非 .then()
5. **表格操作栏**: 优先使用 `TableAction` 组件，自带权限过滤和 PopConfirm
6. **表单验证**: 在 schema 的 `rules` 中配置，或通过 `editRule` 函数自定义
7. **移动端适配**: 布局会自动检测宽度切换，页面内可以使用 `useBreakpoint` 判断
8. **主题色定制**: 修改 `settings/designSetting.ts` 中的 `appTheme` 和 `appThemeList`
9. **全局配置**: `projectSetting.ts` 控制导航模式、标签页、面包屑、页脚等的显隐
10. **Mock 数据**: 仅在开发环境使用，对接后端时设置 `VITE_USE_MOCK = false`

---

## 九、关键文件速查表

| 功能 | 文件路径 |
|------|---------|
| 应用入口 | `src/main.ts` |
| 根组件 | `src/App.vue` |
| 路由主文件 | `src/router/index.ts` |
| 路由守卫 | `src/router/guards.ts` |
| 动态路由生成 | `src/router/generator.ts` |
| 路由图标映射 | `src/router/icons.ts` |
| 用户 Store | `src/store/modules/user.ts` |
| 动态路由 Store | `src/store/modules/asyncRoute.ts` |
| HTTP 实例 | `src/utils/http/alova/index.ts` |
| 表格组件 | `src/components/Table/src/Table.vue` |
| 表格 Hook | `src/components/Table/src/hooks/` |
| 表单组件 | `src/components/Form/src/BasicForm.vue` |
| 表单 Hook | `src/components/Form/src/hooks/useForm.ts` |
| 模态框组件 | `src/components/Modal/src/basicModal.vue` |
| 模态框 Hook | `src/components/Modal/src/hooks/useModal.ts` |
| 上传组件 | `src/components/Upload/src/BasicUpload.vue` |
| 布局组件 | `src/layout/index.vue` |
| 菜单组件 | `src/layout/components/Menu/index.vue` |
| 权限 Hook | `src/hooks/web/usePermission.ts` |
| 全局指令 | `src/directives/permission.ts` |
| 环境变量读取 | `src/hooks/setting/index.ts` |
| 项目主题配置 | `src/settings/designSetting.ts` |
| 项目功能配置 | `src/settings/projectSetting.ts` |
| 组件预设配置 | `src/settings/componentSetting.ts` |
| 全局工具函数 | `src/utils/index.ts` |
| 驼峰/下划线转换 | `src/utils/caseConverter.ts` |
| 缓存工具类 | `src/utils/Storage.ts` |
| Naive UI 组件注册 | `src/plugins/naive.ts` |
| 脱离上下文的 API | `src/plugins/naiveDiscreteApi.ts` |
| Vite 配置 | `vite.config.ts` |
| 环境变量 | `.env.development` / `.env.production` |

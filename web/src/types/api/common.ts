/**
 * HVC 后端 API 通用类型定义。
 *
 * 所有后端 API 统一使用 model.Response 作为响应信封：
 *   Go:  model.Response{ Code int; Message string; Data interface{} }
 *   JSON: { "code": 0, "message": "ok", "data": { ... } }
 *
 * 前端 HTTP 拦截器（src/utils/http/alova/index.ts）已完成响应解包：
 *   - 默认模式：自动提取 res.data 并 toCamelCase 后返回给调用方
 *   - isReturnNativeResponse 模式：返回完整 { code, message, data } 信封
 */

/**
 * 后端统一响应信封。
 * 对应后端: model.Response (internal/model/system.go)
 *
 * @param T — data 字段的实际类型
 *
 * code 语义:
 *   0   — 成功
 *   401 — 未登录或 session 过期
 *   400 — 请求参数错误
 *   500 — 服务端内部错误
 */
export interface ApiEnvelope<T = unknown> {
  /** 业务状态码。0=成功，非 0=错误 */
  code: number;
  /** 提示信息 */
  message: string;
  /** 业务数据（经 toCamelCase 转换后） */
  data: T;
}

/**
 * 分页响应 data 结构。
 * 对应后端: writePageResponse (internal/interfaces/http/admin/page.go)
 *
 * 后端 writePageResponse 返回:
 *   { "page": 1, "page_size": 10, "total": 60, "items": [...] }
 *
 * 经 HTTP 拦截器 toCamelCase 转换后:
 *   { "page": 1, "pageSize": 10, "total": 60, "items": [...] }
 *
 * 【重要】total 字段是总条数，不是总页数。
 * 总页数由前端 useDataSource 计算: Math.ceil(total / pageSize)
 *
 * @param T — items 数组的元素类型
 */
export interface PageData<T> {
  /** 当前页码 */
  page: number;
  /** 每页条数（Go: page_size → TS: pageSize） */
  pageSize: number;
  /** 总条数（Go: total） */
  total: number;
  /** 数据列表 */
  items: T[];
}

/**
 * 非分页列表响应 data 结构。
 * 对应后端: writeItemsResponse (internal/interfaces/http/admin/page.go)
 *
 * 后端 writeItemsResponse 返回:
 *   { "items": [...], "tree": true, ...其他元数据 }
 *
 * 经 toCamelCase 后:
 *   { items: T[]; [metaKey: string]: unknown }
 *
 * @param T — items 数组的元素类型
 */
export interface ItemsData<T> {
  /** 数据列表 */
  items: T[];
  /** 扩展元数据（如 tree: true, roleId, menuIds 等） */
  [metaKey: string]: unknown;
}

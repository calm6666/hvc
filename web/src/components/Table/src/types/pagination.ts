/**
 * 分页配置接口。
 * 字段与后端 writePageResponse 返回的 data 结构对齐（经 toCamelCase 后）。
 */
export interface PaginationProps {
  /** 当前页码 */
  page?: number;
  /** 总条数 */
  itemCount?: number;
  /** 总页数（后端不返回，由前端 Math.ceil(total / pageSize) 计算） */
  pageCount?: number;
  /** 每页条数 */
  pageSize?: number;
  /** 可选的每页条数集合 */
  pageSizes?: number[];
  /** 是否显示每页条数选择器 */
  showSizePicker?: boolean;
  /** 是否显示快速跳转 */
  showQuickJumper?: boolean;
  /**
   * 分页前缀渲染函数。
   * 接收 { itemCount } 参数，返回如 "共 100 条" 的字符串。
   */
  prefix?: (info: { itemCount: number }) => string;
}

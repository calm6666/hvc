import type { PropType } from 'vue';
import { propTypes } from '@/utils/propTypes';
import { BasicColumn } from './types/table';
import { NDataTable } from 'naive-ui';

/** 分页请求参数 */
type RequestParams = Record<string, unknown>;

export const basicProps = {
  /** 继承 Naive UI NDataTable 的所有 Props */
  ...NDataTable.props,

  /** 表格标题 */
  title: { type: String, default: null },

  /** 标题旁的提示文字 */
  titleTooltip: { type: String, default: null },

  /** 表格尺寸: 'small' | 'medium' | 'large' */
  size: { type: String, default: 'medium' },

  /** 静态数据源（与 request 互斥） */
  dataSource: { type: [Object], default: () => [] },

  /** 列配置（必填） */
  columns: { type: [Array] as PropType<BasicColumn[]>, default: () => [], required: true },

  /** 请求前钩子：可修改请求参数 */
  beforeRequest: {
    type: Function as PropType<(params: RequestParams) => RequestParams | Promise<RequestParams>>,
    default: null,
  },

  /** 数据请求函数（必填，与 dataSource 互斥） */
  request: {
    type: Function as PropType<(params: RequestParams) => Promise<Record<string, unknown>>>,
    default: null,
  },

  /** 请求后钩子：可修改响应数据 */
  afterRequest: {
    type: Function as PropType<(data: unknown[]) => unknown[] | Promise<unknown[]>>,
    default: null,
  },

  /** 行唯一标识字段 */
  rowKey: { type: [String, Function] as PropType<string | ((record: Record<string, unknown>) => string)>, default: undefined },

  /** 分页配置（false 关闭分页） */
  pagination: { type: [Object, Boolean], default: () => ({}) },

  /** @deprecated 使用 pagination 控制 */
  showPagination: { type: [String, Boolean], default: 'auto' },

  /** 操作列配置 */
  actionColumn: { type: Object as PropType<BasicColumn>, default: null },

  /** 是否自适应高度 */
  canResize: propTypes.bool.def(true),

  /** 高度偏移量 */
  resizeHeightOffset: propTypes.number.def(0),

  /** 是否显示斑马纹 */
  striped: propTypes.bool.def(false),
};

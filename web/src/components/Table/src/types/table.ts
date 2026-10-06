import type { InternalRowData, TableBaseColumn } from 'naive-ui/lib/data-table/src/interface';
import type { ComponentType } from './componentType';

/**
 * 表格列配置接口（扩展自 Naive UI TableBaseColumn）。
 * 增加编辑表格、权限控制、拖拽等业务功能。
 */
export interface BasicColumn<T = InternalRowData> extends TableBaseColumn<T> {
  /** 是否可编辑（单元格模式） */
  edit?: boolean;
  /** 是否可编辑（行模式） */
  editRow?: boolean;
  /** 是否始终可编辑 */
  editable?: boolean;
  /** 编辑态下使用的组件类型 */
  editComponent?: ComponentType;
  /** 编辑组件的属性 */
  editComponentProps?: Record<string, unknown>;
  /** 编辑校验规则：true=必填, 函数=自定义校验 */
  editRule?: boolean | ((text: string, record: Record<string, unknown>) => Promise<string>);
  /** 编辑值展示转换函数 */
  editValueMap?: (value: unknown) => string;
  /** 编辑行回调 */
  onEditRow?: () => void;
  /** 权限编码：控制该列是否显示 */
  auth?: string[];
  /** 业务条件：控制该列是否显示 */
  ifShow?: boolean | ((column: BasicColumn) => boolean);
  /** 是否支持列拖拽排序 */
  draggable?: boolean;
}

/** 表格暴露给外部的操作方法 */
export interface TableActionType {
  /** 重新加载数据 */
  reload: (opt?: Record<string, unknown>) => Promise<void>;
  /** 事件发射器 */
  emit?: (event: string, ...args: unknown[]) => void;
  /** 获取当前列配置 */
  getColumns: (opt?: Record<string, unknown>) => BasicColumn[];
  /** 设置列配置 */
  setColumns: (columns: BasicColumn[] | string[]) => void;
}

/** BasicTable 组件的 Props 类型 */
export interface BasicTableProps {
  /** 表格标题 */
  title?: string;
  /** 静态数据源（与 request 互斥） */
  dataSource: Record<string, unknown>[];
  /** 列配置 */
  columns: BasicColumn[];
  /** 分页配置 */
  pagination: Record<string, unknown> | boolean;
  /** 是否显示分页 */
  showPagination: boolean;
  /** 操作列配置 */
  actionColumn: BasicColumn | null;
  /** 是否自适应高度 */
  canResize: boolean;
  /** 高度偏移量 */
  resizeHeightOffset: number;
  /** 加载状态 */
  loading: boolean;
}

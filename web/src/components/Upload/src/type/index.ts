/** @deprecated Upload 组件基础类型已迁移至 Naive UI NUpload.props */
export interface BasicProps {
  title?: string;
  dataSource: () => unknown[];
  columns: Record<string, unknown>[];
  pagination: Record<string, unknown>;
  showPagination: boolean;
}

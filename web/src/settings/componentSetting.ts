/**
 * 组件预设配置。
 *
 * 分页字段与后端 writePageResponse 对齐：
 *   后端返回 { page, page_size, total, items }
 *   经 toCamelCase 转换后 → { page, pageSize, total, items }
 *
 * 注意：后端不返回总页数（pageCount），由 useDataSource 根据 total / pageSize 前端计算。
 */
export default {
  table: {
    apiSetting: {
      // 当前页的字段名（后端返回 "page"）
      pageField: 'page',
      // 每页数量字段名（后端返回 "page_size"，toCamelCase 后为 "pageSize"）
      sizeField: 'pageSize',
      // 接口返回的数据列表字段名（后端返回 "items"）
      listField: 'items',
      // 总页数（后端不返回，由 useDataSource 前端计算，此处保留字段名用于分页组件内部传递）
      totalField: 'pageCount',
      // 总条数字段名（后端返回 "total"）
      countField: 'total',
    },
    // 默认分页数量
    defaultPageSize: 10,
    // 可切换每页数量集合
    pageSizes: [10, 20, 30, 40, 50],
  },
  upload: {
    // 考虑接口规范不同
    apiSetting: {
      // 集合字段名
      infoField: 'data',
      // 图片地址字段名
      imgField: 'photo',
    },
    // 最大上传图片大小（MB）
    maxSize: 2,
    // 允许上传的图片类型
    fileType: ['image/png', 'image/jpg', 'image/jpeg', 'image/gif', 'image/svg+xml'],
  },
};

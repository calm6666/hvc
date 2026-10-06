import { h } from 'vue';
import { NTag } from 'naive-ui';

/**
 * 角色表格列定义。
 *
 * 字段名与后端 adminRoleView（toCamelCase 后）对齐：
 *   roleId, roleKey, roleName, roleDesc, status, createdAt, updatedAt
 */
export const columns = [
  {
    title: 'ID',
    key: 'roleId',
    width: 80,
  },
  {
    title: '角色标识',
    key: 'roleKey',
    width: 140,
  },
  {
    title: '角色名称',
    key: 'roleName',
    width: 140,
  },
  {
    title: '说明',
    key: 'roleDesc',
    width: 160,
    ellipsis: { tooltip: true },
  },
  {
    title: '状态',
    key: 'status',
    width: 80,
    render(row: Recordable) {
      return h(
        NTag,
        { type: row.status === 1 ? 'success' : 'default' },
        () => (row.status === 1 ? '启用' : '禁用')
      );
    },
  },
  {
    title: '创建时间',
    key: 'createdAt',
    width: 170,
  },
];

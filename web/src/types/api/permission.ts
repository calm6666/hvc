// ============================================================
// 权限管理 — AdminPermissionRecord / RBACHandler (internal/interfaces/http/admin/)
// ============================================================

/**
 * 权限点详情（叶子节点）。
 * 对应后端: adminPermissionView (internal/interfaces/http/admin/rbac_view.go:34)
 *
 * Go struct → toCamelCase → TS:
 *   PermID    → permId
 *   PermKey   → permKey
 *   PermName  → permName
 *   PermDesc  → permDesc
 *   Module    → module
 *   CreatedAt → createdAt
 */
export interface AdminPermission {
  permId: number;
  /** 权限标识（如 "cluster.read"） */
  permKey: string;
  /** 权限名称 */
  permName: string;
  /** 权限描述 */
  permDesc: string;
  /** 所属模块 */
  module: string;
  createdAt: string;
}

/**
 * 权限树节点。
 * 对应后端: permissionTreeNode (internal/interfaces/http/admin/rbac_view.go:83)
 *
 * Go struct → toCamelCase → TS:
 *   ID         → id
 *   Key        → key
 *   Label      → label
 *   NodeType   → nodeType     // 'module' | 'group' | 'permission'
 *   Module     → module
 *   Permission → permission   // 仅叶子节点有值
 *   Children   → children
 */
export interface PermissionTreeNode {
  id: string;
  key: string;
  label: string;
  /** 节点类型: 'module'=模块, 'group'=分组, 'permission'=权限点 */
  nodeType: 'module' | 'group' | 'permission';
  module: string;
  /** 仅叶子节点有值 */
  permission?: AdminPermission;
  children?: PermissionTreeNode[];
}

/**
 * 创建/更新权限请求体。
 * 对应后端: WriteRBAC.UpsertPermission (rbac_write.go:106)
 */
export interface UpsertPermissionRequest {
  /** 权限标识（如 "cluster.read"） */
  permKey: string;
  /** 权限名称 */
  permName: string;
  /** 权限描述 */
  permDesc?: string;
  /** 所属模块（如 "cluster", "system"） */
  module: string;
}

/**
 * 树状数据 → 可展开行列表的扁平化工具。
 *
 * 用于将 AdminMenuNode / PermissionTreeNode 等树形数据
 * 转换为 BasicTable 可渲染的扁平行列表，支持：
 *   - 展开/折叠控制（通过 expandedRowKeys 管理可见行）
 *   - 层级缩进（depth 字段控制缩进像素）
 *   - 子树过滤（仅展开节点下的子节点可见）
 *
 * 使用方式：
 *   const { rows } = flattenTree(treeData, expandedKeys, 'menuId', 'children');
 *   // rows 为扁平数组，每行含 depth, hasChildren, isExpanded 等元数据
 */

/** 扁平化后的树行（在原始节点字段基础上增加表格渲染所需的元数据） */
export interface FlattenedTreeRow<T> extends Record<string, unknown> {
  /** 原始节点数据 */
  _raw: T;
  /** 层级深度（0 = 根节点） */
  depth: number;
  /** 是否有子节点 */
  hasChildren: boolean;
  /** 当前是否展开（仅 hasChildren=true 时有效） */
  isExpanded: boolean;
}

/**
 * 将树形数据递归扁平化为行列表。
 *
 * @param nodes        — 树形数据节点数组（必须包含 children 字段）
 * @param expandedKeys — 当前展开的节点 key 集合
 * @param keyField     — 节点唯一标识字段名（如 'menuId'）
 * @param childrenField — 子节点数组字段名（如 'children'）
 * @param depth        — 当前递归深度（首次调用不传，内部使用）
 * @returns 扁平行数组
 */
export function flattenTree<T extends Record<string, unknown>>(
  nodes: T[] | undefined,
  expandedKeys: Set<string>,
  keyField: string,
  childrenField: string,
  depth: number = 0
): FlattenedTreeRow<T>[] {
  if (!nodes || !Array.isArray(nodes)) return [];

  const result: FlattenedTreeRow<T>[] = [];

  for (const node of nodes) {
    const children = (node[childrenField] as T[]) || [];
    const hasChildren = children.length > 0;
    const key = String(node[keyField]);
    const isExpanded = expandedKeys.has(key);

    result.push({
      ...node,
      _raw: node,
      depth,
      hasChildren,
      isExpanded,
    });

    // 递归展开子节点
    if (hasChildren && isExpanded) {
      result.push(...flattenTree(children, expandedKeys, keyField, childrenField, depth + 1));
    }
  }

  return result;
}

/**
 * 收集树中所有节点的 key。
 * 用于"全部展开"操作。
 */
export function collectAllKeys<T extends Record<string, unknown>>(
  nodes: T[] | undefined,
  keyField: string,
  childrenField: string
): string[] {
  if (!nodes) return [];
  const keys: string[] = [];
  for (const node of nodes) {
    keys.push(String(node[keyField]));
    const children = node[childrenField] as T[] | undefined;
    if (children) {
      keys.push(...collectAllKeys(children, keyField, childrenField));
    }
  }
  return keys;
}

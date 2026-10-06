import { useUserStore } from '@/store/modules/user';

/**
 * 权限判断 Hook。
 *
 * 后端 WhoAmI 返回的 permission_keys 为 string[]（如 ["cluster.read", "live.channel.create"]），
 * 已由 userStore 存储在 state.permissions（类型为 string[]）。
 *
 * 所有权限判断直接对字符串值做 includes 比较。
 */
export function usePermission() {
  const userStore = useUserStore();

  /**
   * 判断当前用户是否拥有指定权限集合中的至少一个权限。
   * @param accesses — 需要校验的权限键列表
   */
  function hasPermission(accesses: string[]): boolean {
    if (!accesses || !accesses.length) return true;
    const permissions = userStore.getPermissions;
    return accesses.some((access) => permissions.includes(access));
  }

  /**
   * 判断当前用户是否同时拥有指定权限集合中的全部权限。
   * @param accesses — 需要校验的权限键列表
   */
  function hasEveryPermission(accesses: string[]): boolean {
    const permissions = userStore.getPermissions;
    if (!Array.isArray(accesses)) {
      throw new Error(`[hasEveryPermission]: ${String(accesses)} should be an array`);
    }
    return accesses.every((access) => permissions.includes(access));
  }

  /**
   * 判断当前用户是否拥有指定权限集合中的至少一个权限（同 hasPermission）。
   * @param accesses — 需要校验的权限键列表
   */
  function hasSomePermission(accesses: string[]): boolean {
    const permissions = userStore.getPermissions;
    if (!Array.isArray(accesses)) {
      throw new Error(`[hasSomePermission]: ${String(accesses)} should be an array`);
    }
    return accesses.some((access) => permissions.includes(access));
  }

  return { hasPermission, hasEveryPermission, hasSomePermission };
}

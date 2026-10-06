/**
 * 键名转换工具：下划线命名 ↔ 驼峰命名
 *
 * 设计目标：
 * 1. 性能接近原生对象遍历（无额外开销）
 * 2. 零内存泄漏（无全局缓存）
 * 3. 完整的 TypeScript 泛型类型推断
 * 4. 支持嵌套对象、数组、基本类型的递归转换
 */

// ============================================================================
// 第一部分：核心字符串转换函数（纯函数，无副作用）
// ============================================================================

/**
 * 预编译正则表达式。
 * 定义为模块级常量，避免每次调用时重新编译。
 *
 * 性能说明：
 * - 正则对象在模块加载时创建一次，全局复用。
 * - 简单正则在 V8 中被编译为高效的本地代码，单次替换耗时 < 0.5 微秒。
 */
const SNAKE_TO_CAMEL_RE = /_([a-z])/g; // 匹配 "_小写字母"
const CAMEL_TO_SNAKE_RE = /([A-Z])/g; // 匹配任意大写字母
const LEADING_UNDERSCORE_RE = /^_/; // 匹配开头的下划线（处理如 "_id" 的情况）

/**
 * 下划线命名 → 驼峰命名
 * @example snakeToCamel("user_id")   → "userId"
 * @example snakeToCamel("created_at") → "createdAt"
 * @param str - 下划线格式的字符串
 * @returns 驼峰格式的字符串
 */
function snakeToCamel(str: string): string {
  // 正则替换：将 "_字母" 替换为 "字母" 的大写形式
  // 第二个参数是回调函数，_ 代表完整匹配（如 "_i"），letter 代表第一个捕获组（"i"）
  return str.replace(SNAKE_TO_CAMEL_RE, (_, letter: string) => letter.toUpperCase());
}

/**
 * 驼峰命名 → 下划线命名
 * @example camelToSnake("userId")    → "user_id"
 * @example camelToSnake("createdAt") → "created_at"
 * @param str - 驼峰格式的字符串
 * @returns 下划线格式的字符串
 */
function camelToSnake(str: string): string {
  // 步骤：
  // 1. 在所有大写字母前插入下划线 → "created_At"
  // 2. 统一转为小写 → "created_at"
  // 3. 移除开头可能多余的下划线（如 "Id" 变 "_id" 再变 "id"）
  return str.replace(CAMEL_TO_SNAKE_RE, '_$1').toLowerCase().replace(LEADING_UNDERSCORE_RE, '');
}

// ============================================================================
// 第二部分：递归对象转换核心（无缓存，内存安全）
// ============================================================================

/**
 * 键名转换函数类型定义
 */
type KeyTransformer = (key: string) => string;

/**
 * 深度转换对象的键名。
 *
 * 性能优化要点：
 * - 使用 for 循环代替 forEach/map，减少函数调用栈开销。
 * - 对数组和普通对象分别处理，不引入额外抽象层。
 * - 基本类型直接返回，避免不必要的递归。
 * - 无任何缓存机制，保证内存零增长。
 *
 * @param obj - 待转换的任意值（对象、数组、基本类型）
 * @param transformKey - 键名转换函数
 * @returns 转换后的新对象（原对象不变）
 */
function transformKeys<T>(obj: T, transformKey: KeyTransformer): unknown {
  // 基本类型直接返回（null 的 typeof 也是 "object"，需单独判断）
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  // 处理数组：递归转换每个元素，保持数组结构
  if (Array.isArray(obj)) {
    const len = obj.length;
    const newArr = Array.from({ length: len }); // 预分配数组大小，提升性能
    for (let i = 0; i < len; i++) {
      newArr[i] = transformKeys(obj[i], transformKey);
    }
    return newArr;
  }

  // 处理普通对象：遍历所有自有可枚举属性
  const newObj: Record<string, unknown> = {};
  const keys = Object.keys(obj); // 只获取自有属性，忽略原型链
  for (let i = 0; i < keys.length; i++) {
    const originalKey = keys[i];
    const newKey = transformKey(originalKey);
    const originalValue = (obj as Record<string, unknown>)[originalKey];

    // 递归处理属性值（可能是嵌套对象或数组）
    newObj[newKey] = transformKeys(originalValue, transformKey);
  }
  return newObj;
}

// ============================================================================
// 第三部分：TypeScript 类型工具（用于智能类型推断）
// ============================================================================

/**
 * 将字符串字面量类型转换为驼峰命名。
 *
 * 示例：
 *   CamelCase<"user_id">      → "userId"
 *   CamelCase<"created_at">   → "createdAt"
 *   CamelCase<"challenge_id"> → "challengeId"
 */
type CamelCase<S extends string> = S extends `${infer Head}_${infer Tail}`
  ? `${Head}${Capitalize<CamelCase<Tail>>}`
  : S;

/**
 * 将字符串字面量类型转换为下划线命名。
 *
 * 示例：
 *   SnakeCase<"userId">      → "user_id"
 *   SnakeCase<"createdAt">   → "created_at"
 *   SnakeCase<"challengeId"> → "challenge_id"
 */
type SnakeCase<S extends string> = S extends `${infer Head}${infer Tail}`
  ? Head extends Uppercase<Head>
    ? `_${Lowercase<Head>}${SnakeCase<Tail>}`
    : `${Head}${SnakeCase<Tail>}`
  : S;

/**
 * 排除 Date、RegExp、FormData 等内置对象。
 * 只有“普通对象 / 数组”才需要递归映射键名。
 */
type IsPlainLike<T> = T extends
  | Date
  | RegExp
  | FormData
  | Blob
  | File
  | Map<any, any>
  | Set<any>
  | WeakMap<any, any>
  | WeakSet<any>
  | ArrayBuffer
  | DataView
  ? false
  : T extends object
  ? true
  : false;

/**
 * 递归地将对象的键名从下划线格式转换为驼峰格式。
 *
 * - 元组：逐元素转换，保留长度和位置类型
 * - 普通数组：转换元素类型
 * - 只读数组：保留 readonly 修饰
 * - 普通对象：递归转换键名和值
 * - 基本类型 / Date / RegExp / Blob 等：原样保留
 *
 * 示例：
 *   CamelCaseKeys<{ user_id: number; created_at: string }>
 *   → { userId: number; createdAt: string }
 */
export type CamelCaseKeys<T> =
  // 元组：逐元素转换，保留结构
  T extends readonly [infer First, ...infer Rest]
    ? readonly [CamelCaseKeys<First>, ...CamelCaseKeys<Rest>]
    : T extends [infer First, ...infer Rest]
    ? [CamelCaseKeys<First>, ...CamelCaseKeys<Rest>]
    : T extends ReadonlyArray<infer Item> // 只读数组
    ? readonly CamelCaseKeys<Item>[]
    : T extends Array<infer Item> // 可变数组
    ? CamelCaseKeys<Item>[]
    : IsPlainLike<T> extends true // 普通对象（排除内置类型）
    ? {
        [K in keyof T as CamelCase<K & string>]: CamelCaseKeys<T[K]>;
      }
    : T;

/**
 * 递归地将对象的键名从驼峰格式转换为下划线格式。
 *
 * 规则与 CamelCaseKeys 一致。
 *
 * 示例：
 *   SnakeCaseKeys<{ userId: number; createdAt: string }>
 *   → { user_id: number; created_at: string }
 */
export type SnakeCaseKeys<T> = T extends readonly [infer First, ...infer Rest]
  ? readonly [SnakeCaseKeys<First>, ...SnakeCaseKeys<Rest>]
  : T extends [infer First, ...infer Rest]
  ? [SnakeCaseKeys<First>, ...SnakeCaseKeys<Rest>]
  : T extends ReadonlyArray<infer Item>
  ? readonly SnakeCaseKeys<Item>[]
  : T extends Array<infer Item>
  ? SnakeCaseKeys<Item>[]
  : IsPlainLike<T> extends true
  ? {
      [K in keyof T as SnakeCase<K & string>]: SnakeCaseKeys<T[K]>;
    }
  : T;

// ============================================================================
// 第四部分：公开 API
// ============================================================================

/**
 * 将对象（或数组）的所有键名从下划线格式转换为驼峰格式。
 *
 * 泛型 T 从入参自动推断，也可显式传入以获得编译时字段提示：
 *
 * @example 自动推断
 * const apiData = { user_id: 1, created_at: "2024-01-01" };
 * const result = toCamelCase(apiData);
 * // result 类型: { userId: number; createdAt: string }
 * result.userId // ✅ 有字段提示
 *
 * @example 显式泛型（从 API 类型推导）
 * type BackendResp = { challenge_id: string; captcha_type: string };
 * const result = toCamelCase<BackendResp>(data);
 * // result 类型: { challengeId: string; captchaType: string }
 *
 * @param obj - 待转换的对象（键名应为下划线格式）
 * @returns 新对象（键名为驼峰格式），原对象不变
 */
export function toCamelCase<T>(obj: T): CamelCaseKeys<T> {
  return transformKeys(obj, snakeToCamel) as CamelCaseKeys<T>;
}

/**
 * 将对象（或数组）的所有键名从驼峰格式转换为下划线格式。
 *
 * 泛型 T 从入参自动推断，也可显式传入以获得编译时字段提示：
 *
 * @example 自动推断
 * const frontendData = { userId: 1, createdAt: "2024-01-01" };
 * const result = toSnakeCase(frontendData);
 * // result 类型: { user_id: number; created_at: string }
 * result.user_id // ✅ 有字段提示
 *
 * @example 显式泛型
 * type FrontendReq = { userId: number; createTime: string };
 * const result = toSnakeCase<FrontendReq>(req);
 * // result 类型: { user_id: number; create_time: string }
 *
 * @param obj - 待转换的对象（键名应为驼峰格式）
 * @returns 新对象（键名为下划线格式），原对象不变
 */
export function toSnakeCase<T>(obj: T): SnakeCaseKeys<T> {
  return transformKeys(obj, camelToSnake) as SnakeCaseKeys<T>;
}

// ============================================================================
// 第五部分：可选优化 —— 请求级缓存（仅当单次请求字段重复率高时使用）
// ============================================================================

/**
 * 带局部缓存的转换器类。
 * 适用于单次请求内存在大量重复字段名的场景（如批量数据处理），
 * 请求结束后实例被 GC，内存自动回收。
 *
 * @example
 * const converter = new KeyConverter();
 * const result = converter.toCamelCase<BackendUser[]>(users);
 * // result[0].userId ✅ 有字段提示
 */
export class KeyConverter {
  private snakeToCamelCache = new Map<string, string>();
  private camelToSnakeCache = new Map<string, string>();

  /**
   * 下划线转驼峰（带实例缓存）
   */
  private cachedSnakeToCamel(str: string): string {
    const cached = this.snakeToCamelCache.get(str);
    if (cached !== undefined) {
      return cached;
    }
    const result = snakeToCamel(str);
    this.snakeToCamelCache.set(str, result);
    return result;
  }

  /**
   * 驼峰转下划线（带实例缓存）
   */
  private cachedCamelToSnake(str: string): string {
    const cached = this.camelToSnakeCache.get(str);
    if (cached !== undefined) {
      return cached;
    }
    const result = camelToSnake(str);
    this.camelToSnakeCache.set(str, result);
    return result;
  }

  /**
   * 将对象键名转换为驼峰格式（带实例缓存）。
   * 支持显式泛型以获得字段提示。
   */
  toCamelCase<T>(obj: T): CamelCaseKeys<T> {
    return transformKeys(obj, this.cachedSnakeToCamel.bind(this)) as CamelCaseKeys<T>;
  }

  /**
   * 将对象键名转换为下划线格式（带实例缓存）。
   * 支持显式泛型以获得字段提示。
   */
  toSnakeCase<T>(obj: T): SnakeCaseKeys<T> {
    return transformKeys(obj, this.cachedCamelToSnake.bind(this)) as SnakeCaseKeys<T>;
  }

  /**
   * 手动清空缓存（通常不需要，实例失去引用后会自动 GC）
   */
  clearCache(): void {
    this.snakeToCamelCache.clear();
    this.camelToSnakeCache.clear();
  }
}

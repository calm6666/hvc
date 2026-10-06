import { createAlova } from 'alova';
import VueHook from 'alova/vue';
import adapterFetch from 'alova/fetch';
import { isString } from 'lodash-es';
import { useGlobSetting } from '@/hooks/setting';
import { PageEnum } from '@/enums/pageEnum';
import { ResultEnum } from '@/enums/httpEnum';
import { isUrl } from '@/utils';
import { toCamelCase, toSnakeCase } from '@/utils/caseConverter';

const { apiUrl, urlPrefix } = useGlobSetting();

export const Alova = createAlova({
  baseURL: apiUrl,
  statesHook: VueHook,
  // 在开发环境开启缓存命中日志
  cacheLogger: process.env.NODE_ENV === 'development',
  // 直接使用 fetch 适配器（Mock 已移除，走真实后端）
  requestAdapter: adapterFetch(),
  /**
   * 请求前拦截器。
   * - 鉴权改用 HttpOnly Cookie（login 时服务端 Set-Cookie，后续请求浏览器自动携带）。
   *   因此不再需要在请求头中手动注入 token。
   * - 为相对路径的 api 自动拼接 urlPrefix（如 /v1/admin）和 baseURL。
   * - 请求体中的对象键名由驼峰转为下划线，匹配后端 Go 结构体的 json tag。
   *   注意：FormData（文件上传）和字符串 body（如 URLSearchParams）不会被转换。
   */
  beforeRequest(method) {
    // 判断是否为完整的 http(s) 链接，若是则跳过前缀拼接
    const isUrlStr = isUrl(method.url as string);

    // 拼接接口前缀（来自环境变量 VITE_GLOB_API_URL_PREFIX，如 /v1/admin）
    if (!isUrlStr && urlPrefix) {
      method.url = `${urlPrefix}${method.url}`;
    }

    // 拼接 API 基础地址（来自环境变量 VITE_GLOB_API_URL，生产环境使用）
    if (!isUrlStr && apiUrl && isString(apiUrl)) {
      method.url = `${apiUrl}${method.url}`;
    }

    // GET 查询参数键名驼峰 → 下划线（如 pageSize → page_size）
    if (method.config?.params && typeof method.config.params === 'object') {
      method.config.params = toSnakeCase(method.config.params);
    }

    // POST/PUT 请求体键名驼峰 → 下划线（前端 camelCase → 后端 snake_case）
    if (method.data && typeof method.data === 'object' && !(method.data instanceof FormData)) {
      method.data = toSnakeCase(method.data);
    }
  },

  /**
   * 响应拦截器。
   *
   * 后端统一响应格式（package model.Response）：
   *   { "code": 0, "message": "ok", "data": { ... } }
   *
   * code 语义：
   *   0   — 成功
   *   401 — 未登录或 session 过期
   *   400 — 请求参数错误
   *   500 — 服务端内部错误
   *
   * 三种返回模式由 meta 控制：
   *   1. isReturnNativeResponse: true  → 返回完整 { code, message, data }，由调用方自行处理（如 login / WhoAmI）
   *   2. isTransformResponse: false   → 直接返回 res.data，不做任何转换
   *   3. 默认                          → 成功时只返回 data（并自动 toCamelCase），失败时弹窗提示
   */
  responded: {
    onSuccess: async (response, method) => {
      // 解析 JSON 响应体；非 JSON 场景回退到 response.body
      const res = (response.json && (await response.json())) || response.body;

      // ---- 模式 1：返回原生响应对象 ----
      // login / WhoAmI 等接口需要自行读取 code / data / message
      if (method.meta?.isReturnNativeResponse) {
        // 将 data 内的键名转为驼峰（后端 Go 结构体 json tag 为 snake_case）
        if (res.data && typeof res.data === 'object') {
          res.data = toCamelCase(res.data);
        }
        return res;
      }

      // 从响应信封中解构：code（业务状态码）、message（提示信息）、data（业务数据）
      const { message, code, data } = res;

      // ---- 模式 2：跳过转换，直接返回 data ----
      if (method.meta?.isTransformResponse === false) {
        return data;
      }

      // 脱离上下文的 Naive UI API（已在 App.vue 启动时挂载到 window）
      const Message = window['$message'];
      const Modal = window['$dialog'];
      const LoginPath = PageEnum.BASE_LOGIN;

      // ---- 模式 3：成功时返回 data，失败时统一处理 ----
      if (ResultEnum.SUCCESS === code) {
        // 将 data 内的键名转为驼峰（后端 Go 结构体 json tag 为 snake_case）
        if (data && typeof data === 'object') {
          return toCamelCase(data);
        }
        return data;
      }

      // 401：未登录或 session 已过期
      // 后端已通过 Set-Cookie 清除 admin_session，前端只需提示并跳转登录页
      if (code === 401) {
        Modal?.warning({
          title: '提示',
          content: '登录身份已失效，请重新登录!',
          okText: '确定',
          closable: false,
          maskClosable: false,
          onOk: () => {
            window.location.href = LoginPath;
          },
        });
      } else {
        // 其他错误（400 / 500）：弹出 message 提示并抛出异常
        Message?.error(message);
        throw new Error(message);
      }
    },

    /**
     * HTTP 错误响应处理（4xx / 5xx）。
     *
     * isReturnNativeResponse 模式（login / whoAmI）需要自行读取业务 code，
     * 因此将响应 body 原样返回，由调用方处理。
     * 其他模式统一弹出错误提示。
     */
    onError: async (error, method) => {
      // 原生响应模式：尝试读取 body 并返回，让调用方自行判断业务状态码
      if (method.meta?.isReturnNativeResponse) {
        try {
          const res = await error.response?.json();
          if (res) {
            if (res.data && typeof res.data === 'object') {
              res.data = toCamelCase(res.data);
            }
            return res;
          }
        } catch { /* body 不可解析时走下方通用错误 */ }
      }

      // 通用错误处理
      const Message = window['$message'];
      Message?.error('网络请求失败，请检查后端服务是否启动');
      throw error;
    },
  },
});

// 项目，多个不同 api 地址，可导出多个实例
// export const AlovaTwo = createAlova({
//   baseURL: 'http://localhost:9001',
// });

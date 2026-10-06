import { onUnmounted } from 'vue';

// ============================================================
// 类型定义
// ============================================================

/**
 * WebSocket 连接配置选项。
 */
interface WebSocketOptions {
  /** WebSocket 服务端地址（ws:// 或 wss://） */
  url: string;
  /** 子协议（可选） */
  protocols?: string | string[];
  /** 断线重连延迟（毫秒），默认 5000 */
  reconnectTimeout?: number;
}

/**
 * WebSocket 事件回调映射。
 *
 * 使用泛型 T 指定 message 事件的回调参数类型：
 *   useWebSocket<TranscodeSnapshot>({ url: '...' })
 *     → on('message', (data: TranscodeSnapshot) => { ... })  // data 已强类型
 *
 * 使用方式：
 *   const ws = useWebSocket<MyMessageType>({ url: 'ws://...' });
 *   ws.on('message', (data) => { data.xxx });  // data 类型为 MyMessageType
 */
interface WebSocketEventMap<T = unknown> {
  /** 连接成功时触发 */
  open: () => void;
  /** 收到消息时触发。data 为 JSON 解析后的对象（或原始字符串） */
  message: (data: T) => void;
  /** 发生错误时触发。error 为浏览器原生 Event 对象 */
  error: (error: Event) => void;
  /** 连接关闭时触发 */
  close: () => void;
}

/** 事件名称 */
type EventName = keyof WebSocketEventMap;

/** 回调函数类型（根据事件名称推导参数） */
type CallbackForEvent<T, E extends EventName> = WebSocketEventMap<T>[E];

// ============================================================
// WebSocketService 类
// ============================================================

/**
 * WebSocket 连接管理服务。
 *
 * 功能：
 *   - 自动 JSON 解析/序列化
 *   - 断线自动重连（可配置延迟）
 *   - 类型安全的事件回调（通过泛型 T 推导 message 数据类型）
 *   - Vue 组合式 API 集成（onUnmounted 自动销毁）
 *
 * @template T — message 事件回调中 data 参数的类型（默认 unknown）
 *
 * @example
 *   interface Snapshot { jobId: number; fps: number }
 *   const ws = useWebSocket<Snapshot>({ url: 'ws://localhost:8888/v1/admin/transcode/monitor/ws' });
 *   ws.on('message', (snapshot) => { console.log(snapshot.fps); });
 */
class WebSocketService<T = unknown> {
  /** 原生 WebSocket 实例 */
  private ws: WebSocket | null = null;
  /** 事件回调注册表 */
  private callbacks: { [K in EventName]?: CallbackForEvent<T, K>[] } = {};
  /** 断线重连延迟（毫秒） */
  private reconnectTimeoutMs: number = 5000;
  /** 连接地址 */
  private url: string;
  /** 子协议 */
  private protocols?: string | string[];

  constructor(private options: WebSocketOptions) {
    this.url = options.url;
    this.protocols = options.protocols;
    if (options.reconnectTimeout) this.reconnectTimeoutMs = options.reconnectTimeout;
  }

  // ============================================================
  // 公开方法
  // ============================================================

  /**
   * 打开 WebSocket 连接。
   * 如果已有活跃连接（OPEN 或 CONNECTING 状态），则跳过。
   */
  public open(): void {
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) return;
    this.ws = new WebSocket(this.url, this.protocols);
    this.ws.addEventListener('open', this.handleOpen);
    this.ws.addEventListener('message', this.handleMessage);
    this.ws.addEventListener('error', this.handleError);
    this.ws.addEventListener('close', this.handleClose);
  }

  /**
   * 关闭 WebSocket 连接。
   * @param isActiveClose — true 表示用户主动关闭，不触发重连
   */
  public close(isActiveClose = false): void {
    if (this.ws) {
      try { this.ws.close(); } catch { /* 忽略关闭时的网络错误 */ }
      if (!isActiveClose) {
        setTimeout(() => this.reconnect(), this.reconnectTimeoutMs);
      }
    }
  }

  /** 手动重连 */
  public reconnect(): void { this.open(); }

  /**
   * 更换连接地址并重新连接。
   * 如果当前连接已打开，先主动关闭再打开新连接。
   */
  public setUrl(url: string): void {
    this.url = url;
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.close(true);
      this.open();
    }
  }

  /** 检查 WebSocket 是否处于就绪状态 */
  public isReady(): boolean {
    return !!this.ws && this.ws.readyState === WebSocket.OPEN;
  }

  /**
   * 发送消息。
   * 对象类型自动 JSON.stringify，字符串类型原样发送。
   */
  public send(data: T | string): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      const payload = typeof data === 'string' ? data : JSON.stringify(data);
      this.ws.send(payload);
    }
  }

  /**
   * 注册事件回调。
   *
   * @param event — 事件名称：'open' | 'message' | 'error' | 'close'
   * @param callback — 回调函数。message 事件的 data 参数类型由泛型 T 决定
   *
   * @example
   *   ws.on('message', (data: Snapshot) => { ... });  // data 类型 = T
   *   ws.on('open', () => { console.log('connected'); });
   *   ws.on('error', (error) => { console.error(error); });
   *   ws.on('close', () => { console.log('disconnected'); });
   */
  public on<E extends EventName>(event: E, callback: CallbackForEvent<T, E>): void {
    if (!this.callbacks[event]) {
      (this.callbacks[event] as unknown as CallbackForEvent<T, E>[]) = [];
    }
    (this.callbacks[event] as CallbackForEvent<T, E>[])!.push(callback);
  }

  /**
   * 取消事件回调。
   *
   * @param event — 事件名称
   * @param callback — 要移除的特定回调函数（不传则清空该事件的全部回调）
   */
  public off<E extends EventName>(event: E, callback?: CallbackForEvent<T, E>): void {
    if (!this.callbacks[event]) return;
    if (!callback) {
      this.callbacks[event] = [];
      return;
    }
    const cbs = this.callbacks[event] as CallbackForEvent<T, E>[];
    this.callbacks[event] = cbs.filter((cb) => cb !== callback) as typeof this.callbacks[E];
  }

  /**
   * 彻底销毁 WebSocket 连接。
   * 清空全部回调 → 关闭连接 → 置空实例。
   * 通常在组件 onUnmounted 时调用。
   */
  public destroy(): void {
    this.off('message');
    this.off('open');
    this.off('error');
    this.off('close');
    this.close(true);
    this.ws = null;
  }

  // ============================================================
  // 私有方法 — 原生 WebSocket 事件处理器
  // ============================================================

  /** 连接成功 → 触发 open 回调 */
  private handleOpen = (): void => {
    this.callbacks.open?.forEach((cb) => (cb as () => void)());
  };

  /**
   * 收到消息 → 尝试 JSON.parse，成功则传解析后的对象，失败则传原始字符串。
   * 触发 message 回调，data 参数类型为 T。
   */
  private handleMessage = (event: MessageEvent): void => {
    let data: T;
    try {
      data = JSON.parse(event.data) as T;
    } catch {
      data = event.data as unknown as T;
    }
    this.callbacks.message?.forEach((cb) => (cb as (data: T) => void)(data));
  };

  /** 发生错误 → 触发 error 回调 */
  private handleError = (error: Event): void => {
    this.callbacks.error?.forEach((cb) => (cb as (error: Event) => void)(error));
  };

  /** 连接关闭 → 触发 close 回调。若未设置重连超时（即 0），则自动重连 */
  private handleClose = (): void => {
    this.callbacks.close?.forEach((cb) => (cb as () => void)());
    if (!this.options.reconnectTimeout) this.reconnect();
  };
}

// ============================================================
// Vue 组合式 API Hook
// ============================================================

/**
 * 创建 WebSocket 连接并在组件卸载时自动销毁。
 *
 * @template T — message 事件中 data 参数的类型
 * @param options — WebSocket 配置
 * @returns WebSocketService 实例的公开方法集合
 *
 * @example
 *   interface MonitorSnapshot { timestamp: number; jobs: Array<{jobId: number; fps: number}> }
 *
 *   const ws = useWebSocket<MonitorSnapshot>({
 *     url: 'ws://localhost:8888/v1/admin/transcode/monitor/ws',
 *     reconnectTimeout: 3000,
 *   });
 *
 *   ws.on('message', (snapshot) => {
 *     // snapshot 类型为 MonitorSnapshot，IDE 有完整提示
 *     console.log(snapshot.timestamp, snapshot.jobs.length);
 *   });
 *
 *   ws.open();
 */
export default function useWebSocket<T = unknown>(options: WebSocketOptions) {
  const wsService = new WebSocketService<T>(options);

  onUnmounted(() => {
    wsService.destroy();
  });

  return {
    /** 打开 WebSocket 连接 */
    open: wsService.open.bind(wsService),
    /** 关闭 WebSocket 连接 */
    close: wsService.close.bind(wsService),
    /** 手动重连 */
    reconnect: wsService.reconnect.bind(wsService),
    /** 注册事件回调（open / message / error / close） */
    on: wsService.on.bind(wsService),
    /** 取消事件回调 */
    off: wsService.off.bind(wsService),
    /** 检查连接是否就绪 */
    isReady: wsService.isReady.bind(wsService),
    /** 更换地址并重连 */
    setUrl: wsService.setUrl.bind(wsService),
    /** 发送消息（对象自动 JSON.stringify） */
    send: wsService.send.bind(wsService),
    /** 销毁连接（清空回调 + 关闭 + 置空） */
    destroy: wsService.destroy.bind(wsService),
  };
}

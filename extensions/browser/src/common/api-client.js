// PicoClaw 浏览器扩展 - API 客户端
// 处理与 PicoClaw Gateway 浏览器频道的 WebSocket 通信

class PicoClawAPI {
  constructor() {
    this.ws = null;
    this.messageId = 0;
  }

  // 使用存储的设置初始化
  async init() {
    const result = await chrome.storage.sync.get([
      CONFIG.STORAGE.GATEWAY_URL,
      CONFIG.STORAGE.PICO_TOKEN
    ]);

    this.gatewayUrl = (result[CONFIG.STORAGE.GATEWAY_URL] || CONFIG.PICOCAW.DEFAULT_GATEWAY_URL).replace(/\/+$/, '');
    this.picoToken = result[CONFIG.STORAGE.PICO_TOKEN] || '';

    return this;
  }

  // 更新 Gateway 地址
  setGatewayUrl(url) {
    this.gatewayUrl = url.replace(/\/$/, '');
  }

  // 更新 Pico Token
  setPicoToken(token) {
    this.picoToken = token;
  }

  // 连接 WebSocket 进行实时通信
  // 使用专用浏览器频道，通过 Sec-WebSocket-Protocol: token.<value> 认证，无需 Cookie
  async connectWebSocket(sessionId, onMessage, onConnect, onDisconnect) {
    console.log('[PicoClaw] 创建 WebSocket 连接...');

    if (this.ws) {
      console.log('[PicoClaw] 关闭已有 WebSocket 连接');
      this.ws.close();
    }

    const gatewayUrl = (this.gatewayUrl || CONFIG.PICOCAW.DEFAULT_GATEWAY_URL).replace(/\/+$/, '');
    const token = this.picoToken;

    if (!token) {
      throw new Error('Token is required. Please configure it in settings.');
    }

    const wsProtocol = gatewayUrl.startsWith('https') ? 'wss' : 'ws';
    const gatewayHost = gatewayUrl.replace(/^https?:\/\//, '').replace(/\/+$/, '');
    const url = `${wsProtocol}://${gatewayHost}${CONFIG.PICOCAW.WS_ENDPOINT}?session_id=${encodeURIComponent(sessionId)}`;
    console.log('[PicoClaw] 使用 Token 认证连接浏览器频道:', url);

    this.ws = new WebSocket(url, ['token.' + token]);

    this.ws.onopen = () => {
      console.log('[PicoClaw] WebSocket 连接成功');
      if (onConnect) onConnect();
    };

    this.ws.onmessage = (event) => {
      console.log('[PicoClaw] 收到 WebSocket 消息:', event.data);
      try {
        const data = JSON.parse(event.data);
        if (onMessage) onMessage(data);
      } catch (error) {
        console.error('[PicoClaw] 解析 WebSocket 消息失败:', error);
      }
    };

    this.ws.onclose = (event) => {
      console.log('[PicoClaw] WebSocket 已断开:', event.code, event.reason);
      if (onDisconnect) onDisconnect();
    };

    this.ws.onerror = (error) => {
      console.error('[PicoClaw] WebSocket 错误:', error);
    };
  }

  // 断开 WebSocket
  disconnectWebSocket() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  // 通过 WebSocket 发送消息（PicoClaw 协议格式）
  // 格式: { type: "message.send", id: "...", payload: { content: "...", media: [] } }
  sendChatMessage(content, attachments = []) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket not connected');
    }

    const id = `msg-${++this.messageId}-${Date.now()}`;
    const message = {
      type: 'message.send',
      id: id,
      payload: {
        content: content.trim(),
        media: attachments || []
      }
    };

    this.ws.send(JSON.stringify(message));
    return id;
  }
}

// 创建全局实例
const picoclawAPI = new PicoClawAPI();

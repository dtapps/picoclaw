// PicoClaw 浏览器扩展 - 配置文件

const CONFIG = {
  // PicoClaw 服务器配置
  PICOCAW: {
    // 默认 Gateway 地址，浏览器扩展通过 WebSocket 直连 Gateway
    // 使用 Sec-WebSocket-Protocol: token.<value> 进行认证
    DEFAULT_GATEWAY_URL: 'http://localhost:18790',

    // WebSocket 实时通信端点
    // 浏览器频道使用 /browser/ws 进行 WebSocket 聊天
    WS_ENDPOINT: '/browser/ws'
  },
  
  // 存储键名
  STORAGE: {
    GATEWAY_URL: 'picoclaw_gateway_url',
    PICO_TOKEN: 'picoclaw_pico_token',
    CHAT_HISTORY: 'picoclaw_chat_history',
    SETTINGS: 'picoclaw_settings'
  },
  
  // 内部通信消息类型
  MESSAGE_TYPES: {
    // 从弹窗到后台脚本
    SEND_MESSAGE: 'SEND_MESSAGE',
    GET_PAGE_INFO: 'GET_PAGE_INFO',
    EXECUTE_ACTION: 'EXECUTE_ACTION',
    
    // 从后台脚本到弹窗
    MESSAGE_RESPONSE: 'MESSAGE_RESPONSE',
    STREAM_MESSAGE: 'STREAM_MESSAGE',
    ERROR: 'ERROR',
    
    // 从内容脚本
    PAGE_INFO: 'PAGE_INFO',
    ACTION_RESULT: 'ACTION_RESULT',
    ELEMENT_CLICKED: 'ELEMENT_CLICKED'
  },
  
  // 浏览器可执行操作
  ACTIONS: {
    CLICK: 'click',
    TYPE: 'type',
    SCROLL: 'scroll',
    NAVIGATE: 'navigate',
    REFRESH: 'refresh',
    GO_BACK: 'goBack',
    GO_FORWARD: 'goForward',
    GET_TEXT: 'getText',
    GET_HTML: 'getHtml',
    SCREENSHOT: 'screenshot',
    FILL_FORM: 'fillForm',
    SELECT_OPTION: 'selectOption',
    HOVER: 'hover',
    FOCUS: 'focus',
    CLEAR: 'clear'
  }
};

// 导出供其他脚本使用
if (typeof module !== 'undefined' && module.exports) {
  module.exports = CONFIG;
}

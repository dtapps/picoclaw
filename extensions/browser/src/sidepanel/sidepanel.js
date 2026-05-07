// PicoClaw 浏览器扩展 - 侧边栏脚本
// 处理侧边栏聊天界面

// DOM 元素（在 init 中初始化）
let chatContainer, messageInput, sendBtn, settingsBtn, clearBtn;
let settingsModal, closeSettings, saveSettings, testConnection;
let picoTokenInput, gatewayUrlInput, autoConnectCheckbox;
let connectionStatus, statusDot, statusText;
let pageInfoPanel, togglePageInfo, pageInfoContent;

// 初始化 DOM 元素
function initDOMElements() {
  chatContainer = document.getElementById('chatContainer');
  messageInput = document.getElementById('messageInput');
  sendBtn = document.getElementById('sendBtn');
  settingsBtn = document.getElementById('settingsBtn');
  clearBtn = document.getElementById('clearBtn');
  settingsModal = document.getElementById('settingsModal');
  closeSettings = document.getElementById('closeSettings');
  saveSettings = document.getElementById('saveSettings');
  testConnection = document.getElementById('testConnection');
  autoConnectCheckbox = document.getElementById('autoConnect');
  picoTokenInput = document.getElementById('picoToken');
  gatewayUrlInput = document.getElementById('gatewayUrl');
  connectionStatus = document.getElementById('connectionStatus');
  statusDot = connectionStatus?.querySelector('.status-dot');
  statusText = connectionStatus?.querySelector('.status-text');
  pageInfoPanel = document.getElementById('pageInfoPanel');
  togglePageInfo = document.getElementById('togglePageInfo');
  pageInfoContent = document.getElementById('pageInfoContent');

  // 检查是否所有元素都找到了
  const elements = {
    chatContainer, messageInput, sendBtn, settingsBtn, clearBtn,
    settingsModal, closeSettings, saveSettings, testConnection,
    picoTokenInput, gatewayUrlInput, autoConnectCheckbox,
    connectionStatus, pageInfoPanel, togglePageInfo, pageInfoContent
  };

  for (const [name, element] of Object.entries(elements)) {
    if (!element) {
      console.error(`[PicoClaw] DOM element not found: ${name}`);
    }
  }
}

// 状态变量
let isConnected = false;
let isWsConnected = false;
let isProcessing = false;
let chatHistory = [];
let saveHistoryTimer = null;
let currentPageInfo = null;
let currentSessionId = null;
let sidepanelAPI = null;

// 生成会话 ID
function generateSessionId() {
  return 'ext-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
}

// 初始化
async function init() {
  console.log('[PicoClaw] Initializing side panel...');

  // 先初始化 DOM 元素
  initDOMElements();
  console.log('[PicoClaw] DOM elements initialized');

  // 无论是否出错，都隐藏加载提示，让用户可以操作界面
  const hideLoadingIndicator = () => {
    const li = document.getElementById('loadingIndicator');
    if (li) li.style.display = 'none';
  };

  try {
    setupEventListeners();
    console.log('[PicoClaw] Event listeners setup');

    // 初始化 API 客户端（sidepanel 上下文）
    sidepanelAPI = new PicoClawAPI();
    await sidepanelAPI.init();
    console.log('[PicoClaw] API client initialized');

    await loadSettings();
    console.log('[PicoClaw] Settings loaded');
  } catch (error) {
    console.error('[PicoClaw] Init step failed:', error);
  }

  // Always hide loading indicator so user can interact with UI
  hideLoadingIndicator();

  // 这些操作非关键，失败也不阻塞界面
  try {
    await checkConnection();
  } catch (e) { console.warn('[PicoClaw] checkConnection failed:', e); }

  try {
    await connectWebSocket();
    console.log('[PicoClaw] WebSocket connected');
  } catch (e) {
    console.warn('[PicoClaw] WebSocket not connected:', e.message);
  }

  try {
    await loadPageInfo();
  } catch (e) { console.warn('[PicoClaw] loadPageInfo failed:', e); }

  try {
    loadChatHistory();
  } catch (e) { console.warn('[PicoClaw] loadChatHistory failed:', e); }

  // 监听标签页变化以更新页面信息
  try {
    chrome.tabs.onActivated.addListener(handleTabChange);
    chrome.tabs.onUpdated.addListener(handleTabUpdate);
  } catch (e) { /* ignore */ }

  console.log('[PicoClaw] Initialization complete');
}

// WebSocket 连接，使用 Sec-WebSocket-Protocol: token.<value> 认证
async function connectWebSocket() {
  if (isWsConnected) return;

  // 初始化 API client for sidepanel context
  if (!sidepanelAPI) {
    sidepanelAPI = new PicoClawAPI();
    await sidepanelAPI.init();
  }

  if (!sidepanelAPI.picoToken) {
    showNotification('请先配置 Token', 'error');
    settingsModal.classList.add('show');
    updateConnectionStatus('disconnected');
    throw new Error('Token not configured');
  }

  const sessionId = currentSessionId || generateSessionId();

  await new Promise((resolve, reject) => {
    sidepanelAPI.connectWebSocket(
      sessionId,
      (data) => handleWebSocketMessage(data),
      () => {
        console.log('[PicoClaw] WebSocket connected');
        isWsConnected = true;
        currentSessionId = sessionId;
        updateConnectionStatus('connected');
        resolve();
      },
      () => {
        console.log('[PicoClaw] WebSocket disconnected');
        isWsConnected = false;
        updateConnectionStatus('disconnected');
      }
    );

    // 连接超时 10 秒
    setTimeout(() => {
      if (!isWsConnected) {
        reject(new Error('WebSocket connection timeout'));
      }
    }, 10000);
  });
}

// 处理 Pico Protocol 格式的 WebSocket 消息
function handleWebSocketMessage(data) {
  switch (data.type) {
    case 'pong':
      break;

    case 'message.create':
      handleAssistantMessage(data);
      break;

    case 'message.update':
      handleAssistantMessage(data);
      break;

    case 'message.delete':
      // 从 UI 移除消息
      break;

    case 'typing.start':
      showLoading();
      break;

    case 'typing.stop':
      hideLoading();
      break;

    case 'tool_feedback':
      if (data.payload) {
        updateAssistantMessage(data.payload);
      }
      break;

    case 'action.request':
      handleActionRequest(data);
      break;

    case 'error':
      console.error('[PicoClaw] 服务器错误:', data.payload);
      hideLoading();
      if (data.payload && data.payload.message) {
        showNotification('错误: ' + data.payload.message, 'error');
      }
      break;

    default:
      console.log('[PicoClaw] 未知消息类型:', data.type);
  }
}

// 处理 Pico Protocol 的助手消息
let currentAssistantMsgId = null;

function handleAssistantMessage(data) {
  const payload = data.payload || {};
  const msgId = payload.message_id || data.id;
  const content = payload.content || '';
  const kind = payload.kind || '';

  // 跳过思考消息
  if (kind === 'thought' || payload.thought) {
    return;
  }

  // 工具调用消息：只显示摘要，不执行（执行由 action.request 协议处理）
  if (kind === 'tool_calls' && payload.tool_calls) {
    const calls = payload.tool_calls;
    let summary = '';
    for (const call of calls) {
      const name = call.name || call.tool || 'unknown';
      const args = call.params || call.arguments || {};
      summary += `🔧 ${name}`;
      if (args.action) summary += ` → ${args.action}`;
      summary += '\n';
    }
    // 用 msgId 去重，避免重复显示
    const existingMsg = document.querySelector(`[data-msg-id="${msgId}"]`);
    if (!existingMsg && summary) {
      addMessage('assistant', summary.trim(), true);
    }
    return;
  }

  // 更新已有消息
  if (currentAssistantMsgId === msgId) {
    updateAssistantMessage(payload);
    return;
  }

  // 新消息
  currentAssistantMsgId = msgId;
  hideLoading();

  if (content) {
    hideLoading();
    const existingMsg = document.querySelector(`[data-msg-id="${msgId}"]`);
    if (existingMsg) {
      const contentDiv = existingMsg.querySelector('.message-content');
      if (contentDiv) {
        contentDiv.innerHTML = renderMarkdown(content);
      }
    } else {
      addMessage('assistant', content, true);
    }
  }
}

// 更新助手消息内容
function updateAssistantMessage(payload) {
  const msgId = payload.message_id;
  const content = payload.content || '';

  if (!content) return;

  const existingMsg = document.querySelector(`[data-msg-id="${msgId}"]`);
  if (existingMsg) {
    const contentDiv = existingMsg.querySelector('.message-content');
    if (contentDiv) {
      contentDiv.innerHTML = renderMarkdown(content);
    }
  } else {
    addMessage('assistant', content, true);
  }
}

// 处理服务器发来的浏览器操作请求
async function handleActionRequest(data) {
  const payload = data.payload || {};
  const requestID = payload.request_id;
  const action = payload.action;
  const params = payload.params || {};

  if (!requestID || !action) {
    console.error('[PicoClaw] action.request 缺少 request_id 或 action');
    return;
  }

  console.log('[PicoClaw] 收到操作请求:', action, params);

  let result;
  try {
    // 通过 background script 执行操作
    const response = await chrome.runtime.sendMessage({
      type: 'EXECUTE_ACTION',
      data: { action, params }
    });

    if (response && response.success) {
      result = response.data || { success: true, message: `操作完成: ${action}` };
    } else {
      result = { error: (response && response.error) || '操作执行失败' };
    }
  } catch (error) {
    console.error('[PicoClaw] 操作执行失败:', error);
    result = { error: error.message || '操作执行异常' };
  }

  // 发送结果回服务器
  if (sidepanelAPI && sidepanelAPI.ws && sidepanelAPI.ws.readyState === WebSocket.OPEN) {
    sidepanelAPI.ws.send(JSON.stringify({
      type: 'action.result',
      id: requestID,
      payload: {
        request_id: requestID,
        ...result
      }
    }));
  }
}

// 设置事件监听器
function setupEventListeners() {
  if (!sendBtn || !messageInput) {
    console.error('[PicoClaw] Cannot setup event listeners: elements not found');
    return;
  }

  // 初始启用发送按钮
  sendBtn.disabled = false;

  // 发送消息
  sendBtn.addEventListener('click', (e) => {
    console.log('[PicoClaw] Send button clicked');
    e.preventDefault();
    sendMessage();
  });

  messageInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  });

  // 输入框自动调整高度
  messageInput.addEventListener('input', () => {
    const isEmpty = messageInput.value.trim() === '';
    sendBtn.disabled = isEmpty || isProcessing;
    messageInput.style.height = 'auto';
    messageInput.style.height = Math.min(messageInput.scrollHeight, 120) + 'px';
  });

  // 初始化按钮状态
  sendBtn.disabled = messageInput.value.trim() === '' || isProcessing;
  
  // 设置弹窗
  settingsBtn.addEventListener('click', () => {
    settingsModal.classList.add('show');
    loadSettings();
  });
  
  closeSettings.addEventListener('click', () => {
    settingsModal.classList.remove('show');
  });
  
  saveSettings.addEventListener('click', saveSettingsHandler);
  testConnection.addEventListener('click', testConnectionHandler);
  
  // 清空对话
  clearBtn.addEventListener('click', clearChat);
  
  // 切换页面信息面板
  togglePageInfo.addEventListener('click', () => {
    pageInfoPanel.classList.toggle('collapsed');
  });
  
  // 点击弹窗外部关闭
  settingsModal.addEventListener('click', (e) => {
    if (e.target === settingsModal) {
      settingsModal.classList.remove('show');
    }
  });
}

// 处理标签页切换
async function handleTabChange(activeInfo) {
  await loadPageInfo();
}

// 处理标签页更新
async function handleTabUpdate(tabId, changeInfo, tab) {
  if (changeInfo.status === 'complete') {
    await loadPageInfo();
  }
}

// 从存储加载设置
async function loadSettings() {
  try {
    const result = await chrome.storage.sync.get([
      CONFIG.STORAGE.GATEWAY_URL,
      CONFIG.STORAGE.PICO_TOKEN,
      CONFIG.STORAGE.SETTINGS
    ]);

    const gatewayUrl = result[CONFIG.STORAGE.GATEWAY_URL] || CONFIG.PICOCAW.DEFAULT_GATEWAY_URL;
    const picoToken = result[CONFIG.STORAGE.PICO_TOKEN] || '';
    const settings = result[CONFIG.STORAGE.SETTINGS] || {};

    if (gatewayUrlInput) gatewayUrlInput.value = gatewayUrl;
    if (picoTokenInput) picoTokenInput.value = picoToken;
    if (autoConnectCheckbox) autoConnectCheckbox.checked = settings.autoConnect || false;
  } catch (error) {
    console.error('[PicoClaw] Failed to load settings:', error);
  }
}

// 保存设置
async function saveSettingsHandler() {
  const gatewayUrl = gatewayUrlInput.value.trim();
  const picoToken = picoTokenInput.value.trim();
  const autoConnect = autoConnectCheckbox.checked;

  try {
    const storageData = {
      [CONFIG.STORAGE.SETTINGS]: { autoConnect }
    };
    if (gatewayUrl) storageData[CONFIG.STORAGE.GATEWAY_URL] = gatewayUrl;
    if (picoToken) storageData[CONFIG.STORAGE.PICO_TOKEN] = picoToken;
    await chrome.storage.sync.set(storageData);

    showNotification('设置已保存', 'success');

    // 更新 sidepanel API 客户端
    await checkConnection();
  } catch (error) {
    showNotification('保存失败: ' + error.message, 'error');
  }
}

// 测试连接
async function testConnectionHandler() {
  const gatewayUrl = gatewayUrlInput.value.trim();
  const picoToken = picoTokenInput.value.trim();

  if (!picoToken) {
    showNotification('请输入 Token', 'error');
    return;
  }

  if (!gatewayUrl) {
    showNotification('请输入 Gateway 地址', 'error');
    return;
  }

  testConnection.disabled = true;
  testConnection.textContent = '测试中...';

  try {
    // 先保存设置
    const storageData = {
      [CONFIG.STORAGE.SETTINGS]: { autoConnect: autoConnectCheckbox.checked }
    };
    if (gatewayUrl) storageData[CONFIG.STORAGE.GATEWAY_URL] = gatewayUrl;
    if (picoToken) storageData[CONFIG.STORAGE.PICO_TOKEN] = picoToken;
    await chrome.storage.sync.set(storageData);

    // 更新 sidepanel API 客户端
    if (sidepanelAPI) {
      sidepanelAPI.setGatewayUrl(gatewayUrl || CONFIG.PICOCAW.DEFAULT_GATEWAY_URL);
      sidepanelAPI.setPicoToken(picoToken);
    } else {
      sidepanelAPI = new PicoClawAPI();
      await sidepanelAPI.init();
    }

    // 通过 WebSocket 连接来测试
    try {
      await connectWebSocket();
      showNotification('连接成功！', 'success');
    } catch (wsError) {
      showNotification('连接失败: ' + wsError.message, 'error');
    }
  } catch (error) {
    showNotification('连接失败: ' + error.message, 'error');
  } finally {
    testConnection.disabled = false;
    testConnection.textContent = '测试连接';
  }
}

// 检查连接状态
async function checkConnection() {
  updateConnectionStatus('checking');

  try {
    // 按需初始化 API 客户端
    if (!sidepanelAPI) {
      sidepanelAPI = new PicoClawAPI();
      await sidepanelAPI.init();
    }

    // 仅检查 WebSocket 状态 — 浏览器频道只用 token 认证
    isWsConnected = sidepanelAPI.ws && sidepanelAPI.ws.readyState === WebSocket.OPEN;
    isConnected = isWsConnected;
    updateConnectionStatus(isWsConnected ? 'connected' : 'disconnected');
  } catch (error) {
    isConnected = false;
    isWsConnected = false;
    updateConnectionStatus('disconnected');
  }
}

// 更新连接状态 UI
function updateConnectionStatus(status) {
  statusDot.className = 'status-dot';
  
  switch (status) {
    case 'connected':
      statusDot.classList.add('connected');
      statusText.textContent = '已连接';
      break;
    case 'disconnected':
      statusDot.classList.add('disconnected');
      statusText.textContent = '未连接';
      break;
    case 'checking':
      statusText.textContent = '检查中...';
      break;
    default:
      statusText.textContent = '未知';
  }
}

// 加载页面信息
async function loadPageInfo() {
  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    
    if (!tab || tab.url.startsWith('chrome://') || tab.url.startsWith('edge://')) {
      pageInfoContent.innerHTML = '<p class="loading-text">无法访问此页面</p>';
      return;
    }
    
    const response = await chrome.runtime.sendMessage({
      type: 'GET_PAGE_INFO'
    });
    
    if (response.success) {
      currentPageInfo = response.data;
      renderPageInfo(response.data);
    } else {
      pageInfoContent.innerHTML = `<p class="loading-text">获取页面信息失败: ${response.error}</p>`;
    }
  } catch (error) {
    pageInfoContent.innerHTML = `<p class="loading-text">错误: ${error.message}</p>`;
  }
}

// 渲染页面信息
function renderPageInfo(info) {
  const items = [
    { label: '标题', value: info.title },
    { label: 'URL', value: info.url },
    { label: '域名', value: info.domain },
    { label: '按钮数', value: info.buttons?.length || 0 },
    { label: '输入框数', value: info.inputs?.length || 0 },
    { label: '链接数', value: info.links?.length || 0 }
  ];
  
  pageInfoContent.innerHTML = items.map(item => `
    <div class="page-info-item">
      <span class="page-info-label">${item.label}:</span>
      <span class="page-info-value" title="${item.value}">${item.value}</span>
    </div>
  `).join('');
}

// 通过 WebSocket 发送消息
async function sendMessage() {
  const message = messageInput.value.trim();

  if (!message || isProcessing) return;

  if (!isWsConnected || !sidepanelAPI || !sidepanelAPI.ws || sidepanelAPI.ws.readyState !== WebSocket.OPEN) {
    showNotification('未连接到服务器，请检查设置', 'error');
    return;
  }

  // 添加用户消息到聊天
  addMessage('user', message);

  // 清空输入框
  messageInput.value = '';
  messageInput.style.height = 'auto';
  sendBtn.disabled = true;

  try {
    // 发送消息，附带当前页面上下文
    sidepanelAPI.sendChatMessage(message);
  } catch (error) {
    hideLoading();
    addMessage('assistant', `错误: ${error.message}`, true);
  } finally {
    isProcessing = false;
    sendBtn.disabled = messageInput.value.trim() === '';
  }
}

// 添加消息到聊天界面
function addMessage(role, content, isMarkdown = false) {
  const messageDiv = document.createElement('div');
  messageDiv.className = `message ${role}`;

  const contentDiv = document.createElement('div');
  contentDiv.className = 'message-content';

  if (isMarkdown) {
    contentDiv.innerHTML = renderMarkdown(content);
  } else {
    contentDiv.textContent = content;
  }

  messageDiv.appendChild(contentDiv);

  if (role === 'assistant') {
    const actionsDiv = document.createElement('div');
    actionsDiv.className = 'message-actions';
    actionsDiv.innerHTML = `
      <button class="message-action-btn" onclick="copyMessage(this)" title="复制">📋</button>
    `;
    messageDiv.appendChild(actionsDiv);
  }

  chatContainer.appendChild(messageDiv);
  scrollToBottom();

  chatHistory.push({ role, content, timestamp: Date.now() });
  // 防抖保存，避免频繁写 storage
  if (saveHistoryTimer) clearTimeout(saveHistoryTimer);
  saveHistoryTimer = setTimeout(saveChatHistory, 1000);
}

// 简易 Markdown 渲染
function renderMarkdown(text) {
  return text
    .replace(/```(\w+)?\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
    .replace(/\n/g, '<br>');
}

// 显示加载动画
function showLoading() {
  const loadingDiv = document.createElement('div');
  loadingDiv.className = 'message assistant loading-message';
  loadingDiv.innerHTML = `
    <div class="loading">
      <div class="loading-dot"></div>
      <div class="loading-dot"></div>
      <div class="loading-dot"></div>
    </div>
  `;
  chatContainer.appendChild(loadingDiv);
  scrollToBottom();
}

// 隐藏加载动画
function hideLoading() {
  const loadingMessage = chatContainer.querySelector('.loading-message');
  if (loadingMessage) {
    loadingMessage.remove();
  }
}

// 滚动到底部
function scrollToBottom() {
  chatContainer.scrollTop = chatContainer.scrollHeight;
}

// 复制消息到剪贴板
function copyMessage(btn) {
  const messageContent = btn.closest('.message').querySelector('.message-content');
  const text = messageContent.textContent;
  
  navigator.clipboard.writeText(text).then(() => {
    const original = btn.textContent;
    btn.textContent = '✅';
    setTimeout(() => {
      btn.textContent = original;
    }, 2000);
  });
}

// 清空聊天记录
function clearChat() {
  if (confirm('确定要清空所有对话吗？')) {
    chatHistory = [];
    chatContainer.innerHTML = `
      <div class="message system-message">
        <div class="message-content">
          <p>👋 你好！我是 PicoClaw 浏览器助手。</p>
          <p>对话已清空，让我们重新开始吧！</p>
        </div>
      </div>
    `;
    saveChatHistory();
  }
}

// 保存聊天记录到存储
async function saveChatHistory() {
  try {
    await chrome.storage.local.set({
      'picoclaw_chat_history': chatHistory.slice(-50)
    });
  } catch (error) {
    console.error('Failed to save chat history:', error);
  }
}

// 从存储加载聊天记录
async function loadChatHistory() {
  try {
    const result = await chrome.storage.local.get('picoclaw_chat_history');
    if (result.picoclaw_chat_history) {
      chatHistory = result.picoclaw_chat_history;

      if (chatHistory.length > 0) {
        // 直接渲染 DOM，不走 addMessage 避免触发 saveChatHistory
        chatContainer.innerHTML = '';
        for (const msg of chatHistory) {
          const messageDiv = document.createElement('div');
          messageDiv.className = `message ${msg.role}`;
          const contentDiv = document.createElement('div');
          contentDiv.className = 'message-content';
          if (msg.role === 'assistant') {
            contentDiv.innerHTML = renderMarkdown(msg.content);
          } else {
            contentDiv.textContent = msg.content;
          }
          messageDiv.appendChild(contentDiv);
          chatContainer.appendChild(messageDiv);
        }
        scrollToBottom();
      }
    }
  } catch (error) {
    console.error('Failed to load chat history:', error);
  }
}

// 显示通知
function showNotification(message, type = 'info') {
  const notification = document.createElement('div');
  notification.className = type === 'error' ? 'error-message' : 'success-message';
  notification.textContent = message;
  notification.style.cssText = `
    position: fixed;
    top: 60px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 2000;
    max-width: 90%;
  `;
  
  document.body.appendChild(notification);
  
  setTimeout(() => {
    notification.remove();
  }, 3000);
}

  // DOM 就绪后初始化
document.addEventListener('DOMContentLoaded', init);

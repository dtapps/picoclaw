// PicoClaw 浏览器扩展 - 后台服务 Worker
// 处理浏览器操作和设置管理
// WebSocket 连接由 sidepanel 页面上下文管理

importScripts('../common/config.js', '../common/api-client.js');

// 初始化 API 客户端
let apiClient = null;

// 初始化扩展
chrome.runtime.onInstalled.addListener((details) => {
  console.log('PicoClaw 浏览器扩展安装/更新:', details.reason);

  if (details.reason === 'install') {
    chrome.storage.sync.set({
      [CONFIG.STORAGE.SETTINGS]: {
        autoConnect: false,
        showNotifications: true
      }
    });
  }

  // 清理旧版本的存储键
  if (details.reason === 'update') {
    chrome.storage.sync.remove([
      'picoclaw_server_url',
      'picoclaw_password'
    ]);
    // 从设置中移除 rememberPassword
    chrome.storage.sync.get(CONFIG.STORAGE.SETTINGS, (result) => {
      const settings = result[CONFIG.STORAGE.SETTINGS];
      if (settings && 'rememberPassword' in settings) {
        delete settings.rememberPassword;
        chrome.storage.sync.set({ [CONFIG.STORAGE.SETTINGS]: settings });
      }
    });
  }
});

// 点击扩展图标 - 打开侧边栏
chrome.action.onClicked.addListener(async (tab) => {
  await chrome.sidePanel.open({ windowId: tab.windowId });
});

// 启动时初始化 API 客户端
async function initAPIClient() {
  apiClient = new PicoClawAPI();
  await apiClient.init();
  return apiClient;
}

// 处理来自 sidepanel 和 content script 的消息
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  const handleAsync = async () => {
    if (!apiClient) {
      await initAPIClient();
    }

    try {
      switch (request.type) {
        case CONFIG.MESSAGE_TYPES.GET_PAGE_INFO:
          return await handleGetPageInfo(sender.tab);

        case CONFIG.MESSAGE_TYPES.EXECUTE_ACTION:
          return await handleExecuteAction(request.data, sender.tab);

        case 'GET_SETTINGS':
          return await handleGetSettings();

        case 'SAVE_SETTINGS':
          return await handleSaveSettings(request.data);

        default:
          return { success: false, error: '未知的消息类型' };
      }
    } catch (error) {
      console.error('后台脚本错误:', error);
      return { success: false, error: error.message };
    }
  };

  handleAsync().then(sendResponse);
  return true;
});

// 获取页面信息
async function handleGetPageInfo(tab) {
  // sidepanel 发消息时没有 sender.tab，需要主动查询
  if (!tab) {
    const [activeTab] = await chrome.tabs.query({ active: true, currentWindow: true });
    tab = activeTab;
  }

  if (!tab) {
    return { success: false, error: '没有活动标签页' };
  }

  // 跳过浏览器内部页面
  if (tab.url && (tab.url.startsWith('chrome://') || tab.url.startsWith('edge://') || tab.url.startsWith('chrome-extension://'))) {
    return { success: false, error: '无法访问浏览器内部页面' };
  }

  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: () => {
        // 辅助函数：截断长文本
        const trunc = (s, max = 80) => s && s.length > max ? s.slice(0, max) + '...' : s;

        const allButtons = Array.from(document.querySelectorAll('button, [role="button"], input[type="submit"]'));
        const allInputs = Array.from(document.querySelectorAll('input, textarea, select'));
        const allLinks = Array.from(document.querySelectorAll('a[href]'));

        const buttons = allButtons.slice(0, 20).map((btn, i) => ({
          index: i,
          text: trunc(btn.textContent?.trim() || btn.value || ''),
          id: btn.id || '',
          selector: btn.id ? `#${btn.id}` : (btn.name ? `button[name="${btn.name}"]` : `button:nth-of-type(${i + 1})`)
        }));
        const inputs = allInputs.slice(0, 20).map((input, i) => ({
          index: i,
          type: input.type || input.tagName.toLowerCase(),
          name: input.name || '',
          id: input.id || '',
          placeholder: trunc(input.placeholder || ''),
          label: trunc(input.labels?.[0]?.textContent?.trim() || ''),
          selector: input.id ? `#${input.id}` : (input.name ? `${input.tagName.toLowerCase()}[name="${input.name}"]` : '')
        }));
        const links = allLinks.slice(0, 20).map((link, i) => ({
          index: i,
          text: trunc(link.textContent?.trim() || ''),
          href: link.href || ''
        }));

        return {
          url: window.location.href,
          title: document.title,
          domain: window.location.hostname,
          description: trunc(document.querySelector('meta[name="description"]')?.content || '', 200),
          buttons,
          button_count: allButtons.length,
          inputs,
          input_count: allInputs.length,
          links,
          link_count: allLinks.length
        };
      }
    });

    return { success: true, data: results[0].result };
  } catch (error) {
    return { success: false, error: error.message };
  }
}

// 处理浏览器操作执行
async function handleExecuteAction(data, senderTab) {
  // 获取活动标签页 - sidepanel 消息没有 sender.tab
  let tab = senderTab;
  if (!tab) {
    const [activeTab] = await chrome.tabs.query({ active: true, currentWindow: true });
    tab = activeTab;
  }

  if (!tab) {
    return { success: false, error: '没有活动标签页' };
  }

  const { action, params } = data;

  try {
    let result;

    // 同时支持 snake_case（browser_ext 工具发送）和 camelCase（原有格式）
    switch (action) {
      case 'get_page_info':
        return await handleGetPageInfo(tab);

      case 'click':
        result = await executeClick(tab.id, params);
        break;

      case 'type':
        result = await executeType(tab.id, params);
        break;

      case 'fill':
      case 'fillForm':
        // fill: 单个字段（selector + text），fillForm: 多字段（fields 数组）
        if (params.fields) {
          result = await executeFillForm(tab.id, params);
        } else {
          result = await executeFillSingle(tab.id, params);
        }
        break;

      case 'scroll':
        result = await executeScroll(tab.id, params);
        break;

      case 'navigate':
        result = await executeNavigate(tab.id, params);
        break;

      case 'refresh':
        result = await executeRefresh(tab.id);
        break;

      case 'goBack':
        result = await executeGoBack(tab.id);
        break;

      case 'goForward':
        result = await executeGoForward(tab.id);
        break;

      case 'get_text':
      case 'getText':
        result = await executeGetText(tab.id, params);
        break;

      case 'getHtml':
        result = await executeGetHtml(tab.id);
        break;

      case 'screenshot':
        result = await executeScreenshot(tab.id);
        break;

      case 'select_option':
      case 'selectOption':
        result = await executeSelectOption(tab.id, params);
        break;

      case 'hover':
        result = await executeHover(tab.id, params);
        break;

      case 'focus':
        result = await executeFocus(tab.id, params);
        break;

      case 'clear':
        result = await executeClear(tab.id, params);
        break;

      default:
        return { success: false, error: `未知操作: ${action}` };
    }

    return { success: true, data: result };
  } catch (error) {
    return { success: false, error: error.message };
  }
}

// 获取设置
async function handleGetSettings() {
  const result = await chrome.storage.sync.get([
    CONFIG.STORAGE.GATEWAY_URL,
    CONFIG.STORAGE.PICO_TOKEN,
    CONFIG.STORAGE.SETTINGS
  ]);

  return {
    success: true,
    data: {
      gatewayUrl: result[CONFIG.STORAGE.GATEWAY_URL] || CONFIG.PICOCAW.DEFAULT_GATEWAY_URL,
      picoToken: result[CONFIG.STORAGE.PICO_TOKEN] || '',
      settings: result[CONFIG.STORAGE.SETTINGS] || {}
    }
  };
}

// 保存设置
async function handleSaveSettings(data) {
  const { gatewayUrl, picoToken, settings } = data;
  const storageData = {};

  if (gatewayUrl !== undefined) storageData[CONFIG.STORAGE.GATEWAY_URL] = gatewayUrl;
  if (picoToken !== undefined) storageData[CONFIG.STORAGE.PICO_TOKEN] = picoToken;
  if (settings !== undefined) storageData[CONFIG.STORAGE.SETTINGS] = settings;

  await chrome.storage.sync.set(storageData);

  // 更新 API 客户端
  if (apiClient) {
    if (gatewayUrl) apiClient.setGatewayUrl(gatewayUrl);
    if (picoToken !== undefined) apiClient.setPicoToken(picoToken);
  }

  return { success: true };
}

// ===== 操作实现 =====

// 点击元素
async function executeClick(tabId, params) {
  const { selector, index } = params;

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: (sel, idx) => {
      let element;
      if (idx !== undefined) {
        const elements = document.querySelectorAll(sel);
        element = elements[idx];
      } else {
        element = document.querySelector(sel);
      }

      if (element) {
        element.click();
        return { success: true, message: `已点击元素: ${sel}` };
      }
      return { success: false, error: `元素未找到: ${sel}` };
    },
    args: [selector, index]
  });

  return results[0].result;
}

// 输入文本
async function executeType(tabId, params) {
  const { selector, text, index } = params;

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: (sel, txt, idx) => {
      let element;
      if (idx !== undefined) {
        const elements = document.querySelectorAll(sel);
        element = elements[idx];
      } else {
        element = document.querySelector(sel);
      }

      if (element) {
        element.focus();
        element.value = txt;
        element.dispatchEvent(new Event('input', { bubbles: true }));
        element.dispatchEvent(new Event('change', { bubbles: true }));
        return { success: true, message: `已输入文本到: ${sel}` };
      }
      return { success: false, error: `元素未找到: ${sel}` };
    },
    args: [selector, text, index]
  });

  return results[0].result;
}

// 滚动页面
async function executeScroll(tabId, params) {
  const { direction, amount } = params;

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: (dir, amt) => {
      const scrollAmount = amt || 500;
      if (dir === 'up') {
        window.scrollBy(0, -scrollAmount);
      } else if (dir === 'down') {
        window.scrollBy(0, scrollAmount);
      } else if (dir === 'top') {
        window.scrollTo(0, 0);
      } else if (dir === 'bottom') {
        window.scrollTo(0, document.body.scrollHeight);
      }
      return { success: true, message: `已滚动: ${dir}` };
    },
    args: [direction, amount]
  });

  return results[0].result;
}

// 导航到指定 URL
async function executeNavigate(tabId, params) {
  const { url } = params;
  await chrome.tabs.update(tabId, { url });
  return { success: true, message: `已导航到: ${url}` };
}

// 刷新页面
async function executeRefresh(tabId) {
  await chrome.tabs.reload(tabId);
  return { success: true, message: '页面已刷新' };
}

// 后退
async function executeGoBack(tabId) {
  await chrome.tabs.goBack(tabId);
  return { success: true, message: '已后退' };
}

// 前进
async function executeGoForward(tabId) {
  await chrome.tabs.goForward(tabId);
  return { success: true, message: '已前进' };
}

// 获取页面文本
async function executeGetText(tabId, params) {
  const { selector } = params;

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: (sel) => {
      if (sel) {
        const element = document.querySelector(sel);
        return element ? element.textContent : '';
      }
      return document.body.innerText;
    },
    args: [selector]
  });

  return { success: true, text: results[0].result };
}

// 获取页面 HTML
async function executeGetHtml(tabId) {
  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: () => document.documentElement.outerHTML
  });

  return { success: true, html: results[0].result };
}

// 批量填充表单
async function executeFillForm(tabId, params) {
  const { fields } = params;

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: (formFields) => {
      const results = [];
      for (const { selector, value } of formFields) {
        const element = document.querySelector(selector);
        if (element) {
          element.value = value;
          element.dispatchEvent(new Event('input', { bubbles: true }));
          element.dispatchEvent(new Event('change', { bubbles: true }));
          results.push({ selector, success: true });
        } else {
          results.push({ selector, success: false, error: '未找到' });
        }
      }
      return results;
    },
    args: [fields]
  });

  return { success: true, results: results[0].result };
}

// 单个字段填充（browser_ext 工具的 fill action）
async function executeFillSingle(tabId, params) {
  const { selector, text, index } = params;

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: (sel, txt, idx) => {
      let element;
      if (idx !== undefined) {
        const elements = document.querySelectorAll(sel);
        element = elements[idx];
      } else {
        element = document.querySelector(sel);
      }

      if (element) {
        element.focus();
        element.value = txt;
        element.dispatchEvent(new Event('input', { bubbles: true }));
        element.dispatchEvent(new Event('change', { bubbles: true }));
        return { success: true, message: `已填充元素: ${sel}` };
      }
      return { success: false, error: `元素未找到: ${sel}` };
    },
    args: [selector, text, index]
  });

  return results[0].result;
}

// 截取页面可见区域截图
async function executeScreenshot(tabId) {
  const dataUrl = await chrome.tabs.captureVisibleTab(null, { format: 'png' });
  return { success: true, screenshot: dataUrl };
}

// 选择下拉选项
async function executeSelectOption(tabId, params) {
  const { selector, value, text } = params;
  // browser_ext 工具用 text 参数，兼容 value
  const optionValue = value || text;

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: (sel, val) => {
      const element = document.querySelector(sel);
      if (element) {
        element.value = val;
        element.dispatchEvent(new Event('change', { bubbles: true }));
        return { success: true };
      }
      return { success: false, error: '元素未找到' };
    },
    args: [selector, optionValue]
  });

  return results[0].result;
}

// 鼠标悬停
async function executeHover(tabId, params) {
  const { selector } = params;

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: (sel) => {
      const element = document.querySelector(sel);
      if (element) {
        const event = new MouseEvent('mouseover', { bubbles: true });
        element.dispatchEvent(event);
        return { success: true };
      }
      return { success: false, error: '元素未找到' };
    },
    args: [selector]
  });

  return results[0].result;
}

// 聚焦元素
async function executeFocus(tabId, params) {
  const { selector } = params;

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: (sel) => {
      const element = document.querySelector(sel);
      if (element) {
        element.focus();
        return { success: true };
      }
      return { success: false, error: '元素未找到' };
    },
    args: [selector]
  });

  return results[0].result;
}

// 清空输入框
async function executeClear(tabId, params) {
  const { selector } = params;

  const results = await chrome.scripting.executeScript({
    target: { tabId },
    func: (sel) => {
      const element = document.querySelector(sel);
      if (element) {
        element.value = '';
        element.dispatchEvent(new Event('input', { bubbles: true }));
        return { success: true };
      }
      return { success: false, error: '元素未找到' };
    },
    args: [selector]
  });

  return results[0].result;
}

// 启动时初始化
initAPIClient();

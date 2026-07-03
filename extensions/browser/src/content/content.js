// PicoClaw 浏览器扩展 - 内容脚本
// 在网页上下文中运行，实现浏览器控制功能

// 监听来自后台脚本的消息
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  const handleAsync = async () => {
    try {
      switch (request.type) {
        case 'GET_PAGE_INFO':
          return getPageInfo();
        
        case 'EXECUTE_ACTION':
          return executeAction(request.action, request.params);
        
        case 'HIGHLIGHT_ELEMENT':
          return highlightElement(request.selector, request.index);
        
        case 'GET_ELEMENT_INFO':
          return getElementInfo(request.selector, request.index);
        
        default:
          return { success: false, error: '未知的消息类型' };
      }
    } catch (error) {
      console.error('内容脚本错误:', error);
      return { success: false, error: error.message };
    }
  };

  handleAsync().then(sendResponse);
  return true;
});

// 获取页面综合信息
function getPageInfo() {
  return {
    url: window.location.href,
    title: document.title,
    domain: window.location.hostname,
    pathname: window.location.pathname,
    search: window.location.search,
    description: getMetaContent('description'),
    keywords: getMetaContent('keywords'),
    
    // 可交互元素
    buttons: getButtons(),
    inputs: getInputs(),
    links: getLinks(),
    selects: getSelects(),
    textareas: getTextareas(),
    
    // 页面结构
    headings: getHeadings(),
    images: getImages(),
    forms: getForms(),
    
    // 滚动信息
    scrollPosition: {
      x: window.scrollX,
      y: window.scrollY
    },
    pageHeight: document.body.scrollHeight,
    viewportHeight: window.innerHeight,
    
    // 时间戳
    timestamp: new Date().toISOString()
  };
}

// 获取 meta 标签内容
function getMetaContent(name) {
  const meta = document.querySelector(`meta[name="${name}"], meta[property="og:${name}"]`);
  return meta ? meta.content : '';
}

// 获取所有按钮
function getButtons() {
  const buttons = document.querySelectorAll('button, [role="button"], input[type="submit"], input[type="button"], .btn, [class*="button"]');
  return Array.from(buttons).slice(0, 50).map((btn, index) => ({
    index,
    tag: btn.tagName.toLowerCase(),
    text: getElementText(btn),
    id: btn.id || '',
    class: btn.className || '',
    type: btn.type || '',
    selector: generateSelector(btn),
    visible: isElementVisible(btn),
    disabled: btn.disabled || false
  }));
}

// 获取所有输入框
function getInputs() {
  const inputs = document.querySelectorAll('input:not([type="submit"]):not([type="button"]):not([type="hidden"])');
  return Array.from(inputs).slice(0, 50).map((input, index) => ({
    index,
    tag: input.tagName.toLowerCase(),
    type: input.type || 'text',
    name: input.name || '',
    id: input.id || '',
    class: input.className || '',
    placeholder: input.placeholder || '',
    value: input.value || '',
    label: getLabelText(input),
    selector: generateSelector(input),
    visible: isElementVisible(input),
    disabled: input.disabled || false,
    required: input.required || false
  }));
}

// 获取所有链接
function getLinks() {
  const links = document.querySelectorAll('a[href]');
  return Array.from(links).slice(0, 30).map((link, index) => ({
    index,
    text: getElementText(link),
    href: link.href,
    id: link.id || '',
    class: link.className || '',
    selector: generateSelector(link),
    visible: isElementVisible(link)
  }));
}

// 获取所有下拉选择框
function getSelects() {
  const selects = document.querySelectorAll('select');
  return Array.from(selects).map((select, index) => ({
    index,
    name: select.name || '',
    id: select.id || '',
    class: select.className || '',
    label: getLabelText(select),
    options: Array.from(select.options).map(opt => ({
      value: opt.value,
      text: opt.textContent.trim()
    })),
    selector: generateSelector(select),
    visible: isElementVisible(select),
    disabled: select.disabled || false
  }));
}

// 获取所有文本域
function getTextareas() {
  const textareas = document.querySelectorAll('textarea');
  return Array.from(textareas).map((textarea, index) => ({
    index,
    name: textarea.name || '',
    id: textarea.id || '',
    class: textarea.className || '',
    placeholder: textarea.placeholder || '',
    rows: textarea.rows,
    cols: textarea.cols,
    label: getLabelText(textarea),
    selector: generateSelector(textarea),
    visible: isElementVisible(textarea),
    disabled: textarea.disabled || false
  }));
}

// 获取所有标题
function getHeadings() {
  const headings = document.querySelectorAll('h1, h2, h3, h4, h5, h6');
  return Array.from(headings).map((heading, index) => ({
    index,
    level: parseInt(heading.tagName[1]),
    text: getElementText(heading),
    id: heading.id || '',
    selector: generateSelector(heading)
  }));
}

// 获取所有图片
function getImages() {
  const images = document.querySelectorAll('img');
  return Array.from(images).slice(0, 20).map((img, index) => ({
    index,
    src: img.src,
    alt: img.alt || '',
    width: img.width,
    height: img.height,
    selector: generateSelector(img)
  }));
}

// 获取所有表单
function getForms() {
  const forms = document.querySelectorAll('form');
  return Array.from(forms).map((form, index) => ({
    index,
    id: form.id || '',
    class: form.className || '',
    action: form.action || '',
    method: form.method || 'get',
    selector: generateSelector(form),
    inputCount: form.querySelectorAll('input, select, textarea').length
  }));
}

// 获取元素文本内容
function getElementText(element) {
  return element.textContent?.trim().substring(0, 100) || '';
}

// 获取输入框的标签文本
function getLabelText(input) {
  // 检查显式关联的 label
  if (input.id) {
    const label = document.querySelector(`label[for="${input.id}"]`);
    if (label) return getElementText(label);
  }
  
  // 检查父级 label
  const parentLabel = input.closest('label');
  if (parentLabel) {
    // 获取排除输入框本身的文本
    const clone = parentLabel.cloneNode(true);
    const inputs = clone.querySelectorAll('input, select, textarea');
    inputs.forEach(el => el.remove());
    return getElementText(clone);
  }
  
  // 检查 aria-label
  if (input.getAttribute('aria-label')) {
    return input.getAttribute('aria-label');
  }
  
  // 使用 placeholder 作为兜底
  return input.placeholder || '';
}

// 检查元素是否可见
function isElementVisible(element) {
  const rect = element.getBoundingClientRect();
  const style = window.getComputedStyle(element);
  
  return rect.width > 0 && 
         rect.height > 0 && 
         style.visibility !== 'hidden' && 
         style.display !== 'none' &&
         style.opacity !== '0';
}

// 为元素生成唯一 CSS 选择器
function generateSelector(element) {
  if (element.id) {
    return `#${element.id}`;
  }
  
  // 尝试基于 class 的选择器
  if (element.className) {
    const classes = element.className.split(' ').filter(c => c.trim());
    if (classes.length > 0) {
      const classSelector = `.${classes.join('.')}`;
      if (document.querySelectorAll(classSelector).length === 1) {
        return classSelector;
      }
    }
  }
  
  // 尝试标签+属性选择器
  let selector = element.tagName.toLowerCase();
  if (element.name) {
    selector += `[name="${element.name}"]`;
  }
  
  // 如有需要添加 nth-child
  const siblings = Array.from(element.parentNode?.children || []);
  const sameTagSiblings = siblings.filter(s => s.tagName === element.tagName);
  if (sameTagSiblings.length > 1) {
    const index = sameTagSiblings.indexOf(element) + 1;
    selector += `:nth-of-type(${index})`;
  }
  
  return selector;
}

// 在页面上执行各种操作
function executeAction(action, params) {
  switch (action) {
    case 'click':
      return clickElement(params.selector, params.index);
    
    case 'type':
      return typeText(params.selector, params.text, params.index);
    
    case 'scroll':
      return scrollPage(params.direction, params.amount);
    
    case 'getText':
      return getElementTextBySelector(params.selector);
    
    case 'getHtml':
      return { html: document.documentElement.outerHTML };
    
    case 'fillForm':
      return fillFormFields(params.fields);
    
    case 'selectOption':
      return selectOption(params.selector, params.value, params.index);
    
    case 'hover':
      return hoverElement(params.selector, params.index);
    
    case 'focus':
      return focusElement(params.selector, params.index);
    
    case 'clear':
      return clearElement(params.selector, params.index);
    
    default:
      return { success: false, error: `未知操作: ${action}` };
  }
}

// 点击元素
function clickElement(selector, index) {
  const element = getElement(selector, index);
  if (!element) {
    return { success: false, error: `元素未找到: ${selector}` };
  }
  
  element.click();
  return { success: true, message: `已点击元素: ${selector}` };
}

// 在元素中输入文本
function typeText(selector, text, index) {
  const element = getElement(selector, index);
  if (!element) {
    return { success: false, error: `元素未找到: ${selector}` };
  }
  
  element.focus();
  element.value = text;
  
  // 触发事件
  element.dispatchEvent(new Event('input', { bubbles: true }));
  element.dispatchEvent(new Event('change', { bubbles: true }));
  
  return { success: true, message: `已输入文本到: ${selector}` };
}

// 滚动页面
function scrollPage(direction, amount = 500) {
  switch (direction) {
    case 'up':
      window.scrollBy(0, -amount);
      break;
    case 'down':
      window.scrollBy(0, amount);
      break;
    case 'top':
      window.scrollTo(0, 0);
      break;
    case 'bottom':
      window.scrollTo(0, document.body.scrollHeight);
      break;
    case 'toElement':
      const element = document.querySelector(amount); // amount 此处为选择器
      if (element) {
        element.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
      break;
  }
  
  return { 
    success: true, 
    message: `已滚动: ${direction}`,
    scrollPosition: { x: window.scrollX, y: window.scrollY }
  };
}

// 获取元素的文本内容
function getElementTextBySelector(selector) {
  const element = document.querySelector(selector);
  if (!element) {
    return { success: false, error: `元素未找到: ${selector}` };
  }
  
  return { 
    success: true, 
    text: element.textContent.trim(),
    selector 
  };
}

// 批量填充表单字段
function fillFormFields(fields) {
  const results = [];
  
  for (const field of fields) {
    const { selector, value, index } = field;
    const element = getElement(selector, index);
    
    if (element) {
      element.focus();
      element.value = value;
      element.dispatchEvent(new Event('input', { bubbles: true }));
      element.dispatchEvent(new Event('change', { bubbles: true }));
      results.push({ selector, success: true });
    } else {
      results.push({ selector, success: false, error: '未找到' });
    }
  }
  
  return { success: true, fields: results };
}

// 选择下拉选项
function selectOption(selector, value, index) {
  const element = getElement(selector, index);
  if (!element) {
    return { success: false, error: `元素未找到: ${selector}` };
  }
  
  if (element.tagName.toLowerCase() !== 'select') {
    return { success: false, error: `元素不是下拉选择框: ${selector}` };
  }
  
  element.value = value;
  element.dispatchEvent(new Event('change', { bubbles: true }));
  
  return { success: true, message: `已选择选项: ${value}` };
}

// 鼠标悬停元素
function hoverElement(selector, index) {
  const element = getElement(selector, index);
  if (!element) {
    return { success: false, error: `元素未找到: ${selector}` };
  }
  
  const event = new MouseEvent('mouseover', { bubbles: true });
  element.dispatchEvent(event);
  
  return { success: true, message: `已悬停: ${selector}` };
}

// 聚焦元素
function focusElement(selector, index) {
  const element = getElement(selector, index);
  if (!element) {
    return { success: false, error: `元素未找到: ${selector}` };
  }
  
  element.focus();
  return { success: true, message: `已聚焦: ${selector}` };
}

// 清空输入框
function clearElement(selector, index) {
  const element = getElement(selector, index);
  if (!element) {
    return { success: false, error: `元素未找到: ${selector}` };
  }
  
  element.value = '';
  element.dispatchEvent(new Event('input', { bubbles: true }));
  
  return { success: true, message: `已清空: ${selector}` };
}

// 根据选择器和可选索引获取元素
function getElement(selector, index) {
  if (index !== undefined) {
    const elements = document.querySelectorAll(selector);
    return elements[index] || null;
  }
  return document.querySelector(selector);
}

// 高亮显示元素
function highlightElement(selector, index) {
  // 移除已有高亮
  document.querySelectorAll('.picoclaw-highlight').forEach(el => {
    el.classList.remove('picoclaw-highlight');
    el.style.outline = '';
  });
  
  const element = getElement(selector, index);
  if (!element) {
    return { success: false, error: `元素未找到: ${selector}` };
  }
  
  element.classList.add('picoclaw-highlight');
  element.style.outline = '3px solid #4CAF50';
  element.style.outlineOffset = '2px';
  
  // 滚动到元素可见区域
  element.scrollIntoView({ behavior: 'smooth', block: 'center' });
  
  // 3 秒后移除高亮
  setTimeout(() => {
    element.classList.remove('picoclaw-highlight');
    element.style.outline = '';
    element.style.outlineOffset = '';
  }, 3000);
  
  return { success: true, message: `已高亮: ${selector}` };
}

// 获取指定元素的详细信息
function getElementInfo(selector, index) {
  const element = getElement(selector, index);
  if (!element) {
    return { success: false, error: `元素未找到: ${selector}` };
  }
  
  const rect = element.getBoundingClientRect();
  const style = window.getComputedStyle(element);
  
  return {
    success: true,
    info: {
      tag: element.tagName.toLowerCase(),
      id: element.id || '',
      class: element.className || '',
      text: getElementText(element),
      html: element.outerHTML.substring(0, 500),
      attributes: Array.from(element.attributes).reduce((acc, attr) => {
        acc[attr.name] = attr.value;
        return acc;
      }, {}),
      position: {
        x: rect.x,
        y: rect.y,
        width: rect.width,
        height: rect.height
      },
      styles: {
        display: style.display,
        visibility: style.visibility,
        opacity: style.opacity
      },
      visible: isElementVisible(element)
    }
  };
}

// 添加高亮样式
const style = document.createElement('style');
style.textContent = `
  .picoclaw-highlight {
    transition: outline 0.2s ease;
  }
`;
document.head.appendChild(style);

console.log('PicoClaw 内容脚本已加载');

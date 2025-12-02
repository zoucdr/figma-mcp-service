/**
 * Figma Plugin Bridge
 * 
 * 这个文件提供了dashboard页面与Figma Plugin之间的通信桥梁
 * 允许dashboard直接调用Figma Plugin API进行节点预览，而不需要通过后端
 */

class FigmaPluginBridge {
  constructor() {
    this.isPluginAvailable = false;
    this.messageHandlers = new Map();
    this.requestId = 0;
    
    // 检测Figma Plugin是否可用
    this.detectPlugin();
    
    // 设置消息监听器
    this.setupMessageListener();
  }

  // 检测Figma Plugin是否可用
  detectPlugin() {
    // 检查是否在Figma环境中
    if (typeof window !== 'undefined' && window.parent !== window) {
      // 尝试发送测试消息
      this.sendTestMessage();
    } else {
      console.log('FigmaPluginBridge: 不在Figma环境中');
    }
  }

  // 发送测试消息检测插件
  sendTestMessage() {
    const testId = `test_${Date.now()}`;
    
    // 设置超时检测
    const timeout = setTimeout(() => {
      console.log('FigmaPluginBridge: Figma Plugin不可用');
      this.isPluginAvailable = false;
    }, 3000);

    // 监听测试响应
    const testHandler = (event) => {
      const msg = event.data.pluginMessage;
      if (msg && msg.type === 'pluginAvailable' && msg.testId === testId) {
        clearTimeout(timeout);
        window.removeEventListener('message', testHandler);
        this.isPluginAvailable = true;
        console.log('FigmaPluginBridge: Figma Plugin可用');
      }
    };

    window.addEventListener('message', testHandler);
    
    // 发送测试消息
    this.postMessage({
      type: 'testPlugin',
      testId: testId
    });
  }

  // 设置消息监听器
  setupMessageListener() {
    window.addEventListener('message', (event) => {
      const msg = event.data.pluginMessage;
      if (!msg) return;

      // 处理预览结果
      if (msg.type === 'previewNodeResult' || msg.type === 'previewNodesResult' || msg.type === 'previewError') {
        const requestId = msg.requestId;
        if (requestId && this.messageHandlers.has(requestId)) {
          const handler = this.messageHandlers.get(requestId);
          handler(msg);
          this.messageHandlers.delete(requestId);
        }
      }
    });
  }

  // 发送消息到插件
  postMessage(message) {
    if (window.parent && window.parent !== window) {
      window.parent.postMessage({ pluginMessage: message }, '*');
    }
  }

  // 生成请求ID
  generateRequestId() {
    return `req_${++this.requestId}_${Date.now()}`;
  }

  // 预览单个节点
  async previewNode(nodeId, scale = 1.0, format = 'PNG', ignoreNodes = []) {
    if (!this.isPluginAvailable) {
      throw new Error('Figma Plugin不可用');
    }

    return new Promise((resolve, reject) => {
      const requestId = this.generateRequestId();
      
      // 设置消息处理器
      const handler = (msg) => {
        if (msg.type === 'previewNodeResult') {
          resolve(msg.data);
        } else if (msg.type === 'previewError') {
          reject(new Error(msg.message));
        }
      };

      this.messageHandlers.set(requestId, handler);

      // 构建请求数据
      const requestData = { nodeId, scale, format };
      if (ignoreNodes && ignoreNodes.length > 0) {
        requestData.ignoreNodes = ignoreNodes;
      }

      // 发送预览请求
      this.postMessage({
        type: 'previewNode',
        requestId: requestId,
        data: requestData
      });

      // 设置超时
      setTimeout(() => {
        if (this.messageHandlers.has(requestId)) {
          this.messageHandlers.delete(requestId);
          reject(new Error('预览请求超时'));
        }
      }, 30000);
    });
  }

  // 预览多个节点
  async previewNodes(nodeIds, scale = 1.0, format = 'PNG') {
    if (!this.isPluginAvailable) {
      throw new Error('Figma Plugin不可用');
    }

    return new Promise((resolve, reject) => {
      const requestId = this.generateRequestId();
      
      // 设置消息处理器
      const handler = (msg) => {
        if (msg.type === 'previewNodesResult') {
          resolve(msg.data);
        } else if (msg.type === 'previewError') {
          reject(new Error(msg.message));
        }
      };

      this.messageHandlers.set(requestId, handler);

      // 发送预览请求
      this.postMessage({
        type: 'previewNodes',
        requestId: requestId,
        data: { nodeIds, scale, format }
      });

      // 设置超时
      setTimeout(() => {
        if (this.messageHandlers.has(requestId)) {
          this.messageHandlers.delete(requestId);
          reject(new Error('批量预览请求超时'));
        }
      }, 60000);
    });
  }

  // 检查插件是否可用
  isAvailable() {
    return this.isPluginAvailable;
  }
}

// 创建全局实例
window.figmaPluginBridge = new FigmaPluginBridge();

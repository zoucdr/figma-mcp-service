// This is the main code file for the Cursor MCP Figma plugin
// It handles Figma API commands

// Plugin state
const state = {
  serverHost: "localhost", // Default host
  serverPort: 8080, // Default port
  connectionId: "", // MCP Token for remote connections
};


// Helper function for progress updates
function sendProgressUpdate(
  commandId,
  commandType,
  status,
  progress,
  totalItems,
  processedItems,
  message,
  payload = null
) {
  const update = {
    type: "command_progress",
    commandId,
    commandType,
    status,
    progress,
    totalItems,
    processedItems,
    message,
    timestamp: Date.now(),
  };

  // Add optional chunk information if present
  if (payload) {
    if (
      payload.currentChunk !== undefined &&
      payload.totalChunks !== undefined
    ) {
      update.currentChunk = payload.currentChunk;
      update.totalChunks = payload.totalChunks;
      update.chunkSize = payload.chunkSize;
    }
    update.payload = payload;
  }

  // Send to UI
  figma.ui.postMessage(update);
  console.log(`Progress update: ${status} - ${progress}% - ${message}`);

  return update;
}

// Show UI
figma.showUI(__html__, { width: 350, height: 600 });

// Listen for selection changes
figma.on("selectionchange", () => {
  // Send current selection to UI
  const selection = figma.currentPage.selection;
  if (selection.length > 0) {
    const node = selection[0];
    figma.ui.postMessage({
      type: "current-selection",
      nodeId: node.id,
      nodeName: node.name,
      nodeType: node.type,
    });
  } else {
    figma.ui.postMessage({
      type: "current-selection",
      nodeId: null,
    });
  }
});

// Load saved settings when plugin starts
async function loadSettings() {
  try {
    const savedSettings = await figma.clientStorage.getAsync("settings");
    if (savedSettings) {
      // Update state
      if (savedSettings.serverHost) state.serverHost = savedSettings.serverHost;
      if (savedSettings.serverPort) state.serverPort = savedSettings.serverPort;
      if (savedSettings.connectionId !== undefined) state.connectionId = savedSettings.connectionId;
      
      // Send settings to UI
      figma.ui.postMessage({
        type: "settings-loaded",
        settings: savedSettings,
      });
      
      console.log("Settings loaded:", savedSettings);
    }
  } catch (error) {
    console.error("Error loading settings:", error);
  }
}

// Load settings after UI is ready
loadSettings();

// Plugin commands from UI
figma.ui.onmessage = async (msg) => {
  switch (msg.type) {
    case "get-settings":
      // Load and send settings to UI
      await loadSettings();
      break;
    case "save-settings":
      // Save settings from UI
      if (msg.settings) {
        await saveSettings(msg.settings);
      }
      break;
    case "get-call-history":
      // Load and send call history to UI
      await loadCallHistory();
      break;
    case "save-call-history":
      // Save call history from UI
      if (msg.history) {
        await saveCallHistory(msg.history);
      }
      break;
    case "update-settings":
      updateSettings(msg);
      break;
    case "resize-window":
      // Resize the plugin window
      if (msg.width && msg.height) {
        figma.ui.resize(msg.width, msg.height);
      }
      break;
    case "notify":
      figma.notify(msg.message);
      break;
    case "close-plugin":
      figma.closePlugin();
      break;
    case "execute-command":
      // Execute commands received from UI (which gets them from WebSocket)
      try {
        const result = await handleCommand(msg.command, msg.params);
        // Send result back to UI
        figma.ui.postMessage({
          type: "command-result",
          id: msg.id,
          command: msg.command,
          params: msg.params,
          result,
        });
      } catch (error) {
        figma.ui.postMessage({
          type: "command-error",
          id: msg.id,
          command: msg.command,
          params: msg.params,
          error: error.message || "Error executing command",
        });
      }
      break;
    case "get-current-selection":
      // Get current selection and send to UI
      try {
        const selection = figma.currentPage.selection;
        console.log("[get-current-selection] Selection length:", selection.length);
        
        if (selection.length > 0) {
          const node = selection[0];
          const responseData = {
            type: "current-selection",
            nodeId: node.id,
            nodeName: node.name,
            nodeType: node.type,
          };
          
          console.log("[get-current-selection] Sending node data:", responseData);
          figma.ui.postMessage(responseData);
        } else {
          console.log("[get-current-selection] No node selected");
          figma.ui.postMessage({
            type: "current-selection",
            nodeId: null,
          });
        }
      } catch (error) {
        console.error("[get-current-selection] Error:", error.message);
        figma.ui.postMessage({
          type: "current-selection",
          nodeId: null,
          error: error.message,
        });
      }
      break;
    case "download-node-tree":
      // Download node tree data
      try {
        const result = await downloadNodeTree(msg.params);
        figma.ui.postMessage({
          type: "node-tree-downloaded",
          nodeId: msg.params.nodeId,
          data: result,
        });
      } catch (error) {
        figma.ui.postMessage({
          type: "node-tree-error",
          nodeId: msg.params.nodeId,
          error: error.message || "Error downloading node tree",
        });
      }
      break;
    case "download-node-image":
      // Download node image
      try {
        const result = await downloadNodeImage(msg.params);
        figma.ui.postMessage({
          type: "node-image-downloaded",
          nodeId: msg.params.nodeId,
          imageUrl: result.imageUrl,
          format: result.format,
        });
      } catch (error) {
        figma.ui.postMessage({
          type: "node-image-error",
          nodeId: msg.params.nodeId,
          error: error.message || "Error downloading node image",
        });
      }
      break;
    case "generate-node-preview":
      // Generate node preview
      try {
        const result = await generateNodePreview(msg.nodeId, msg.ignore_text, msg.ignore_nodes);
        figma.ui.postMessage({
          type: "node-preview-generated",
          nodeId: msg.nodeId,
          imageUrl: result.imageUrl,
        });
      } catch (error) {
        figma.ui.postMessage({
          type: "node-preview-error",
          nodeId: msg.nodeId,
          error: error.message || "Error generating node preview",
        });
      }
      break;
    case "force-restore-nodes":
      // Force restore all nodes (Emergency fix)
      try {
        console.log("[force-restore-nodes] Starting emergency restore...");
        const results = nodeStateManager.forceRestoreAll();
        
        figma.notify(`✅ 已恢复 ${results.restored} 个节点${results.failed > 0 ? ` (${results.failed} 个失败)` : ''}`);
        
        figma.ui.postMessage({
          type: "force-restore-success",
          count: results.restored,
          failed: results.failed,
          total: results.total,
        });
      } catch (error) {
        console.error("[force-restore-nodes] Error:", error.message);
        figma.notify("❌ 恢复节点失败: " + error.message);
        
        figma.ui.postMessage({
          type: "force-restore-error",
          error: error.message || "Error restoring nodes",
        });
      }
      break;
  }
};

// Listen for plugin commands from menu
figma.on("run", ({ command }) => {
  figma.ui.postMessage({ type: "auto-connect" });
});

// Save settings to storage
async function saveSettings(settings) {
  try {
    // Update state
    if (settings.serverHost) state.serverHost = settings.serverHost;
    if (settings.serverPort) state.serverPort = settings.serverPort;
    if (settings.connectionId !== undefined) state.connectionId = settings.connectionId;
    
    // Save to storage
    await figma.clientStorage.setAsync("settings", {
      serverHost: state.serverHost,
      serverPort: state.serverPort,
      connectionId: state.connectionId,
    });
    
    console.log("Settings saved:", settings);
  } catch (error) {
    console.error("Error saving settings:", error);
  }
}

// Save call history to storage
async function saveCallHistory(history) {
  try {
    await figma.clientStorage.setAsync("callHistory", history);
    console.log("Call history saved:", history.length, "records");
  } catch (error) {
    console.error("Error saving call history:", error);
  }
}

// Load call history from storage
async function loadCallHistory() {
  try {
    const history = await figma.clientStorage.getAsync("callHistory");
    figma.ui.postMessage({
      type: "call-history-loaded",
      history: history || [],
    });
    console.log("Call history loaded:", history ? history.length : 0, "records");
  } catch (error) {
    console.error("Error loading call history:", error);
    figma.ui.postMessage({
      type: "call-history-loaded",
      history: [],
    });
  }
}

// Update plugin settings
function updateSettings(settings) {
  if (settings.serverHost) {
    state.serverHost = settings.serverHost;
  }
  if (settings.serverPort) {
    state.serverPort = settings.serverPort;
  }
  if (settings.connectionId !== undefined) {
    state.connectionId = settings.connectionId;
  }

  figma.clientStorage.setAsync("settings", {
    serverHost: state.serverHost,
    serverPort: state.serverPort,
    connectionId: state.connectionId,
  });
}

// Handle commands from UI
async function handleCommand(command, params) {
  switch (command) {
    case "read_my_design":
      return await readMyDesign(params ? params.nodeId : undefined);
    case "export_node_as_image":
      return await exportNodeAsImage(params);
    case "export_nodes_as_images":
      return await exportNodesAsImages(params);
    default:
      throw new Error(`Unknown command: ${command}`);
  }
}

// Command implementations

function rgbaToHex(color) {
  var r = Math.round(color.r * 255);
  var g = Math.round(color.g * 255);
  var b = Math.round(color.b * 255);
  var a = color.a !== undefined ? Math.round(color.a * 255) : 255;

  if (a === 255) {
    return (
      "#" +
      [r, g, b]
        .map((x) => {
          return x.toString(16).padStart(2, "0");
        })
        .join("")
    );
  }

  return (
    "#" +
    [r, g, b, a]
      .map((x) => {
        return x.toString(16).padStart(2, "0");
      })
      .join("")
  );
}

// 全局节点状态管理器：防止并发请求导致的状态混乱
const nodeStateManager = {
  // 存储节点的原始状态 { nodeId: { opacity: number, refCount: number } }
  originalStates: new Map(),
  
  // 保存节点的原始状态（首次修改时）
  saveOriginalState(node) {
    const nodeId = node.id;
    
    if (this.originalStates.has(nodeId)) {
      // 已经保存过，增加引用计数
      const state = this.originalStates.get(nodeId);
      state.refCount++;
      console.log(`[nodeStateManager] Node ${nodeId} refCount increased to ${state.refCount}`);
      return state.opacity;
    } else {
      // 首次保存，记录原始 opacity
      const originalOpacity = node.opacity;
      this.originalStates.set(nodeId, {
        opacity: originalOpacity,
        refCount: 1
      });
      console.log(`[nodeStateManager] Saved original opacity for ${nodeId}: ${originalOpacity}`);
      return originalOpacity;
    }
  },
  
  // 恢复节点状态（减少引用计数）
  restoreState(node) {
    const nodeId = node.id;
    
    if (!this.originalStates.has(nodeId)) {
      console.warn(`[nodeStateManager] No saved state for node ${nodeId}`);
      return false;
    }
    
    const state = this.originalStates.get(nodeId);
    state.refCount--;
    
    console.log(`[nodeStateManager] Node ${nodeId} refCount decreased to ${state.refCount}`);
    
    // 只有当引用计数为 0 时才真正恢复
    if (state.refCount <= 0) {
      node.opacity = state.opacity;
      this.originalStates.delete(nodeId);
      console.log(`[nodeStateManager] Restored opacity for ${nodeId} to ${state.opacity}`);
      return true;
    } else {
      console.log(`[nodeStateManager] Node ${nodeId} still in use, not restoring yet`);
      return false;
    }
  },
  
  // 清理（用于错误恢复）
  cleanup() {
    console.log(`[nodeStateManager] Cleaning up ${this.originalStates.size} saved states`);
    this.originalStates.clear();
  },
  
  // 强制恢复所有节点（紧急修复）
  forceRestoreAll() {
    console.log(`[nodeStateManager] Force restoring ${this.originalStates.size} nodes`);
    
    const results = {
      restored: 0,
      failed: 0,
      total: this.originalStates.size
    };
    
    for (const [nodeId, state] of this.originalStates.entries()) {
      try {
        // 尝试通过 ID 找到节点
        const node = figma.getNodeById(nodeId);
        
        if (!node) {
          console.warn(`[nodeStateManager] Node ${nodeId} not found, skipping`);
          results.failed++;
          continue;
        }
        
        // 强制恢复 opacity
        node.opacity = state.opacity;
        console.log(`[nodeStateManager] Force restored ${nodeId} to opacity ${state.opacity}`);
        results.restored++;
      } catch (error) {
        console.error(`[nodeStateManager] Failed to restore ${nodeId}: ${error.message}`);
        results.failed++;
      }
    }
    
    // 清空状态
    this.originalStates.clear();
    
    console.log(`[nodeStateManager] Force restore complete: ${results.restored} restored, ${results.failed} failed`);
    return results;
  }
};

// Helper function: 保留最多2位小数
function round2(value) {
  if (typeof value !== 'number') return value;
  return Math.round(value * 100) / 100;
}

// Helper function: 处理边界框精度（保留最多2位小数）
function roundBoundingBox(box) {
  if (!box) return box;
  
  var rounded = {};
  if (box.x !== undefined) {
    rounded.x = round2(box.x);
  }
  if (box.y !== undefined) {
    rounded.y = round2(box.y);
  }
  if (box.width !== undefined) {
    rounded.width = round2(box.width);
  }
  if (box.height !== undefined) {
    rounded.height = round2(box.height);
  }
  
  return rounded;
}

// Helper function: 处理颜色精度（保留最多2位小数）
function roundColor(color) {
  if (!color) return color;
  
  return {
    r: round2(color.r),
    g: round2(color.g),
    b: round2(color.b),
    a: round2(color.a)
  };
}

// Helper function: 处理坐标点精度（保留最多2位小数）
function roundPoint(point) {
  if (!point) return point;
  
  return {
    x: round2(point.x),
    y: round2(point.y)
  };
}

function filterFigmaNode(node) {
  // 不再过滤 VECTOR 类型，保留所有节点
  // if (node.type === "VECTOR") {
  //   return null;
  // }

  var filtered = {
    id: node.id,
    name: node.name,
    type: node.type,
  };

  // ===== 重要：保留所有边界框信息 =====
  // absoluteBoundingBox: 节点的绝对边界框（相对于画布）
  if (node.absoluteBoundingBox) {
    filtered.absoluteBoundingBox = roundBoundingBox(node.absoluteBoundingBox);
  }
  
  // absoluteRenderBounds: 节点的渲染边界（包括效果如阴影、模糊等）
  if (node.absoluteRenderBounds) {
    filtered.absoluteRenderBounds = roundBoundingBox(node.absoluteRenderBounds);
  }

  // relativeTransform: 相对变换矩阵（保留最多2位小数）
  if (node.relativeTransform) {
    filtered.relativeTransform = node.relativeTransform.map(function(row) {
      return row.map(round2);
    });
  }

  // size: 节点尺寸（保留最多2位小数）
  if (node.size) {
    filtered.size = {
      width: round2(node.size.width),
      height: round2(node.size.height)
    };
  }

  // width 和 height（保留最多2位小数）
  if (node.width !== undefined) {
    filtered.width = round2(node.width);
  }
  if (node.height !== undefined) {
    filtered.height = round2(node.height);
  }

  // x 和 y 坐标（保留最多2位小数）
  if (node.x !== undefined) {
    filtered.x = round2(node.x);
  }
  if (node.y !== undefined) {
    filtered.y = round2(node.y);
  }

  if (node.fills && node.fills.length > 0) {
    filtered.fills = node.fills.map((fill) => {
      var processedFill = Object.assign({}, fill);
      delete processedFill.boundVariables;
      delete processedFill.imageRef;

      // 处理 opacity（保留最多2位小数）
      if (processedFill.opacity !== undefined) {
        processedFill.opacity = round2(processedFill.opacity);
      }

      // 处理 gradientHandlePositions（保留最多2位小数）
      if (processedFill.gradientHandlePositions) {
        processedFill.gradientHandlePositions = processedFill.gradientHandlePositions.map(roundPoint);
      }

      // 处理 gradientStops
      if (processedFill.gradientStops) {
        processedFill.gradientStops = processedFill.gradientStops.map(
          (stop) => {
            var processedStop = Object.assign({}, stop);
            
            // 处理颜色精度
            if (processedStop.color) {
              processedStop.color = roundColor(processedStop.color);
              processedStop.color_hex = rgbaToHex(processedStop.color);
            }
            
            // 处理 position 精度
            if (processedStop.position !== undefined) {
              processedStop.position = round2(processedStop.position);
            }
            
            delete processedStop.boundVariables;
            return processedStop;
          }
        );
      }

      // 处理纯色填充的颜色
      if (processedFill.color) {
        processedFill.color = roundColor(processedFill.color);
        processedFill.color_hex = rgbaToHex(processedFill.color);
      }

      return processedFill;
    });
  }

  if (node.strokes && node.strokes.length > 0) {
    filtered.strokes = node.strokes.map((stroke) => {
      var processedStroke = Object.assign({}, stroke);
      delete processedStroke.boundVariables;
      
      // 处理颜色精度
      if (processedStroke.color) {
        processedStroke.color = roundColor(processedStroke.color);
        processedStroke.color_hex = rgbaToHex(processedStroke.color);
      }
      
      // 处理 opacity 精度
      if (processedStroke.opacity !== undefined) {
        processedStroke.opacity = round2(processedStroke.opacity);
      }
      
      return processedStroke;
    });
  }

  if (node.cornerRadius !== undefined) {
    filtered.cornerRadius = round2(node.cornerRadius);
  }

  // 保留 opacity（保留最多2位小数）
  if (node.opacity !== undefined) {
    filtered.opacity = round2(node.opacity);
  }

  // 保留 visible
  if (node.visible !== undefined) {
    filtered.visible = node.visible;
  }

  // 保留 locked
  if (node.locked !== undefined) {
    filtered.locked = node.locked;
  }

  if (node.characters) {
    filtered.characters = node.characters;
  }

  if (node.style) {
    filtered.style = {
      fontFamily: node.style.fontFamily,
      fontStyle: node.style.fontStyle,
      fontWeight: node.style.fontWeight,
      fontSize: node.style.fontSize,
      textAlignHorizontal: node.style.textAlignHorizontal,
      letterSpacing: node.style.letterSpacing,
      lineHeightPx: node.style.lineHeightPx,
    };
  }

  // 保留约束信息
  if (node.constraints) {
    filtered.constraints = node.constraints;
  }

  // 保留布局信息
  if (node.layoutMode) {
    filtered.layoutMode = node.layoutMode;
  }
  if (node.layoutAlign) {
    filtered.layoutAlign = node.layoutAlign;
  }
  if (node.layoutGrow) {
    filtered.layoutGrow = node.layoutGrow;
  }
  if (node.primaryAxisSizingMode) {
    filtered.primaryAxisSizingMode = node.primaryAxisSizingMode;
  }
  if (node.counterAxisSizingMode) {
    filtered.counterAxisSizingMode = node.counterAxisSizingMode;
  }

  // 保留子节点
  if (node.children) {
    filtered.children = node.children
      .map((child) => {
        return filterFigmaNode(child);
      })
      .filter((child) => {
        return child !== null;
      });
  }

  return filtered;
}

async function readMyDesign(nodeId) {
  try {
    let node;
    if (nodeId) {
      // 将 - 转换为 : (例如: "1-123" -> "1:123")
      const convertedNodeId = nodeId.replace(/-/g, ':');
      node = await figma.getNodeByIdAsync(convertedNodeId);
      if (!node) {
        throw new Error(`Node not found with ID: ${nodeId} (converted to ${convertedNodeId})`);
      }
    } else {
      const selection = figma.currentPage.selection;
      if (selection.length > 0) {
        node = selection[0];
      } else {
        throw new Error("No node selected. Please select a node or provide a nodeId.");
      }
    }

    const response = await node.exportAsync({
      format: "JSON_REST_V1",
    });

    return {
      nodeId: node.id,
      document: filterFigmaNode(response.document),
    };
  } catch (error) {
    throw new Error(`Error reading design: ${error.message}`);
  }
}

async function exportNodeAsImage(params) {
  const { nodeId, scale = 1, ignore_text = false, ignore_nodes = [] } = params || {};
  const format = "PNG";

  if (!nodeId) {
    throw new Error("Missing nodeId parameter");
  }

  // 将 - 转换为 : (例如: "1-123" -> "1:123")
  const convertedNodeId = nodeId.replace(/-/g, ':');
  
  const node = await figma.getNodeByIdAsync(convertedNodeId);
  if (!node) {
    throw new Error(`Node not found with ID: ${nodeId} (converted to ${convertedNodeId})`);
  }

  if (!("exportAsync" in node)) {
    throw new Error(`Node does not support exporting: ${nodeId}`);
  }

  // 如果需要忽略文本或节点，设置透明度为 0
  let modifiedNodes = [];
  const needsModifying = ignore_text || (ignore_nodes && ignore_nodes.length > 0);

  if (needsModifying) {
    const nodesToProcess = await collectNodesToHide(node, ignore_text, ignore_nodes);
    console.log(`Processing ${nodesToProcess.length} nodes (${ignore_text ? 'text' : ''}${ignore_text && ignore_nodes && ignore_nodes.length ? ' + ' : ''}${ignore_nodes && ignore_nodes.length ? ignore_nodes.length + ' specified' : ''})`);
    
    modifiedNodes = hideNodesByOpacity(nodesToProcess);
    console.log(`Set opacity to 0 for ${modifiedNodes.length} nodes`);
  }

  try {
    const settings = {
      format: format,
      constraint: { type: "SCALE", value: scale },
    };

    console.log(`[exportNodeAsImage] Starting export with ${modifiedNodes.length} modified nodes`);
    const bytes = await node.exportAsync(settings);
    console.log(`[exportNodeAsImage] Export completed successfully`);

    let mimeType;
    switch (format) {
      case "PNG":
        mimeType = "image/png";
        break;
      case "JPG":
        mimeType = "image/jpeg";
        break;
      case "SVG":
        mimeType = "image/svg+xml";
        break;
      case "PDF":
        mimeType = "application/pdf";
        break;
      default:
        mimeType = "application/octet-stream";
    }

    // Proper way to convert Uint8Array to base64
    const base64 = customBase64Encode(bytes);

    return {
      nodeId,
      format,
      scale,
      mimeType,
      imageData: base64,
    };
  } catch (error) {
    console.error(`[exportNodeAsImage] Export error: ${error.message}`);
    throw new Error(`Error exporting node as image: ${error.message}`);
  } finally {
    // 恢复修改的节点
    console.log(`[exportNodeAsImage] Finally block: restoring ${modifiedNodes.length} nodes`);
    if (modifiedNodes.length > 0) {
      try {
        const restoredCount = restoreNodeOpacity(modifiedNodes);
        console.log(`[exportNodeAsImage] Restored opacity for ${restoredCount}/${modifiedNodes.length} nodes`);
      } catch (restoreError) {
        console.error(`[exportNodeAsImage] Error during restore: ${restoreError.message}`);
      }
    }
  }
}

// Helper function: 收集需要隐藏的节点
async function collectNodesToHide(rootNode, ignore_text, ignore_nodes) {
  const nodesToHide = [];
  
  // 处理忽略文本节点
  if (ignore_text) {
    let textNodes = [];
    
    // 尝试使用 findAllWithCriteria
    if ("findAllWithCriteria" in rootNode) {
      try {
        textNodes = rootNode.findAllWithCriteria({ types: ['TEXT'] });
      } catch (error) {
        textNodes = [];
      }
    }
    
    // 如果失败，使用递归查找
    if (textNodes.length === 0 && 'children' in rootNode) {
      textNodes = findAllTextNodes(rootNode);
    }
    
    nodesToHide.push(...textNodes);
  }
  
  // 处理忽略指定节点
  if (ignore_nodes && ignore_nodes.length > 0) {
    for (const ignoreNodeId of ignore_nodes) {
      try {
        const convertedIgnoreNodeId = ignoreNodeId.replace(/-/g, ':');
        const nodeToIgnore = await figma.getNodeByIdAsync(convertedIgnoreNodeId);
        if (nodeToIgnore) {
          nodesToHide.push(nodeToIgnore);
        }
      } catch (error) {
        console.warn(`Could not find node ${ignoreNodeId}`);
      }
    }
  }
  
  return nodesToHide;
}

// Helper function: 设置节点透明度为 0
function hideNodesByOpacity(nodes) {
  const modifiedNodes = [];
  
  for (const node of nodes) {
    try {
      // 使用状态管理器保存原始状态（防止并发请求覆盖）
      const originalOpacity = nodeStateManager.saveOriginalState(node);
      
      console.log(`[hideNodesByOpacity] Setting opacity=0 for ${node.id} (${node.name}), saved original: ${originalOpacity}, current: ${node.opacity}`);
      
      // 设置透明度为 0
      node.opacity = 0;
      
      modifiedNodes.push({ 
        node, 
        originalOpacity
      });
    } catch (error) {
      console.warn(`[hideNodesByOpacity] Could not set opacity for node ${node.id}: ${error.message}`);
    }
  }
  
  console.log(`[hideNodesByOpacity] Modified ${modifiedNodes.length} nodes`);
  return modifiedNodes;
}

// Helper function: 恢复节点透明度
function restoreNodeOpacity(modifiedNodes) {
  let restoredCount = 0;
  let failedCount = 0;
  let skippedCount = 0;
  
  console.log(`[restoreNodeOpacity] Attempting to restore ${modifiedNodes.length} nodes`);
  
  for (const { node, originalOpacity } of modifiedNodes) {
    try {
      // 检查节点是否仍然有效
      if (!node || node.removed) {
        console.warn(`[restoreNodeOpacity] Node ${node ? node.id : 'unknown'} was removed, skipping`);
        failedCount++;
        continue;
      }
      
      // 使用状态管理器恢复（考虑引用计数）
      const wasRestored = nodeStateManager.restoreState(node);
      
      if (wasRestored) {
        console.log(`[restoreNodeOpacity] ✓ Fully restored ${node.id} (${node.name})`);
        restoredCount++;
      } else {
        console.log(`[restoreNodeOpacity] ⏸ Skipped ${node.id} (still in use by other requests)`);
        skippedCount++;
      }
    } catch (error) {
      console.error(`[restoreNodeOpacity] Failed to restore node ${node ? node.id : 'unknown'}: ${error.message}`);
      failedCount++;
    }
  }
  
  console.log(`[restoreNodeOpacity] Restored: ${restoredCount}, Skipped: ${skippedCount}, Failed: ${failedCount}`);
  return restoredCount;
}

// Helper function: 递归查找所有文本节点
function findAllTextNodes(node, textNodes = []) {
  if (node.type === 'TEXT') {
    textNodes.push(node);
  }
  
  if ('children' in node && node.children && node.children.length > 0) {
    for (const child of node.children) {
      findAllTextNodes(child, textNodes);
    }
  }
  
  return textNodes;
}

// Helper function: 获取节点从根到自身的路径索引
// 返回从根节点到目标节点的子节点索引路径
function getNodePath(targetNode, rootNode) {
  // 如果目标节点就是根节点，返回空路径
  if (targetNode.id === rootNode.id) {
    console.log(`[getNodePath] Target is root node, returning empty path`);
    return [];
  }
  
  const path = [];
  let currentNode = targetNode;
  const nodeChain = []; // 用于日志记录节点链
  
  // 从目标节点向上遍历到根节点
  while (currentNode && currentNode.id !== rootNode.id) {
    nodeChain.push(`${currentNode.id}(${currentNode.name})`);
    
    const parent = currentNode.parent;
    if (!parent) {
      // 已经到达文档根，但还没找到 rootNode
      // 说明 targetNode 不在 rootNode 的子树内
      console.warn(`[getNodePath] Node ${targetNode.id} is not a descendant of ${rootNode.id}`);
      console.warn(`[getNodePath] Chain: ${nodeChain.join(' -> ')}`);
      return null;
    }
    
    if (!parent.children) {
      console.warn(`[getNodePath] Parent node ${parent.id} has no children property`);
      return null;
    }
    
    // 找到当前节点在父节点children中的索引
    let index = parent.children.indexOf(currentNode);
    
    // 如果使用 indexOf 找不到（可能因为节点是实例），尝试通过 ID 匹配
    if (index === -1) {
      console.warn(`[getNodePath] indexOf failed for node ${currentNode.id}, trying ID match`);
      for (let i = 0; i < parent.children.length; i++) {
        if (parent.children[i].id === currentNode.id) {
          index = i;
          console.log(`[getNodePath] Found by ID match at index ${i}`);
          break;
        }
      }
    }
    
    if (index === -1) {
      console.warn(`[getNodePath] Could not find node ${currentNode.id} in parent ${parent.id}'s children (parent has ${parent.children.length} children)`);
      console.warn(`[getNodePath] Parent children IDs:`, parent.children.map(c => c.id).join(', '));
      return null; // 找不到路径
    }
    
    path.unshift(index); // 添加到路径开头
    currentNode = parent;
  }
  
  // 检查是否成功到达根节点
  if (currentNode && currentNode.id === rootNode.id) {
    console.log(`[getNodePath] ✓ Found path for ${targetNode.id} (${targetNode.name}): [${path.join(', ')}] (depth: ${path.length})`);
    return path;
  }
  
  // 没有找到有效路径
  console.warn(`[getNodePath] Could not find valid path from ${targetNode.id} to ${rootNode.id}`);
  return null;
}

// Helper function: 通过路径索引在节点树中找到对应节点
function getNodeByPath(rootNode, path) {
  if (!path || path.length === 0) {
    console.log(`[getNodeByPath] Empty path, returning root node`);
    return rootNode;
  }
  
  let currentNode = rootNode;
  const traversalLog = [`root: ${rootNode.id}(${rootNode.name})`];
  
  for (let i = 0; i < path.length; i++) {
    const index = path[i];
    if (!currentNode.children) {
      console.warn(`[getNodeByPath] Node ${currentNode.id} has no children at step ${i}, path: [${path.join(', ')}]`);
      return null; // 路径无效
    }
    if (index >= currentNode.children.length) {
      console.warn(`[getNodeByPath] Index ${index} out of bounds at step ${i} (node has ${currentNode.children.length} children), path: [${path.join(', ')}]`);
      return null; // 路径无效
    }
    currentNode = currentNode.children[index];
    traversalLog.push(`[${index}]: ${currentNode.id}(${currentNode.name})`);
  }
  
  console.log(`[getNodeByPath] ✓ Path [${path.join(', ')}] -> ${currentNode.id}(${currentNode.name}, type: ${currentNode.type})`);
  return currentNode;
}

function customBase64Encode(bytes) {
  const chars =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
  
  const byteLength = bytes.byteLength;
  const byteRemainder = byteLength % 3;
  const mainLength = byteLength - byteRemainder;

  // 使用数组收集字符，避免字符串拼接的性能问题
  const parts = [];
  let a, b, c, d;
  let chunk;

  // Main loop deals with bytes in chunks of 3
  for (let i = 0; i < mainLength; i = i + 3) {
    // Combine the three bytes into a single integer
    chunk = (bytes[i] << 16) | (bytes[i + 1] << 8) | bytes[i + 2];

    // Use bitmasks to extract 6-bit segments from the triplet
    a = (chunk & 16515072) >> 18; // 16515072 = (2^6 - 1) << 18
    b = (chunk & 258048) >> 12; // 258048 = (2^6 - 1) << 12
    c = (chunk & 4032) >> 6; // 4032 = (2^6 - 1) << 6
    d = chunk & 63; // 63 = 2^6 - 1

    // Convert the raw binary segments to the appropriate ASCII encoding
    parts.push(chars[a] + chars[b] + chars[c] + chars[d]);
  }

  // Deal with the remaining bytes and padding
  if (byteRemainder === 1) {
    chunk = bytes[mainLength];

    a = (chunk & 252) >> 2; // 252 = (2^6 - 1) << 2

    // Set the 4 least significant bits to zero
    b = (chunk & 3) << 4; // 3 = 2^2 - 1

    parts.push(chars[a] + chars[b] + "==");
  } else if (byteRemainder === 2) {
    chunk = (bytes[mainLength] << 8) | bytes[mainLength + 1];

    a = (chunk & 64512) >> 10; // 64512 = (2^6 - 1) << 10
    b = (chunk & 1008) >> 4; // 1008 = (2^6 - 1) << 4

    // Set the 2 least significant bits to zero
    c = (chunk & 15) << 2; // 15 = 2^4 - 1

    parts.push(chars[a] + chars[b] + chars[c] + "=");
  }

  // 一次性拼接所有字符，比逐个拼接快得多
  return parts.join('');
}

async function exportNodesAsImages(params) {
  const { nodeIds, scale = 1, ignore_text = false, ignore_nodes = [] } = params || {};

  if (!nodeIds) {
    throw new Error("Missing nodeIds parameter");
  }

  // 解析nodeIds：如果是字符串则用逗号分割，如果是数组则直接使用
  let nodeIdArray = [];
  if (typeof nodeIds === 'string') {
    nodeIdArray = nodeIds.split(',').map(id => id.trim()).filter(id => id.length > 0);
  } else if (Array.isArray(nodeIds)) {
    nodeIdArray = nodeIds;
  } else {
    throw new Error("nodeIds must be a string (comma-separated) or an array");
  }

  if (nodeIdArray.length === 0) {
    throw new Error("nodeIds is empty");
  }

  const results = {};
  const errors = {};

  // 批量处理所有节点
  for (let i = 0; i < nodeIdArray.length; i++) {
    const nodeId = nodeIdArray[i];
    try {
      // 将 - 转换为 : (例如: "1-123" -> "1:123")
      const convertedNodeId = nodeId.replace(/-/g, ':');
      
      const node = await figma.getNodeByIdAsync(convertedNodeId);
      if (!node) {
        errors[nodeId] = `Node not found with ID: ${nodeId}`;
        continue;
      }

      if (!("exportAsync" in node)) {
        errors[nodeId] = `Node does not support exporting: ${nodeId}`;
        continue;
      }

      // 如果需要忽略文本或节点，使用克隆方式避免布局问题
      const needsClone = ignore_text || (ignore_nodes && ignore_nodes.length > 0);
      let exportNode = node;
      let clonedNode = null;

      if (needsClone) {
        try {
          // 第一步：在原始节点中收集要删除的节点路径
          const nodePaths = [];

          // 处理忽略文本节点
          if (ignore_text && "findAllWithCriteria" in node) {
            const textNodes = node.findAllWithCriteria({ types: ['TEXT'] });
            for (const textNode of textNodes) {
              const path = getNodePath(textNode, node);
              if (path) {
                nodePaths.push(path);
              }
            }
          }

          // 处理忽略指定节点
          if (ignore_nodes && ignore_nodes.length > 0) {
            for (const ignoreNodeId of ignore_nodes) {
              try {
                const convertedIgnoreNodeId = ignoreNodeId.replace(/-/g, ':');
                const nodeToIgnore = await figma.getNodeByIdAsync(convertedIgnoreNodeId);
                
                if (nodeToIgnore) {
                  const path = getNodePath(nodeToIgnore, node);
                  if (path) {
                    nodePaths.push(path);
                  }
                }
              } catch (error) {
                console.warn(`Could not find node to ignore with ID: ${ignoreNodeId}`, error);
              }
            }
          }

          // 第二步：克隆节点
          clonedNode = node.clone();
          exportNode = clonedNode;

          // 第三步：在克隆节点中通过路径找到并删除节点（包括整个子树）
          // 按路径深度倒序排序，先删除深层节点，避免父节点被删后子节点路径失效
          nodePaths.sort((a, b) => b.length - a.length);
          
          for (const path of nodePaths) {
            try {
              const nodeToRemove = getNodeByPath(clonedNode, path);
              if (nodeToRemove) {
                nodeToRemove.remove(); // 删除节点及其整个子树
              }
            } catch (error) {
              console.warn(`Could not remove node at path [${path.join(',')}]: ${error.message}`);
            }
          }
        } catch (error) {
          console.warn(`Could not clone node, falling back to original: ${error.message}`);
          exportNode = node;
          clonedNode = null;
        }
      }

      try {
        const settings = {
          format: "PNG",
          constraint: { type: "SCALE", value: scale },
        };

        const bytes = await exportNode.exportAsync(settings);
        const base64 = customBase64Encode(bytes);
        
        // 使用原始nodeId作为key
        results[nodeId] = base64;

      } finally {
        // 清理克隆的节点
        if (clonedNode) {
          try {
            clonedNode.remove();
          } catch (error) {
            console.warn(`Could not remove cloned node: ${error.message}`);
          }
        }
      }

    } catch (error) {
      errors[nodeId] = error.message || "Error exporting node";
    }
  }

  return {
    images: results,
    errors: Object.keys(errors).length > 0 ? errors : undefined,
    total: nodeIdArray.length,
    success: Object.keys(results).length,
    failed: Object.keys(errors).length,
  };
}

// Download node tree data
async function downloadNodeTree(params) {
  const { nodeId, includeChildren = true, includeStyles = true, includeFills = true } = params || {};
  
  try {
    let node;
    if (nodeId) {
      // Convert - to : (e.g., "1-123" -> "1:123")
      const convertedNodeId = nodeId.replace(/-/g, ':');
      node = await figma.getNodeByIdAsync(convertedNodeId);
      if (!node) {
        throw new Error(`Node not found with ID: ${nodeId} (converted to ${convertedNodeId})`);
      }
    } else {
      const selection = figma.currentPage.selection;
      if (selection.length > 0) {
        node = selection[0];
      } else {
        throw new Error("No node selected. Please select a node or provide a nodeId.");
      }
    }

    // Create a detailed node tree structure
    const nodeTree = await buildNodeTree(node, includeChildren, includeStyles, includeFills);
    
    return {
      nodeId: node.id,
      nodeName: node.name,
      nodeType: node.type,
      timestamp: new Date().toISOString(),
      options: {
        includeChildren,
        includeStyles,
        includeFills
      },
      tree: nodeTree
    };
  } catch (error) {
    throw new Error(`Error downloading node tree: ${error.message}`);
  }
}

// Build detailed node tree
async function buildNodeTree(node, includeChildren, includeStyles, includeFills) {
  const nodeData = {
    id: node.id,
    name: node.name,
    type: node.type,
    visible: node.visible,
    locked: node.locked,
  };

  // Add position and size information（保留最多2位小数）
  if (node.x !== undefined) nodeData.x = round2(node.x);
  if (node.y !== undefined) nodeData.y = round2(node.y);
  if (node.width !== undefined) nodeData.width = round2(node.width);
  if (node.height !== undefined) nodeData.height = round2(node.height);
  if (node.absoluteBoundingBox) nodeData.absoluteBoundingBox = roundBoundingBox(node.absoluteBoundingBox);
  if (node.absoluteRenderBounds) nodeData.absoluteRenderBounds = roundBoundingBox(node.absoluteRenderBounds);
  if (node.relativeTransform) {
    nodeData.relativeTransform = node.relativeTransform.map(function(row) {
      return row.map(round2);
    });
  }

  // Add opacity and blend mode
  if (node.opacity !== undefined) nodeData.opacity = round2(node.opacity);
  if (node.blendMode) nodeData.blendMode = node.blendMode;

  // Add constraints and layout information
  if (node.constraints) nodeData.constraints = node.constraints;
  if (node.layoutMode) nodeData.layoutMode = node.layoutMode;
  if (node.layoutAlign) nodeData.layoutAlign = node.layoutAlign;
  if (node.layoutGrow) nodeData.layoutGrow = node.layoutGrow;
  if (node.primaryAxisSizingMode) nodeData.primaryAxisSizingMode = node.primaryAxisSizingMode;
  if (node.counterAxisSizingMode) nodeData.counterAxisSizingMode = node.counterAxisSizingMode;
  if (node.paddingLeft !== undefined) nodeData.paddingLeft = round2(node.paddingLeft);
  if (node.paddingRight !== undefined) nodeData.paddingRight = round2(node.paddingRight);
  if (node.paddingTop !== undefined) nodeData.paddingTop = round2(node.paddingTop);
  if (node.paddingBottom !== undefined) nodeData.paddingBottom = round2(node.paddingBottom);
  if (node.itemSpacing !== undefined) nodeData.itemSpacing = round2(node.itemSpacing);

  // Add fills if requested
  if (includeFills && node.fills && node.fills.length > 0) {
    nodeData.fills = node.fills.map((fill) => {
      const processedFill = Object.assign({}, fill);
      delete processedFill.boundVariables;
      delete processedFill.imageRef;
      
      // 处理 opacity
      if (processedFill.opacity !== undefined) {
        processedFill.opacity = round2(processedFill.opacity);
      }
      
      // 处理 gradientHandlePositions
      if (processedFill.gradientHandlePositions) {
        processedFill.gradientHandlePositions = processedFill.gradientHandlePositions.map(roundPoint);
      }
      
      // 处理 gradientStops
      if (processedFill.gradientStops) {
        processedFill.gradientStops = processedFill.gradientStops.map((stop) => {
          const processedStop = Object.assign({}, stop);
          
          if (processedStop.color) {
            processedStop.color = roundColor(processedStop.color);
            processedStop.color_hex = rgbaToHex(processedStop.color);
          }
          
          if (processedStop.position !== undefined) {
            processedStop.position = round2(processedStop.position);
          }
          
          delete processedStop.boundVariables;
          return processedStop;
        });
      }
      
      // 处理纯色填充
      if (processedFill.color) {
        processedFill.color = roundColor(processedFill.color);
        processedFill.color_hex = rgbaToHex(processedFill.color);
      }
      
      return processedFill;
    });
  }

  // Add strokes
  if (node.strokes && node.strokes.length > 0) {
    nodeData.strokes = node.strokes.map((stroke) => {
      const processedStroke = Object.assign({}, stroke);
      delete processedStroke.boundVariables;
      
      if (processedStroke.color) {
        processedStroke.color = roundColor(processedStroke.color);
        processedStroke.color_hex = rgbaToHex(processedStroke.color);
      }
      
      if (processedStroke.opacity !== undefined) {
        processedStroke.opacity = round2(processedStroke.opacity);
      }
      
      return processedStroke;
    });
  }

  // Add corner radius（保留最多2位小数）
  if (node.cornerRadius !== undefined) nodeData.cornerRadius = round2(node.cornerRadius);
  if (node.topLeftRadius !== undefined) nodeData.topLeftRadius = round2(node.topLeftRadius);
  if (node.topRightRadius !== undefined) nodeData.topRightRadius = round2(node.topRightRadius);
  if (node.bottomLeftRadius !== undefined) nodeData.bottomLeftRadius = round2(node.bottomLeftRadius);
  if (node.bottomRightRadius !== undefined) nodeData.bottomRightRadius = round2(node.bottomRightRadius);

  // Add text-specific properties
  if (node.type === 'TEXT') {
    if (node.characters) nodeData.characters = node.characters;
    if (node.fontSize) {
      nodeData.fontSize = round2(node.fontSize);
    }
    if (node.fontName) nodeData.fontName = node.fontName;
    if (node.textAlignHorizontal) nodeData.textAlignHorizontal = node.textAlignHorizontal;
    if (node.textAlignVertical) nodeData.textAlignVertical = node.textAlignVertical;
    if (node.letterSpacing) {
      nodeData.letterSpacing = typeof node.letterSpacing === 'object' && node.letterSpacing.value !== undefined 
        ? { unit: node.letterSpacing.unit, value: round2(node.letterSpacing.value) }
        : round2(node.letterSpacing);
    }
    if (node.lineHeight) {
      nodeData.lineHeight = typeof node.lineHeight === 'object' && node.lineHeight.value !== undefined
        ? { unit: node.lineHeight.unit, value: round2(node.lineHeight.value) }
        : round2(node.lineHeight);
    }
  }

  // Add style information if requested
  if (includeStyles && node.style) {
    nodeData.style = {
      fontFamily: node.style.fontFamily,
      fontStyle: node.style.fontStyle,
      fontWeight: node.style.fontWeight,
      fontSize: round2(node.style.fontSize),
      textAlignHorizontal: node.style.textAlignHorizontal,
      letterSpacing: round2(node.style.letterSpacing),
      lineHeightPx: round2(node.style.lineHeightPx),
    };
  }

  // Add effects
  if (node.effects && node.effects.length > 0) {
    nodeData.effects = node.effects.map((effect) => {
      const processedEffect = Object.assign({}, effect);
      
      // 处理颜色精度
      if (processedEffect.color) {
        processedEffect.color = roundColor(processedEffect.color);
        processedEffect.color_hex = rgbaToHex(processedEffect.color);
      }
      
      // 处理阴影/模糊等数值精度
      if (processedEffect.radius !== undefined) {
        processedEffect.radius = round2(processedEffect.radius);
      }
      if (processedEffect.offset) {
        processedEffect.offset = roundPoint(processedEffect.offset);
      }
      if (processedEffect.spread !== undefined) {
        processedEffect.spread = round2(processedEffect.spread);
      }
      
      return processedEffect;
    });
  }

  // Add children if requested
  if (includeChildren && node.children && node.children.length > 0) {
    nodeData.children = [];
    for (const child of node.children) {
      const childData = await buildNodeTree(child, includeChildren, includeStyles, includeFills);
      nodeData.children.push(childData);
    }
  }

  return nodeData;
}

// Generate node preview (small thumbnail)
async function generateNodePreview(nodeId, ignore_text = false, ignore_nodes = []) {
  try {
    let node;
    if (nodeId) {
      // Convert - to : (e.g., "1-123" -> "1:123")
      const convertedNodeId = nodeId.replace(/-/g, ':');
      node = await figma.getNodeByIdAsync(convertedNodeId);
      if (!node) {
        throw new Error(`Node not found with ID: ${nodeId} (converted to ${convertedNodeId})`);
      }
    } else {
      const selection = figma.currentPage.selection;
      if (selection.length > 0) {
        node = selection[0];
      } else {
        throw new Error("No node selected. Please select a node or provide a nodeId.");
      }
    }

    if (!("exportAsync" in node)) {
      throw new Error(`Node does not support exporting: ${nodeId}`);
    }

    // 检查是否需要修改节点（忽略文本或节点）
    const needsModifying = ignore_text || (ignore_nodes && ignore_nodes.length > 0);

    // 如果不需要修改，直接导出原图（快速路径）
    if (!needsModifying) {
      console.log(`[generateNodePreview] Fast path: direct export without modifications`);
      
      const settings = {
        format: "PNG",
        constraint: { type: "SCALE", value: 1 },
      };

      const bytes = await node.exportAsync(settings);
      const base64 = customBase64Encode(bytes);
      const imageUrl = `data:image/png;base64,${base64}`;

      return {
        nodeId: node.id,
        nodeName: node.name,
        imageUrl: imageUrl,
        timestamp: new Date().toISOString(),
      };
    }

    // 需要修改节点：设置透明度为 0
    console.log(`[generateNodePreview] Slow path: modifying nodes (ignore_text=${ignore_text}, ignore_nodes=${ignore_nodes.length})`);
    let modifiedNodes = [];

    const nodesToProcess = await collectNodesToHide(node, ignore_text, ignore_nodes);
    modifiedNodes = hideNodesByOpacity(nodesToProcess);

    try {
      // Generate small preview image (PNG, 1x scale)
      const settings = {
        format: "PNG",
        constraint: { type: "SCALE", value: 1 },
      };

      console.log(`[generateNodePreview] Starting preview with ${modifiedNodes.length} modified nodes`);
      const bytes = await node.exportAsync(settings);
      console.log(`[generateNodePreview] Preview export completed`);
      
      const base64 = customBase64Encode(bytes);
      const imageUrl = `data:image/png;base64,${base64}`;

      return {
        nodeId: node.id,
        nodeName: node.name,
        imageUrl: imageUrl,
        timestamp: new Date().toISOString(),
      };
    } catch (error) {
      console.error(`[generateNodePreview] Preview error: ${error.message}`);
      throw new Error(`Error generating node preview: ${error.message}`);
    } finally {
      // 恢复修改的节点
      console.log(`[generateNodePreview] Finally block: restoring ${modifiedNodes.length} nodes`);
      if (modifiedNodes.length > 0) {
        try {
          const restoredCount = restoreNodeOpacity(modifiedNodes);
          console.log(`[generateNodePreview] Restored ${restoredCount}/${modifiedNodes.length} nodes`);
        } catch (restoreError) {
          console.error(`[generateNodePreview] Error during restore: ${restoreError.message}`);
        }
      }
    }
  } catch (error) {
    throw new Error(`Error generating node preview: ${error.message}`);
  }
}

// Download node as image
async function downloadNodeImage(params) {
  const { 
    nodeId, 
    format = 'PNG', 
    scale = 2, 
    ignore_text = false,
    ignore_nodes = []
  } = params || {};
  
  try {
    let node;
    if (nodeId) {
      // Convert - to : (e.g., "1-123" -> "1:123")
      const convertedNodeId = nodeId.replace(/-/g, ':');
      node = await figma.getNodeByIdAsync(convertedNodeId);
      if (!node) {
        throw new Error(`Node not found with ID: ${nodeId} (converted to ${convertedNodeId})`);
      }
    } else {
      const selection = figma.currentPage.selection;
      if (selection.length > 0) {
        node = selection[0];
      } else {
        throw new Error("No node selected. Please select a node or provide a nodeId.");
      }
    }

    if (!("exportAsync" in node)) {
      throw new Error(`Node does not support exporting: ${nodeId}`);
    }

    // 检查是否需要修改节点（忽略文本或节点）
    const needsModifying = ignore_text || (ignore_nodes && ignore_nodes.length > 0);

    // 如果不需要修改，直接导出原图（快速路径）
    if (!needsModifying) {
      console.log(`[downloadNodeImage] Fast path: direct export without modifications`);
      
      const settings = {
        format: format,
        constraint: { type: "SCALE", value: scale },
      };

      const bytes = await node.exportAsync(settings);
      const base64 = customBase64Encode(bytes);
      
      // Determine MIME type
      let mimeType;
      switch (format) {
        case "PNG":
          mimeType = "image/png";
          break;
        case "JPG":
          mimeType = "image/jpeg";
          break;
        case "SVG":
          mimeType = "image/svg+xml";
          break;
        case "PDF":
          mimeType = "application/pdf";
          break;
        default:
          mimeType = "application/octet-stream";
      }

      const imageUrl = `data:${mimeType};base64,${base64}`;

      return {
        nodeId: node.id,
        nodeName: node.name,
        format: format,
        scale: scale,
        mimeType: mimeType,
        imageUrl: imageUrl,
        timestamp: new Date().toISOString(),
      };
    }

    // 需要修改节点：设置透明度为 0
    console.log(`[downloadNodeImage] Slow path: modifying nodes (ignore_text=${ignore_text}, ignore_nodes=${ignore_nodes.length})`);
    let modifiedNodes = [];

    const nodesToProcess = await collectNodesToHide(node, ignore_text, ignore_nodes);
    modifiedNodes = hideNodesByOpacity(nodesToProcess);

    try {
      // Prepare export settings
      const settings = {
        format: format,
        constraint: { type: "SCALE", value: scale },
      };

      console.log(`[downloadNodeImage] Starting export with ${modifiedNodes.length} modified nodes`);
      const bytes = await node.exportAsync(settings);
      console.log(`[downloadNodeImage] Export completed`);

      // Convert to base64
      const base64 = customBase64Encode(bytes);
      
      // Determine MIME type
      let mimeType;
      switch (format) {
        case "PNG":
          mimeType = "image/png";
          break;
        case "JPG":
          mimeType = "image/jpeg";
          break;
        case "SVG":
          mimeType = "image/svg+xml";
          break;
        case "PDF":
          mimeType = "application/pdf";
          break;
        default:
          mimeType = "application/octet-stream";
      }

      // Create data URL
      const imageUrl = `data:${mimeType};base64,${base64}`;

      return {
        nodeId: node.id,
        nodeName: node.name,
        format: format,
        scale: scale,
        mimeType: mimeType,
        imageUrl: imageUrl,
        timestamp: new Date().toISOString(),
      };
    } catch (error) {
      console.error(`[downloadNodeImage] Download error: ${error.message}`);
      throw new Error(`Error downloading node image: ${error.message}`);
    } finally {
      // 恢复修改的节点
      console.log(`[downloadNodeImage] Finally block: restoring ${modifiedNodes.length} nodes`);
      if (modifiedNodes.length > 0) {
        try {
          const restoredCount = restoreNodeOpacity(modifiedNodes);
          console.log(`[downloadNodeImage] Restored ${restoredCount}/${modifiedNodes.length} nodes`);
        } catch (restoreError) {
          console.error(`[downloadNodeImage] Error during restore: ${restoreError.message}`);
        }
      }
    }
  } catch (error) {
    throw new Error(`Error downloading node image: ${error.message}`);
  }
}

// Initialize settings on load
(async function initializePlugin() {
  try {
    const savedSettings = await figma.clientStorage.getAsync("settings");
    if (savedSettings) {
      if (savedSettings.serverPort) {
        state.serverPort = savedSettings.serverPort;
      }
    }

    // Send initial settings to UI
    figma.ui.postMessage({
      type: "init-settings",
      settings: {
        serverPort: state.serverPort,
      },
    });
  } catch (error) {
    console.error("Error loading settings:", error);
  }
})();

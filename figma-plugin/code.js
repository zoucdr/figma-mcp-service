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
    filtered.absoluteBoundingBox = node.absoluteBoundingBox;
  }
  
  // absoluteRenderBounds: 节点的渲染边界（包括效果如阴影、模糊等）
  if (node.absoluteRenderBounds) {
    filtered.absoluteRenderBounds = node.absoluteRenderBounds;
  }

  // relativeTransform: 相对变换矩阵
  if (node.relativeTransform) {
    filtered.relativeTransform = node.relativeTransform;
  }

  // size: 节点尺寸
  if (node.size) {
    filtered.size = node.size;
  }

  // width 和 height
  if (node.width !== undefined) {
    filtered.width = node.width;
  }
  if (node.height !== undefined) {
    filtered.height = node.height;
  }

  // x 和 y 坐标
  if (node.x !== undefined) {
    filtered.x = node.x;
  }
  if (node.y !== undefined) {
    filtered.y = node.y;
  }

  if (node.fills && node.fills.length > 0) {
    filtered.fills = node.fills.map((fill) => {
      var processedFill = Object.assign({}, fill);
      delete processedFill.boundVariables;
      delete processedFill.imageRef;

      if (processedFill.gradientStops) {
        processedFill.gradientStops = processedFill.gradientStops.map(
          (stop) => {
            var processedStop = Object.assign({}, stop);
            if (processedStop.color) {
              // 保留原始 color 对象，增加 color_hex
              processedStop.color_hex = rgbaToHex(processedStop.color);
            }
            delete processedStop.boundVariables;
            return processedStop;
          }
        );
      }

      if (processedFill.color) {
        // 保留原始 color 对象，增加 color_hex
        processedFill.color_hex = rgbaToHex(processedFill.color);
      }

      return processedFill;
    });
  }

  if (node.strokes && node.strokes.length > 0) {
    filtered.strokes = node.strokes.map((stroke) => {
      var processedStroke = Object.assign({}, stroke);
      delete processedStroke.boundVariables;
      if (processedStroke.color) {
        // 保留原始 color 对象，增加 color_hex
        processedStroke.color_hex = rgbaToHex(processedStroke.color);
      }
      return processedStroke;
    });
  }

  if (node.cornerRadius !== undefined) {
    filtered.cornerRadius = node.cornerRadius;
  }

  // 保留 opacity
  if (node.opacity !== undefined) {
    filtered.opacity = node.opacity;
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
  const { nodeId, scale = 1, ignore_text = false } = params || {};

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

  // Handle ignore_text by temporarily hiding text nodes
  const hiddenTextNodes = [];
  if (ignore_text) {
    // Find all visible text nodes in children
    if ("findAllWithCriteria" in node) {
      const textNodes = node.findAllWithCriteria({ types: ['TEXT'] });
      for (const textNode of textNodes) {
        if (textNode.visible) {
          textNode.visible = false;
          hiddenTextNodes.push(textNode);
        }
      }
    }
  }

  try {
    const settings = {
      format: format,
      constraint: { type: "SCALE", value: scale },
    };

    const bytes = await node.exportAsync(settings);

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
    // const imageData = `data:${mimeType};base64,${base64}`;

    return {
      nodeId,
      format,
      scale,
      mimeType,
      imageData: base64,
    };
  } catch (error) {
    throw new Error(`Error exporting node as image: ${error.message}`);
  } finally {
    // Restore visibility of text nodes
    if (hiddenTextNodes.length > 0) {
      for (const textNode of hiddenTextNodes) {
        try {
          textNode.visible = true;
        } catch (e) {
          console.error("Error restoring text node visibility", e);
        }
      }
    }
  }
}

function customBase64Encode(bytes) {
  const chars =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
  let base64 = "";

  const byteLength = bytes.byteLength;
  const byteRemainder = byteLength % 3;
  const mainLength = byteLength - byteRemainder;

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
    base64 += chars[a] + chars[b] + chars[c] + chars[d];
  }

  // Deal with the remaining bytes and padding
  if (byteRemainder === 1) {
    chunk = bytes[mainLength];

    a = (chunk & 252) >> 2; // 252 = (2^6 - 1) << 2

    // Set the 4 least significant bits to zero
    b = (chunk & 3) << 4; // 3 = 2^2 - 1

    base64 += chars[a] + chars[b] + "==";
  } else if (byteRemainder === 2) {
    chunk = (bytes[mainLength] << 8) | bytes[mainLength + 1];

    a = (chunk & 64512) >> 10; // 64512 = (2^6 - 1) << 10
    b = (chunk & 1008) >> 4; // 1008 = (2^6 - 1) << 4

    // Set the 2 least significant bits to zero
    c = (chunk & 15) << 2; // 15 = 2^4 - 1

    base64 += chars[a] + chars[b] + chars[c] + "=";
  }

  return base64;
}

async function exportNodesAsImages(params) {
  const { nodeIds, scale = 1, ignore_text = false } = params || {};

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

      // Handle ignore_text by temporarily hiding text nodes
      const hiddenTextNodes = [];
      if (ignore_text) {
        // Find all visible text nodes in children
        if ("findAllWithCriteria" in node) {
          const textNodes = node.findAllWithCriteria({ types: ['TEXT'] });
          for (const textNode of textNodes) {
            if (textNode.visible) {
              textNode.visible = false;
              hiddenTextNodes.push(textNode);
            }
          }
        }
      }

      try {
        const settings = {
          format: "PNG",
          constraint: { type: "SCALE", value: scale },
        };

        const bytes = await node.exportAsync(settings);
        const base64 = customBase64Encode(bytes);
        
        // 使用原始nodeId作为key
        results[nodeId] = base64;

      } finally {
        // Restore visibility of text nodes
        if (hiddenTextNodes.length > 0) {
          for (const textNode of hiddenTextNodes) {
            try {
              textNode.visible = true;
            } catch (e) {
              console.error("Error restoring text node visibility", e);
            }
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

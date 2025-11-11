// This plugin connects Figma designs to the Figma Deliver service

// Show UI when the plugin is launched
figma.showUI(__html__, { width: 400, height: 500 });

// Handle messages from the UI
figma.ui.onmessage = async (msg) => {
  switch (msg.type) {
    case 'export':
      await handleExport(msg.data);
      break;
    case 'authenticate':
      await handleAuthentication(msg.data);
      break;
    case 'getSelection':
      sendSelection();
      break;
    case 'previewNode':
      await handlePreviewNode(msg.data, msg.requestId);
      break;
    case 'previewNodes':
      await handlePreviewNodes(msg.data, msg.requestId);
      break;
    case 'testPlugin':
      handleTestPlugin(msg.testId);
      break;
    case 'closePlugin':
      figma.closePlugin();
      break;
    default:
      console.error('Unknown message type:', msg.type);
  }
};

// Handle exporting designs to the Figma Deliver service
async function handleExport(data) {
  try {
    // Check if user is authenticated
    const token = await figma.clientStorage.getAsync('figmaDeliverToken');
    const serverUrl = await figma.clientStorage.getAsync('figmaDeliverServerUrl');
    
    if (!token) {
      figma.ui.postMessage({ type: 'error', message: 'Please authenticate first' });
      return;
    }

    if (!serverUrl) {
      figma.ui.postMessage({ type: 'error', message: 'Server URL not configured' });
      return;
    }

    // Get current selection
    const selection = figma.currentPage.selection;
    if (selection.length === 0) {
      figma.ui.postMessage({ type: 'error', message: 'Please select at least one node' });
      return;
    }

    // Get the selected node ID
    const nodeId = selection[0].id;
    const fileKey = figma.fileKey;
    
    // Prepare the data to send to the server
    const exportData = {
      fileKey,
      nodeId,
      projectName: data.projectName || `Figma Project - ${fileKey}`,
      token,
      serverUrl
    };

    // Send message to UI to make API call to server
    figma.ui.postMessage({ 
      type: 'exportToServer', 
      data: exportData 
    });
  } catch (error) {
    figma.ui.postMessage({ 
      type: 'error', 
      message: `Export failed: ${error.message}` 
    });
  }
}

// Handle authentication with Figma Deliver service
async function handleAuthentication(data) {
  try {
    // Store the authentication token and server URL
    await figma.clientStorage.setAsync('figmaDeliverToken', data.token);
    await figma.clientStorage.setAsync('figmaDeliverServerUrl', data.serverUrl);
    figma.ui.postMessage({ 
      type: 'authSuccess', 
      message: 'Authentication successful' 
    });
  } catch (error) {
    figma.ui.postMessage({ 
      type: 'error', 
      message: `Authentication failed: ${error.message}` 
    });
  }
}

// Send current selection to UI
function sendSelection() {
  const selection = figma.currentPage.selection;
  if (selection.length > 0) {
    const selectedNodes = selection.map(node => ({
      id: node.id,
      name: node.name,
      type: node.type
    }));
    
    figma.ui.postMessage({ 
      type: 'selectionUpdate', 
      data: selectedNodes 
    });
  } else {
    figma.ui.postMessage({ 
      type: 'selectionUpdate', 
      data: [] 
    });
  }
}

// Initialize the plugin
async function initializePlugin() {
  // Check if user is already authenticated
  const token = await figma.clientStorage.getAsync('figmaDeliverToken');
  const serverUrl = await figma.clientStorage.getAsync('figmaDeliverServerUrl');
  
  figma.ui.postMessage({ 
    type: 'initPlugin', 
    data: { 
      isAuthenticated: !!token,
      serverUrl: serverUrl || '',
      fileKey: figma.fileKey,
      fileName: figma.root.name
    } 
  });
  
  // Send initial selection
  sendSelection();
}

// Handle test plugin request
function handleTestPlugin(testId) {
  figma.ui.postMessage({ 
    type: 'pluginAvailable',
    testId: testId
  });
}

// Handle single node preview request
async function handlePreviewNode(data, requestId) {
  try {
    const { nodeId, scale = 1.0, format = 'PNG' } = data;
    
    // Find the node by ID
    const node = figma.getNodeById(nodeId);
    if (!node) {
      figma.ui.postMessage({ 
        type: 'previewError', 
        requestId: requestId,
        message: `Node with ID ${nodeId} not found` 
      });
      return;
    }

    // Export the node as image
    const imageBytes = await node.exportAsync({
      format: format.toUpperCase(),
      constraint: { type: 'SCALE', value: scale }
    });

    // Convert to base64
    const base64 = figma.base64Encode(imageBytes);
    const dataUrl = `data:image/${format.toLowerCase()};base64,${base64}`;

    figma.ui.postMessage({ 
      type: 'previewNodeResult', 
      requestId: requestId,
      data: { 
        nodeId, 
        imageUrl: dataUrl,
        nodeName: node.name,
        nodeType: node.type
      } 
    });
  } catch (error) {
    figma.ui.postMessage({ 
      type: 'previewError', 
      requestId: requestId,
      message: `Preview failed: ${error.message}` 
    });
  }
}

// Handle multiple nodes preview request
async function handlePreviewNodes(data, requestId) {
  try {
    const { nodeIds, scale = 1.0, format = 'PNG' } = data;
    
    if (!nodeIds || !Array.isArray(nodeIds)) {
      figma.ui.postMessage({ 
        type: 'previewError', 
        requestId: requestId,
        message: 'Invalid nodeIds provided' 
      });
      return;
    }

    const results = {};
    
    for (const nodeId of nodeIds) {
      try {
        // Find the node by ID
        const node = figma.getNodeById(nodeId);
        if (!node) {
          console.warn(`Node with ID ${nodeId} not found`);
          continue;
        }

        // Export the node as image
        const imageBytes = await node.exportAsync({
          format: format.toUpperCase(),
          constraint: { type: 'SCALE', value: scale }
        });

        // Convert to base64
        const base64 = figma.base64Encode(imageBytes);
        const dataUrl = `data:image/${format.toLowerCase()};base64,${base64}`;

        results[nodeId] = {
          imageUrl: dataUrl,
          nodeName: node.name,
          nodeType: node.type
        };
      } catch (nodeError) {
        console.error(`Failed to export node ${nodeId}:`, nodeError);
        // Continue with other nodes even if one fails
      }
    }

    figma.ui.postMessage({ 
      type: 'previewNodesResult', 
      requestId: requestId,
      data: { 
        images: results,
        count: Object.keys(results).length
      } 
    });
  } catch (error) {
    figma.ui.postMessage({ 
      type: 'previewError', 
      requestId: requestId,
      message: `Batch preview failed: ${error.message}` 
    });
  }
}

// Start the plugin
initializePlugin();

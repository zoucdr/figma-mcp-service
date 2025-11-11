// Service layer for communication between the Figma plugin and the backend service

class FigmaDeliverService {
  constructor() {
    this.serverUrl = '';
    this.token = '';
  }

  // Initialize the service with server URL and token
  initialize(serverUrl, token) {
    this.serverUrl = serverUrl;
    this.token = token;
  }

  // Set the server URL
  setServerUrl(url) {
    this.serverUrl = url;
  }

  // Set the authentication token
  setToken(token) {
    this.token = token;
  }

  // Check if the service is initialized
  isInitialized() {
    return !!this.serverUrl && !!this.token;
  }

  // Authenticate with the server
  async authenticate(username, password) {
    try {
      const response = await fetch(`${this.serverUrl}/plugin/login`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: `username=${encodeURIComponent(username)}&password=${encodeURIComponent(password)}`
      });
      
      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Authentication failed');
      }
      
      const data = await response.json();
      
      if (data.token) {
        this.token = data.token;
        return { success: true, token: data.token };
      } else {
        throw new Error('No token received from server');
      }
    } catch (error) {
      console.error('Authentication error:', error);
      throw error;
    }
  }

  // Parse a Figma link
  async parseFigmaLink(link) {
    try {
      if (!this.isInitialized()) {
        throw new Error('Service not initialized');
      }
      
      const response = await fetch(`${this.serverUrl}/figma/parse`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded',
          'Authorization': `Bearer ${this.token}`
        },
        body: `link=${encodeURIComponent(link)}`
      });
      
      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to parse Figma link');
      }
      
      return await response.json();
    } catch (error) {
      console.error('Parse Figma link error:', error);
      throw error;
    }
  }

  // Create a new project from Figma file and node
  async createProject(fileKey, nodeId, name) {
    try {
      if (!this.isInitialized()) {
        throw new Error('Service not initialized');
      }
      
      // Construct the Figma URL for the file and node
      const figmaUrl = `https://www.figma.com/file/${fileKey}?node-id=${nodeId}`;
      
      // 构建URL，包含查询参数
      const url = new URL(`${this.serverUrl}/figma/node/${fileKey}/${nodeId}`);
      url.searchParams.append('name', name);
      url.searchParams.append('figma_url', figmaUrl);
      
      const response = await fetch(url.toString(), {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${this.token}`
        }
      });
      
      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to create project');
      }
      
      return await response.json();
    } catch (error) {
      console.error('Create project error:', error);
      throw error;
    }
  }

  // Export a design from Figma
  async exportDesign(projectId, format = 'png') {
    try {
      if (!this.isInitialized()) {
        throw new Error('Service not initialized');
      }
      
      const response = await fetch(`${this.serverUrl}/figma/project/${projectId}/export`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.token}`
        },
        body: JSON.stringify({ format })
      });
      
      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to export design');
      }
      
      const data = await response.json();
      return { jobId: data.job_id };
    } catch (error) {
      console.error('Export design error:', error);
      throw error;
    }
  }

  // Check the status of an export job
  async checkExportStatus(jobId) {
    try {
      if (!this.isInitialized()) {
        throw new Error('Service not initialized');
      }
      
      const response = await fetch(`${this.serverUrl}/figma/export/${jobId}`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${this.token}`
        }
      });
      
      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to check export status');
      }
      
      return await response.json();
    } catch (error) {
      console.error('Check export status error:', error);
      throw error;
    }
  }

  // Get the download URL for an exported design
  getExportDownloadUrl(jobId) {
    if (!this.isInitialized()) {
      throw new Error('Service not initialized');
    }
    
    return `${this.serverUrl}/figma/export/${jobId}/download`;
  }

  // Preview single node using Figma Plugin API
  async previewNode(nodeId, scale = 1.0, format = 'PNG') {
    return new Promise((resolve, reject) => {
      // Set up message listener for the response
      const messageHandler = (event) => {
        const msg = event.data.pluginMessage;
        if (!msg) return;

        if (msg.type === 'previewNodeResult' && msg.data.nodeId === nodeId) {
          window.removeEventListener('message', messageHandler);
          resolve(msg.data);
        } else if (msg.type === 'previewError') {
          window.removeEventListener('message', messageHandler);
          reject(new Error(msg.message));
        }
      };

      window.addEventListener('message', messageHandler);

      // Send preview request to plugin
      parent.postMessage({ 
        pluginMessage: { 
          type: 'previewNode', 
          data: { nodeId, scale, format } 
        } 
      }, '*');

      // Set timeout to avoid hanging
      setTimeout(() => {
        window.removeEventListener('message', messageHandler);
        reject(new Error('Preview request timeout'));
      }, 30000); // 30 seconds timeout
    });
  }

  // Preview multiple nodes using Figma Plugin API
  async previewNodes(nodeIds, scale = 1.0, format = 'PNG') {
    return new Promise((resolve, reject) => {
      // Set up message listener for the response
      const messageHandler = (event) => {
        const msg = event.data.pluginMessage;
        if (!msg) return;

        if (msg.type === 'previewNodesResult') {
          window.removeEventListener('message', messageHandler);
          resolve(msg.data);
        } else if (msg.type === 'previewError') {
          window.removeEventListener('message', messageHandler);
          reject(new Error(msg.message));
        }
      };

      window.addEventListener('message', messageHandler);

      // Send preview request to plugin
      parent.postMessage({ 
        pluginMessage: { 
          type: 'previewNodes', 
          data: { nodeIds, scale, format } 
        } 
      }, '*');

      // Set timeout to avoid hanging
      setTimeout(() => {
        window.removeEventListener('message', messageHandler);
        reject(new Error('Batch preview request timeout'));
      }, 60000); // 60 seconds timeout for batch operations
    });
  }
}

// Export the service
const figmaDeliverService = new FigmaDeliverService();

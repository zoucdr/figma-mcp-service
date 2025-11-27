// 日志查看应用
new Vue({
    el: '#logs-app',
    data() {
        return {
            // 当前路径
            currentPath: window.initialData.relativePath || '',
            // 对象列表
            objects: [],
            // 加载状态
            loading: false,
            // 上传对话框
            uploadDialogVisible: false,
            uploadTabActive: 'web', // 默认显示网页上传标签页
            uploading: false,
            uploadForm: {},
            fileList: [],
            // 查看文件对话框
            viewDialogVisible: false,
            viewDialogTitle: '',
            loadingContent: false,
            fileContent: '',
            currentFile: null
        };
    },
    computed: {
        // 路径分段
        pathSegments() {
            if (!this.currentPath || this.currentPath === '/') {
                return [];
            }
            return this.currentPath.split('/').filter(s => s);
        }
    },
    mounted() {
        console.log('日志查看应用已加载');
        console.log('初始路径:', this.currentPath);
        this.loadList();
    },
    methods: {
        // 加载文件列表
        async loadList() {
            this.loading = true;
            try {
                const response = await axios.get('/tools/api/logs/list', {
                    params: {
                        path: this.currentPath
                    }
                });
                
                if (response.data.success) {
                    this.objects = response.data.objects || [];
                    console.log('加载文件列表成功:', this.objects.length, '个对象');
                } else {
                    this.$message.error(response.data.error || '加载失败');
                }
            } catch (error) {
                console.error('加载文件列表失败:', error);
                this.$message.error('加载失败: ' + (error.response?.data?.error || error.message));
            } finally {
                this.loading = false;
            }
        },
        
        // 刷新列表
        refreshList() {
            this.loadList();
        },
        
        // 导航到指定路径
        navigateTo(path) {
            this.currentPath = path;
            // 更新URL（不刷新页面）
            const newUrl = path ? `/tools/logs/${path}` : '/tools/logs';
            window.history.pushState({}, '', newUrl);
            this.loadList();
        },
        
        // 获取到指定分段的路径
        getPathUpToSegment(index) {
            return this.pathSegments.slice(0, index + 1).join('/');
        },
        
        // 处理行点击
        handleRowClick(row) {
            if (row.is_dir) {
                // 点击目录，进入该目录
                const newPath = this.currentPath 
                    ? `${this.currentPath}/${row.name}`
                    : row.name;
                this.navigateTo(newPath);
            } else {
                // 点击文件，查看内容
                this.viewFile(row);
            }
        },
        
        // 显示上传对话框
        showUploadDialog() {
            this.uploadDialogVisible = true;
            this.uploadTabActive = 'web'; // 重置为网页上传标签页
            this.fileList = [];
        },
        
        // 处理文件选择
        handleFileChange(file, fileList) {
            this.fileList = fileList;
        },
        
        // 提交上传
        async submitUpload() {
            if (this.fileList.length === 0) {
                this.$message.warning('请选择要上传的文件');
                return;
            }
            
            const file = this.fileList[0].raw;
            const formData = new FormData();
            formData.append('file', file);
            formData.append('path', this.currentPath);
            
            this.uploading = true;
            try {
                const response = await axios.post('/tools/api/logs/upload', formData, {
                    headers: {
                        'Content-Type': 'multipart/form-data'
                    }
                });
                
                if (response.data.success) {
                    this.$message.success('上传成功');
                    this.uploadDialogVisible = false;
                    this.fileList = [];
                    // 刷新列表
                    this.loadList();
                } else {
                    this.$message.error(response.data.error || '上传失败');
                }
            } catch (error) {
                console.error('上传失败:', error);
                this.$message.error('上传失败: ' + (error.response?.data?.error || error.message));
            } finally {
                this.uploading = false;
            }
        },
        
        // 查看文件
        async viewFile(row) {
            this.currentFile = row;
            this.viewDialogTitle = `查看文件: ${row.name}`;
            this.viewDialogVisible = true;
            this.loadingContent = true;
            this.fileContent = '';
            
            try {
                const response = await axios.get('/tools/api/logs/view', {
                    params: {
                        key: row.key
                    }
                });
                
                if (response.data.success) {
                    this.fileContent = response.data.content;
                } else {
                    this.$message.error(response.data.error || '加载文件内容失败');
                    this.viewDialogVisible = false;
                }
            } catch (error) {
                console.error('加载文件内容失败:', error);
                this.$message.error('加载失败: ' + (error.response?.data?.error || error.message));
                this.viewDialogVisible = false;
            } finally {
                this.loadingContent = false;
            }
        },
        
        // 下载文件
        downloadFile(row) {
            const url = `/tools/api/logs/download?key=${encodeURIComponent(row.key)}`;
            window.open(url, '_blank');
        },
        
        // 复制内容
        copyContent() {
            if (!this.fileContent) {
                this.$message.warning('没有可复制的内容');
                return;
            }
            
            // 创建临时textarea元素
            const textarea = document.createElement('textarea');
            textarea.value = this.fileContent;
            textarea.style.position = 'fixed';
            textarea.style.opacity = '0';
            document.body.appendChild(textarea);
            textarea.select();
            
            try {
                document.execCommand('copy');
                this.$message.success('已复制到剪贴板');
            } catch (error) {
                console.error('复制失败:', error);
                this.$message.error('复制失败');
            }
            
            document.body.removeChild(textarea);
        },
        
        // 回到首页
        goToHome() {
            window.location.href = '/';
        },
        
        // 获取API URL
        getApiUrl() {
            return window.location.origin;
        },
        
        // 复制API URL
        copyApiUrl() {
            const url = `${this.getApiUrl()}/tools/api/logs/upload`;
            this.copyToClipboard(url, 'API地址已复制到剪贴板');
        },
        
        // 复制cURL示例
        copyCurlExample() {
            const example = `curl -X POST ${this.getApiUrl()}/tools/api/logs/upload \\
  -F "file=@/path/to/logfile.txt" \\
  -F "path=mycoria/Windows"`;
            this.copyToClipboard(example, 'cURL示例已复制');
        },
        
        // 复制PowerShell示例
        copyPsExample() {
            const example = `$form = @{
    path = "mycoria/Windows"
    file = Get-Item -Path "C:\\logs\\test.log"
}
Invoke-RestMethod -Uri "${this.getApiUrl()}/tools/api/logs/upload" \`
  -Method Post -Form $form`;
            this.copyToClipboard(example, 'PowerShell示例已复制');
        },
        
        // 复制C#示例
        copyCsharpExample() {
            const example = `using var client = new HttpClient();
using var formData = new MultipartFormDataContent();

formData.Add(new StringContent("mycoria/Windows"), "path");

var fileBytes = File.ReadAllBytes(filePath);
var fileContent = new ByteArrayContent(fileBytes);
formData.Add(fileContent, "file", fileName);

var response = await client.PostAsync(
    "${this.getApiUrl()}/tools/api/logs/upload", 
    formData);`;
            this.copyToClipboard(example, 'C#示例已复制');
        },
        
        // 复制Python示例
        copyPythonExample() {
            const example = `import requests

files = {'file': open('/path/to/logfile.txt', 'rb')}
data = {'path': 'mycoria/Windows'}

response = requests.post(
    '${this.getApiUrl()}/tools/api/logs/upload',
    files=files,
    data=data
)`;
            this.copyToClipboard(example, 'Python示例已复制');
        },
        
        // 复制到剪贴板通用方法
        copyToClipboard(text, message) {
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            textarea.style.opacity = '0';
            document.body.appendChild(textarea);
            textarea.select();
            
            try {
                document.execCommand('copy');
                this.$message.success(message || '已复制到剪贴板');
            } catch (error) {
                console.error('复制失败:', error);
                this.$message.error('复制失败');
            }
            
            document.body.removeChild(textarea);
        },
        
        // 删除当前目录
        async deleteCurrentDirectory() {
            if (!this.currentPath) {
                this.$message.warning('无法删除根目录');
                return;
            }
            
            try {
                await this.$confirm(
                    `确定要删除目录 "${this.currentPath}" 及其下的所有文件和子目录吗？此操作不可恢复！`, 
                    '警告', 
                    {
                        confirmButtonText: '确定删除',
                        cancelButtonText: '取消',
                        type: 'warning',
                        dangerouslyUseHTMLString: true
                    }
                );
                
                const loading = this.$loading({
                    lock: true,
                    text: '正在删除...',
                    spinner: 'el-icon-loading',
                    background: 'rgba(0, 0, 0, 0.7)'
                });
                
                try {
                    const response = await axios.delete('/tools/api/logs/directory', {
                        params: {
                            path: this.currentPath
                        }
                    });
                    
                    loading.close();
                    
                    if (response.data.success) {
                        this.$message.success(response.data.message || '删除成功');
                        
                        // 返回上级目录
                        const segments = this.currentPath.split('/').filter(s => s);
                        if (segments.length > 1) {
                            // 有上级目录，跳转到上级
                            const parentPath = segments.slice(0, -1).join('/');
                            this.navigateTo(parentPath);
                        } else {
                            // 只有一级目录，跳转到根目录
                            this.navigateTo('');
                        }
                    } else {
                        this.$message.error(response.data.error || '删除失败');
                    }
                } catch (error) {
                    loading.close();
                    console.error('删除目录失败:', error);
                    this.$message.error('删除失败: ' + (error.response?.data?.error || error.message));
                }
            } catch {
                // 用户取消删除
            }
        },
        
        // 格式化文件大小
        formatSize(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        },
        
        // 格式化日期
        formatDate(dateString) {
            if (!dateString) return '-';
            const date = new Date(dateString);
            const year = date.getFullYear();
            const month = String(date.getMonth() + 1).padStart(2, '0');
            const day = String(date.getDate()).padStart(2, '0');
            const hours = String(date.getHours()).padStart(2, '0');
            const minutes = String(date.getMinutes()).padStart(2, '0');
            const seconds = String(date.getSeconds()).padStart(2, '0');
            return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
        }
    }
});



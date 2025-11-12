// Android代码预览页面JavaScript
new Vue({
    el: '#android-code-preview-app',
    data: {
        projectId: 0,
        projectName: '',
        codeFiles: {},
        activeFile: '',
        generatingPreview: false,
        androidExportConfig: {
            imageFormat: 'png',
            imageScale: 2.0,
            codeStyle: 'kotlin-constraintlayout',
            options: ['generateExtensions', 'generateResourceManager', 'generateStyles', 'generateColors']
        },
        csrfToken: '',
        androidExporting: false
    },
    mounted() {
        // 从全局变量加载初始数据
        if (window.androidCodePreviewData) {
            this.projectId = window.androidCodePreviewData.projectId;
            this.projectName = window.androidCodePreviewData.projectName;
            this.codeFiles = window.androidCodePreviewData.codeFiles || {};
            this.androidExportConfig = window.androidCodePreviewData.exportConfig || this.androidExportConfig;
            this.csrfToken = window.androidCodePreviewData.csrfToken;
            
            // 如果有文件，选择第一个
            const fileNames = Object.keys(this.codeFiles);
            if (fileNames.length > 0) {
                this.activeFile = fileNames[0];
            }
        }
        
        // 如果没有代码文件，自动生成预览
        if (Object.keys(this.codeFiles).length === 0) {
            this.generateAndroidCodePreview();
        }
    },
    methods: {
        // 生成Android代码预览
        async generateAndroidCodePreview() {
            this.generatingPreview = true;
            try {
                const response = await fetch(`/figma/project/${this.projectId}/android/preview-full`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': this.csrfToken
                    },
                    body: JSON.stringify(this.androidExportConfig)
                });

                const data = await response.json();
                
                if (data.success) {
                    this.codeFiles = data.files || {};
                    
                    // 选择第一个文件
                    const fileNames = Object.keys(this.codeFiles);
                    if (fileNames.length > 0) {
                        this.activeFile = fileNames[0];
                    }
                    
                    this.$message.success('Android代码预览生成成功');
                } else {
                    this.$message.error(data.error || 'Android代码预览生成失败');
                }
            } catch (error) {
                console.error('生成Android代码预览失败:', error);
                this.$message.error('网络错误，请重试');
            } finally {
                this.generatingPreview = false;
            }
        },

        // 从预览导出Android代码
        async exportAndroidCodeFromPreview() {
            this.androidExporting = true;
            try {
                const response = await fetch(`/figma/project/${this.projectId}/export/android`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': this.csrfToken
                    },
                    body: JSON.stringify(this.androidExportConfig)
                });

                const data = await response.json();
                
                if (data.success) {
                    this.$message.success('Android代码导出任务已创建，请在主页面查看进度');
                    
                    // 可选：打开主页面的导出状态
                    if (window.opener) {
                        try {
                            window.opener.postMessage({
                                type: 'exportStarted',
                                jobId: data.job_id,
                                platform: 'android'
                            }, '*');
                        } catch (e) {
                            console.log('无法向父窗口发送消息:', e);
                        }
                    }
                } else {
                    this.$message.error(data.error || 'Android代码导出失败');
                }
            } catch (error) {
                console.error('导出Android代码失败:', error);
                this.$message.error('网络错误，请重试');
            } finally {
                this.androidExporting = false;
            }
        },

        // 选择文件
        selectFile(filename) {
            this.activeFile = filename;
        },

        // 复制文件内容到剪贴板
        async copyFile(filename, content) {
            try {
                await navigator.clipboard.writeText(content);
                this.$message.success(`已复制 ${filename} 到剪贴板`);
            } catch (error) {
                console.error('复制失败:', error);
                
                // 降级方案：使用传统方法
                const textArea = document.createElement('textarea');
                textArea.value = content;
                textArea.style.position = 'fixed';
                textArea.style.opacity = '0';
                document.body.appendChild(textArea);
                textArea.select();
                
                try {
                    document.execCommand('copy');
                    this.$message.success(`已复制 ${filename} 到剪贴板`);
                } catch (fallbackError) {
                    this.$message.error('复制失败，请手动选择复制');
                } finally {
                    document.body.removeChild(textArea);
                }
            }
        },

        // 获取文件图标
        getFileIcon(filename) {
            const ext = filename.split('.').pop().toLowerCase();
            
            switch (ext) {
                case 'kt':
                case 'java':
                    return 'el-icon-document';
                case 'xml':
                    return 'el-icon-files';
                case 'gradle':
                    return 'el-icon-setting';
                case 'properties':
                    return 'el-icon-tickets';
                default:
                    return 'el-icon-document';
            }
        },

        // 获取文件大小（字符数）
        getFileSize(content) {
            if (!content) return '0 字符';
            
            const size = content.length;
            if (size < 1000) {
                return `${size} 字符`;
            } else if (size < 1000000) {
                return `${(size / 1000).toFixed(1)}K 字符`;
            } else {
                return `${(size / 1000000).toFixed(1)}M 字符`;
            }
        }
    },
    computed: {
        // 当前文件的语言类型
        currentFileLanguage() {
            if (!this.activeFile) return 'text';
            
            const ext = this.activeFile.split('.').pop().toLowerCase();
            switch (ext) {
                case 'kt':
                    return 'kotlin';
                case 'java':
                    return 'java';
                case 'xml':
                    return 'xml';
                case 'gradle':
                    return 'gradle';
                case 'properties':
                    return 'properties';
                default:
                    return 'text';
            }
        }
    },
    watch: {
        // 监听活动文件变化，可以在这里添加语法高亮等功能
        activeFile(newFile, oldFile) {
            if (newFile !== oldFile) {
                // 这里可以添加代码高亮逻辑
                this.$nextTick(() => {
                    // 例如：使用 Prism.js 或其他语法高亮库
                    // this.highlightCode();
                });
            }
        }
    }
});

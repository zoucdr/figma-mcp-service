/**
 * Figma Bridge 项目列表脚本
 */

// 项目列表页面
const ProjectListApp = {
    data() {
        return {
            projects: window.initialData?.projects || [],
            filteredProjects: [],
            linkDialogVisible: false,
            editDialogVisible: false,
            currentEditProject: null,
            parsing: false,
            searchQuery: '',
            sortOrder: 'desc',  // 默认按时间降序排列（最新的在前面）
            linkForm: {
                name: '',
                link: ''
            },
            editForm: {
                name: '',
                figma_url: '',
                file_key: '',
                root_node_id: ''
            },
            linkRules: {
                name: [
                    { required: true, message: '请输入项目名称', trigger: 'blur' }
                ],
                link: [
                    { required: true, message: '请输入Figma链接', trigger: 'blur' },
                    { pattern: /figma\.com/, message: '请输入有效的Figma链接', trigger: 'blur' }
                ]
            }
        };
    },
    
    // 计算属性
    computed: {
        // 处理过的项目列表（排序和筛选）
        processedProjects() {
            // 首先进行搜索过滤
            let result = this.projects;
            if (this.searchQuery) {
                const query = this.searchQuery.toLowerCase();
                result = result.filter(project => {
                    return project.name.toLowerCase().includes(query) || 
                           project.file_key.toLowerCase().includes(query) ||
                           (project.figma_url && project.figma_url.toLowerCase().includes(query));
                });
            }
            
            // 然后进行排序
            result = [...result].sort((a, b) => {
                const dateA = new Date(a.created_at);
                const dateB = new Date(b.created_at);
                
                if (this.sortOrder === 'asc') {
                    return dateA - dateB;
                } else {
                    return dateB - dateA;
                }
            });
            
            return result;
        }
    },
    
    // 监听器
    watch: {
        // 当项目列表变化时更新过滤后的列表
        projects: {
            immediate: true,
            handler(newVal) {
                this.filteredProjects = [...newVal];
            }
        }
    },
    
    methods: {
        // 用户菜单处理
        handleCommand(command) {
            if (command === 'profile') {
                window.location.href = '/profile';
            } else if (command === 'logout') {
                window.location.href = '/logout';
            }
        },
        
        // 显示添加链接对话框
        showLinkDialog() {
            this.linkDialogVisible = true;
        },
        
        // 解析Figma链接
        parseFigmaLink() {
            this.$refs.linkForm.validate(valid => {
                if (valid) {
                    this.parsing = true;
                    const formData = new FormData();
                    formData.append('link', this.linkForm.link);
                    formData.append('name', this.linkForm.name);
                    
                    axios.post('/figma/parse', formData)
                        .then(response => {
                            const { fileKey, nodeId } = response.data;
                            this.linkDialogVisible = false;
                            this.getFigmaNode(fileKey, nodeId);
                        })
                        .catch(error => {
                            this.$message.error(error.response?.data?.error || '解析链接失败');
                        })
                        .finally(() => {
                            this.parsing = false;
                        });
                }
            });
        },
        
        // 获取Figma节点数据
        getFigmaNode(fileKey, nodeId) {
            this.$message({
                message: '正在获取Figma节点数据，请稍候...',
                type: 'info',
                duration: 0,
                showClose: true
            });
            
            // 获取项目名称和Figma URL
            const name = this.linkForm.name;
            const figmaURL = this.linkForm.link;
            
            axios.get(`/figma/node/${fileKey}/${nodeId || ''}?name=${encodeURIComponent(name)}&figma_url=${encodeURIComponent(figmaURL)}`)
                .then(response => {
                    this.$message.closeAll();
                    this.$message.success('获取节点数据成功');
                    // 更新项目列表
                    this.projects = [...this.projects, response.data.project];
                    // 跳转到编辑页面
                    window.location.href = `/dashboard?project=${response.data.project.id}`;
                })
                .catch(error => {
                    this.$message.closeAll();
                    this.$message.error(error.response?.data?.error || '获取节点数据失败');
                });
        },
        
        // 打开项目
        openProject(project) {
            window.location.href = `/dashboard?project=${project.id}`;
        },
        
        // 打开Figma链接
        openFigmaLink(figmaUrl) {
            if (figmaUrl) {
                window.open(figmaUrl, '_blank');
            }
        },
        
        // 显示编辑对话框
        showEditDialog(project) {
            this.currentEditProject = project;
            this.editForm.name = project.name;
            this.editForm.figma_url = project.figma_url || '';
            this.editForm.file_key = project.file_key;
            this.editForm.root_node_id = project.root_node_id;
            this.editDialogVisible = true;
        },
        
        // 更新项目
        updateProject() {
            if (!this.currentEditProject) return;
            
            const formData = new FormData();
            formData.append('name', this.editForm.name);
            formData.append('figma_url', this.editForm.figma_url);
            
            axios.put(`/figma/project/${this.currentEditProject.id}`, formData)
                .then(() => {
                    this.$message.success('项目更新成功');
                    // 更新本地项目列表中的项目信息
                    const index = this.projects.findIndex(p => p.id === this.currentEditProject.id);
                    if (index !== -1) {
                        this.projects[index].name = this.editForm.name;
                        this.projects[index].figma_url = this.editForm.figma_url;
                        this.projects = [...this.projects]; // 触发视图更新
                    }
                    this.editDialogVisible = false;
                })
                .catch(error => {
                    this.$message.error(error.response?.data?.error || '更新项目失败');
                });
        },
        
        // 删除项目
        deleteProject(project) {
            this.$confirm('确定要删除该项目吗？', '提示', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                type: 'warning'
            }).then(() => {
                axios.delete(`/figma/project/${project.id}`)
                    .then(() => {
                        this.$message.success('项目删除成功');
                        this.projects = this.projects.filter(p => p.id !== project.id);
                    })
                    .catch(error => {
                        this.$message.error(error.response?.data?.error || '删除项目失败');
                    });
            }).catch(() => {});
        },
        
        // 切换排序方式
        toggleSortOrder() {
            this.sortOrder = this.sortOrder === 'desc' ? 'asc' : 'desc';
        },
        
        // 搜索项目
        searchProjects() {
            // 搜索通过计算属性自动处理
        },
        
        // 清除搜索
        clearSearch() {
            this.searchQuery = '';
        },
        
        // 格式化日期
        formatDate(dateString) {
            if (!dateString) return '';
            const date = new Date(dateString);
            return date.toLocaleString('zh-CN', { 
                year: 'numeric', 
                month: '2-digit', 
                day: '2-digit',
                hour: '2-digit',
                minute: '2-digit'
            });
        }
    }
};

// 导出Vue应用配置
window.ProjectListApp = ProjectListApp;
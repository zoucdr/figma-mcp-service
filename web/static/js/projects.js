/**
 * Figma Deliver 项目列表脚本
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
            sortOrder: 'desc',  // 默认按降序排列
            sortField: 'time',  // 默认按时间排序
            expandedGroups: {}, // 分组展开状态，key为fileKey，value为boolean
            linkForm: {
                name: '',
                group_name: '',
                link: ''
            },
            editForm: {
                name: '',
                group_name: '',
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
        // 处理过的项目列表（按分组名称或FileKey分组）
        processedProjects() {
            // 首先进行搜索过滤
            let result = this.projects;
            if (this.searchQuery) {
                const query = this.searchQuery.toLowerCase();
                result = result.filter(project => {
                    return project.name.toLowerCase().includes(query) || 
                           project.file_key.toLowerCase().includes(query) ||
                           (project.group_name && project.group_name.toLowerCase().includes(query)) ||
                           (project.figma_url && project.figma_url.toLowerCase().includes(query));
                });
            }
            
            // 按分组名称或FileKey分组
            const groupedProjects = {};
            result.forEach(project => {
                // 如果有分组名称，使用分组名称；否则使用file_key
                const groupKey = project.group_name || project.file_key || '未知文件';
                if (!groupedProjects[groupKey]) {
                    groupedProjects[groupKey] = [];
                }
                groupedProjects[groupKey].push(project);
            });
            
            // 对每个分组内的项目进行排序
            Object.keys(groupedProjects).forEach(fileKey => {
                groupedProjects[fileKey].sort((a, b) => {
                    if (this.sortField === 'time') {
                        const dateA = new Date(a.created_at);
                        const dateB = new Date(b.created_at);
                        
                        if (this.sortOrder === 'asc') {
                            return dateA - dateB;
                        } else {
                            return dateB - dateA;
                        }
                    } else if (this.sortField === 'name') {
                        const nameA = a.name.toLowerCase();
                        const nameB = b.name.toLowerCase();
                        
                        if (this.sortOrder === 'asc') {
                            return nameA.localeCompare(nameB);
                        } else {
                            return nameB.localeCompare(nameA);
                        }
                    }
                    
                    // 默认按时间排序
                    return this.sortOrder === 'asc' ? 
                        new Date(a.created_at) - new Date(b.created_at) : 
                        new Date(b.created_at) - new Date(a.created_at);
                });
            });
            
            // 对分组按分组键排序
            const sortedGroups = Object.keys(groupedProjects).sort((a, b) => {
                if (this.sortField === 'fileKey') {
                    if (this.sortOrder === 'asc') {
                        return a.localeCompare(b);
                    } else {
                        return b.localeCompare(a);
                    }
                }
                // 默认按分组键升序排序
                return a.localeCompare(b);
            });
            
            // 返回分组后的数据结构
            return sortedGroups.map(groupKey => {
                const projects = groupedProjects[groupKey];
                let displayName;
                
                // 检查是否有分组名称
                const hasGroupName = projects.some(p => p.group_name);
                
                // 统一按项目名组合方式显示
                if (projects.length === 1) {
                    // 只有一个项目时直接显示项目名
                    displayName = projects[0].name;
                } else if (projects.length <= 3) {
                    // 3个或以下项目时显示所有项目名
                    displayName = projects.map(p => p.name).join('、');
                } else {
                    // 超过3个项目时显示前2个项目名 + "等N个项目"
                    const firstTwo = projects.slice(0, 2).map(p => p.name).join('、');
                    displayName = `${firstTwo}等${projects.length}个项目`;
                }
                
                return {
                    groupKey: groupKey,
                    fileKey: projects[0].file_key, // 添加 file_key 用于显示
                    projects: projects,
                    projectCount: projects.length,
                    displayName: displayName, // 分组显示名称
                    groupName: hasGroupName ? groupKey : '', // 分组名称（自定义组名）
                    expanded: this.expandedGroups[groupKey] === true // 默认收起，除非明确设置为true
                };
            });
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
            } else if (command === 'about') {
                window.location.href = '/about';
            } else if (command === 'logout') {
                window.location.href = '/logout';
            }
        },
        
        // 显示添加链接对话框
        showLinkDialog() {
            // 重置表单
            this.linkForm = {
                name: '',
                group_name: '',
                link: ''
            };
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
                    formData.append('group_name', this.linkForm.group_name);
                    
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
            
            // 获取项目名称、分组名称和Figma URL
            const name = this.linkForm.name;
            const groupName = this.linkForm.group_name;
            const figmaURL = this.linkForm.link;
            
            axios.get(`/figma/node/${fileKey}/${nodeId || ''}?name=${encodeURIComponent(name)}&figma_url=${encodeURIComponent(figmaURL)}&group_name=${encodeURIComponent(groupName)}`)
                .then(response => {
                    this.$message.closeAll();
                    this.$message.success('项目创建成功！');
                    
                    // 后端已支持 group_name 参数，直接使用返回的数据
                    const data = response.data;
                    
                    // 更新项目列表
                    this.projects = [...this.projects, data.project];
                    
                    // 关闭对话框
                    this.linkDialogVisible = false;
                    
                    // 重置表单
                    this.linkForm = {
                        name: '',
                        group_name: '',
                        link: ''
                    };
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
            this.editForm.group_name = project.group_name || '';
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
            formData.append('group_name', this.editForm.group_name);
            formData.append('figma_url', this.editForm.figma_url);
            
            axios.put(`/figma/project/${this.currentEditProject.id}`, formData)
                .then(() => {
                    this.$message.success('项目更新成功');
                    // 更新本地项目列表中的项目信息
                    const index = this.projects.findIndex(p => p.id === this.currentEditProject.id);
                    if (index !== -1) {
                        this.projects[index].name = this.editForm.name;
                        this.projects[index].group_name = this.editForm.group_name;
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
        toggleSortOrder(field) {
            // 如果点击的是当前排序字段，则切换排序顺序
            if (field === this.sortField) {
                this.sortOrder = this.sortOrder === 'desc' ? 'asc' : 'desc';
            } else {
                // 如果点击的是新字段，设置为该字段并使用降序排序
                this.sortField = field;
                this.sortOrder = 'desc';
            }
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
        },
        
        // 切换分组展开状态
        toggleGroupExpanded(groupKey) {
            this.$set(this.expandedGroups, groupKey, !this.expandedGroups[groupKey]);
        },
        
        // 点击分组标题跳转到第一个项目的dashboard
        navigateToFirstProject(group) {
            if (group.projects && group.projects.length > 0) {
                const firstProject = group.projects[0];
                window.location.href = `/dashboard?project=${firstProject.id}`;
            }
        },
        
        // 展开所有分组
        expandAllGroups() {
            this.processedProjects.forEach(group => {
                this.$set(this.expandedGroups, group.groupKey, true);
            });
        },
        
        // 收起所有分组
        collapseAllGroups() {
            this.processedProjects.forEach(group => {
                this.$set(this.expandedGroups, group.groupKey, false);
            });
        },
        
        // 复制 FileKey
        copyFileKey(fileKey) {
            if (!fileKey) return;
            
            // 使用 Clipboard API 复制
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(fileKey).then(() => {
                    this.$message.success('FileKey 已复制到剪贴板');
                }).catch(err => {
                    console.error('复制失败:', err);
                    this.fallbackCopyFileKey(fileKey);
                });
            } else {
                // 降级方案：使用传统方法
                this.fallbackCopyFileKey(fileKey);
            }
        },
        
        // 降级复制方法
        fallbackCopyFileKey(fileKey) {
            const textArea = document.createElement('textarea');
            textArea.value = fileKey;
            textArea.style.position = 'fixed';
            textArea.style.left = '-999999px';
            textArea.style.top = '-999999px';
            document.body.appendChild(textArea);
            textArea.focus();
            textArea.select();
            
            try {
                const successful = document.execCommand('copy');
                if (successful) {
                    this.$message.success('FileKey 已复制到剪贴板');
                } else {
                    this.$message.error('复制失败，请手动复制');
                }
            } catch (err) {
                console.error('复制失败:', err);
                this.$message.error('复制失败，请手动复制');
            }
            
            document.body.removeChild(textArea);
        },
        
        /**
         * 批量更新分组的节点树
         * @param {Object} group - 分组对象
         */
        async batchRefreshNodeTree(group) {
            // 收集该分组的所有项目
            const projects = group.projects || [];
            
            if (projects.length === 0) {
                this.$message.warning('该分组没有项目');
                return;
            }
            
            // 按 file_key 分组
            const projectsByFileKey = {};
            
            for (const project of projects) {
                const fileKey = project.file_key;
                const rootNodeId = project.root_node_id;
                
                if (!fileKey || !rootNodeId) {
                    console.warn('项目缺少 file_key 或 root_node_id:', project);
                    continue;
                }
                
                if (!projectsByFileKey[fileKey]) {
                    projectsByFileKey[fileKey] = {
                        fileKey: fileKey,
                        nodeIds: new Set(),
                        projectNames: []
                    };
                }
                
                projectsByFileKey[fileKey].nodeIds.add(rootNodeId);
                projectsByFileKey[fileKey].projectNames.push(project.name);
            }
            
            const fileKeys = Object.keys(projectsByFileKey);
            
            if (fileKeys.length === 0) {
                this.$message.warning('没有找到有效的项目');
                return;
            }
            
            // 显示确认对话框
            const totalProjects = projects.length;
            const totalNodes = Array.from(new Set(projects.map(p => p.root_node_id))).length;
            const fileKeysInfo = fileKeys.map(fk => {
                const data = projectsByFileKey[fk];
                return `- ${fk.substring(0, 12)}... (${data.nodeIds.size}个节点)`;
            }).join('\n');
            
            try {
                await this.$confirm(
                    `将批量更新以下节点树：\n\n` +
                    `📁 分组：${group.displayName}\n` +
                    `📊 包含：${totalProjects}个项目，${totalNodes}个节点\n` +
                    `📦 涉及 ${fileKeys.length} 个 Figma 文件：\n\n${fileKeysInfo}\n\n` +
                    `确认要批量更新吗？`,
                    '批量更新节点树',
                    {
                        confirmButtonText: '确定更新',
                        cancelButtonText: '取消',
                        type: 'warning',
                        dangerouslyUseHTMLString: false
                    }
                );
            } catch {
                // 用户取消
                return;
            }
            
            // 显示加载状态
            const loading = this.$loading({
                lock: true,
                text: '正在批量更新节点树...',
                spinner: 'el-icon-loading'
            });
            
            try {
                let successCount = 0;
                let queuedCount = 0;
                let errorCount = 0;
                const errors = [];
                
                // 为每个 file_key 调用批量更新接口
                for (const fileKey of fileKeys) {
                    const data = projectsByFileKey[fileKey];
                    const nodeIds = Array.from(data.nodeIds);
                    
                    try {
                        console.log('📤 [batchRefreshNodeTree] 批量更新请求:', { 
                            file_key: fileKey, 
                            node_ids: nodeIds 
                        });
                        
                        const response = await axios.post('/figma/batch-refresh-node-tree', {
                            file_key: fileKey,
                            node_ids: nodeIds
                        });
                        
                        if (response.data.success) {
                            if (response.data.status === 'queued') {
                                queuedCount++;
                                const cooldown = response.data.cooldown_remaining || 0;
                                console.log(`⏰ [batchRefreshNodeTree] 文件 ${fileKey} 加入队列，等待 ${cooldown} 秒`);
                            } else if (response.data.status === 'success') {
                                successCount++;
                                console.log(`✅ [batchRefreshNodeTree] 文件 ${fileKey} 更新成功`);
                            }
                        } else {
                            errorCount++;
                            errors.push(`${fileKey}: ${response.data.error || '未知错误'}`);
                        }
                    } catch (error) {
                        errorCount++;
                        const errorMsg = error.response?.data?.error || error.message || '请求失败';
                        errors.push(`${fileKey}: ${errorMsg}`);
                        console.error(`批量更新失败 (${fileKey}):`, error);
                    }
                }
                
                loading.close();
                
                // 显示结果
                if (successCount > 0 || queuedCount > 0) {
                    let message = '';
                    if (successCount > 0) {
                        message += `✅ ${successCount} 个文件更新成功`;
                    }
                    if (queuedCount > 0) {
                        if (message) message += '，';
                        message += `⏰ ${queuedCount} 个文件加入队列`;
                    }
                    if (errorCount > 0) {
                        if (message) message += '，';
                        message += `❌ ${errorCount} 个文件失败`;
                    }
                    
                    if (errorCount > 0) {
                        this.$notify({
                            title: '批量更新完成（部分失败）',
                            message: message,
                            type: 'warning',
                            duration: 5000
                        });
                        
                        // 显示错误详情
                        this.$alert(
                            errors.join('\n'),
                            '更新失败详情',
                            {
                                confirmButtonText: '确定',
                                type: 'error'
                            }
                        );
                    } else {
                        this.$message.success(message);
                    }
                } else {
                    this.$message.error('批量更新全部失败');
                    if (errors.length > 0) {
                        this.$alert(
                            errors.join('\n'),
                            '更新失败详情',
                            {
                                confirmButtonText: '确定',
                                type: 'error'
                            }
                        );
                    }
                }
            } catch (error) {
                loading.close();
                console.error('批量更新异常:', error);
                this.$message.error('批量更新过程中发生异常');
            }
        }
    }
};

// 导出Vue应用配置
window.ProjectListApp = ProjectListApp;
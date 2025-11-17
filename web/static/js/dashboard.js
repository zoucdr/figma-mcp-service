/**
 * Figma Deliver 项目编辑器脚本
 * 
 * 过滤预览图片加载策略：
 * 1. 只有手动点击展开过滤预览时才加载所有图片
 * 2. 切换节点时，只有在过滤预览已经展开的情况下才加载图片
 * 3. 其他情况下禁止发送获取所有图片的请求
 * 
 * 这些修改可以减少不必要的网络请求，提高性能和用户体验
 */

// 项目编辑页面
const ProjectEditorApp = {
    data() {
        return {
            project: window.initialData?.project || null,
            nodes: [],
            nodeModifys: {},
            refNodes: [], // 依赖节点列表
            refNodeDetails: {}, // 依赖节点详细信息
            currentNode: null,
            treeData: [],
            nodeSettings: {},
            previewImages: [], // 预览图片列表
            preloadedNodes: [], // 预加载的节点列表（占位区域）
            previewScale: 1, // 预览缩放比例
            zoomLevel: 1, // 当前缩放级别
            minZoom: 0.2, // 最小缩放级别
            maxZoom: 5, // 最大缩放级别
            zoomStep: 0.2, // 每次缩放步长
            previewOffset: { x: 0, y: 0 }, // 预览偏移量
            filteredNodeImage: null, // 过滤后的节点图片URL
            filteredNodeImages: {}, // 批量过滤预览图片（节点ID到图片路径的映射）
            filteredImagesList: [], // 过滤预览图片列表（用于滑动显示）
            currentFilteredImageIndex: 0, // 当前显示的过滤图片索引
            isFilteredPreviewExpanded: false, // 过滤预览是否展开
            isCurrentPreviewExpanded: true, // 当前预览是否展开（默认展开）
            isLoadingFilteredImages: false, // 是否正在加载批量过滤图片
            rootNodeBounds: null, // 根节点边界
            refNodeRootBounds: null, // 依赖节点根节点边界
            refPreviewImages: [], // 依赖节点预览图片列表
            selectionOverlayStyle: null, // 选中节点的高层级预览样式
            layerMenuVisible: false, // 层级菜单是否可见（默认隐藏，不是一加载就显示）
            layerMenuStyle: { // 层级菜单样式
                top: '0px',
                left: '0px'
            },
            contextMenuNodeId: null, // 右键点击的节点ID
            contextMenuIsRefGroup: false, // 是否是依赖节点分组
            contextMenuIsRefNode: false, // 是否是依赖节点
            contextMenuIsRefChild: false, // 是否是依赖节点的子节点
            contextMenuIsEmptyArea: false, // 是否是空白区域
            contextMenuSource: null, // 右键菜单来源：'preview'（预览图）、'tree'（节点树）或'minimap'（缩略图）
            contextMenuMinimapType: null, // 缩略图类型：'current'（当前预览）或'filtered'（过滤预览）
            // 批量选择相关
            selectedNodes: [], // 当前选中的节点ID列表
            lastSelectedNode: null, // 最后选中的节点ID（用于Shift范围选择）
            isShiftPressed: false, // Shift键是否被按下
            isCtrlPressed: false, // Ctrl键是否被按下
            filteredPreviewImages: [], // 过滤后的预览图片列表（用于层级菜单）
            resetModeMenuVisible: false, // 重置模式菜单是否可见
            resetModeMenuStyle: { // 重置模式菜单样式
                top: '0px',
                left: '0px'
            },
            exportStatus: '', // pending, processing, completed, failed
            exportProgress: 0, // 导出进度 0-100
            exportJobId: null,
            defaultImageFormat: 'png', // 默认图片格式
            defaultImageScale: 1.0, // 默认图片缩放比例
            exportingNodeId: null, // 正在导出的节点ID
            jsonPreviewDialogVisible: false, // JSON预览弹窗是否可见
            renameDialogVisible: false, // 重命名对话框是否可见
            renameProcessing: false, // 重命名处理中标志，防止重复调用
            renameForm: {
                nodeId: null,
                newName: ''
            },
            propertyForm: {
                img_ext: 'png',
                components: [],
                rename: '',
                ignore: false,
                res_mode: 'attach',
                img_name: '',
                img_id: '', // 图片ID
                horizontal: 'CENTER',
                vertical: "CENTER",
                parent_id: ''
            },
            loading: false,
            showCustomSettings: false,
            showLayoutSettings: false,
            showExportSettings: false,
            // 各部分展开状态
            sectionExpanded: {
                basicInfo: true,
                controlSettings: true,
                layoutSettings: true,
                exportSettings: true
            },
            expandedKeys: [], // 要展开的节点ID列表
            includeChildrenInJson: false, // 是否在JSON中包含子节点数据
            nodeJsonString: '', // 格式化后的节点JSON字符串
            // 拖拽相关
            dragStartX: 0, // 拖拽开始时的X坐标
            dragStartY: 0, // 拖拽开始时的Y坐标
            isDragging: false, // 是否正在拖拽
            dragPossible: false, // 是否可能开始拖拽（鼠标按下但未确认为拖拽）
            clickStartTime: 0, // 点击开始时间，用于区分点击和拖拽
            isOverNode: false, // 鼠标是否悬停在节点上
            // 记录上次点击位置的匹配节点和当前索引，用于循环选择
            lastClickPosition: {
                x: null,
                y: null,
                matchingNodeIds: [],
                currentIndex: -1,
                timestamp: 0 // 用于判断是否是短时间内的重复点击
            },
            // MCP相关数据
            mcpPanelVisible: false, // MCP面板是否可见
            mcpLogs: [], // MCP调用日志
            expandedLogs: {}, // 展开的日志项
            // 界面描述相关
            interfaceDescription: '', // 界面描述内容
            interfaceDescriptionChanged: false, // 界面描述是否有变化
            // 下载按钮右键菜单
            downloadMenuVisible: false,
            downloadMenuStyle: {
                top: '0px',
                left: '0px'
            },
            downloadLevel: 1, // 下载简化级别：0=不砍任何原数据, 1=过滤空值, 2=激进简化
            // 项目切换相关
            siblingProjects: [], // 相同file_key下的所有项目
            currentProjectId: null, // 当前项目ID
            selectedProjectId: null, // 下拉框选中的项目ID（用于显示）
            loadingSiblingProjects: false, // 是否正在加载兄弟项目
            // 防抖定时器
            renameTimer: null,
            parentIdTimer: null,
            imgNameTimer: null,
            imgIdTimer: null,
            settingsSaveTimer: null, // 项目设置保存防抖定时器
            isInitializingSettings: true, // 是否正在初始化设置，防止初始化时触发保存
            // 节点搜索相关
            nodeSearchKeyword: '', // 节点搜索关键词
            searchMatchedNodes: [], // 搜索匹配的节点列表
            currentSearchIndex: 0, // 当前搜索结果索引
            // 模板代码导出相关
            swiftExportDialogVisible: false, // Swift导出弹窗是否可见
            swiftExporting: false, // 是否正在导出Swift代码
            generatingPreview: false, // 是否正在生成预览
            swiftExportPreview: '', // Swift代码预览内容（已废弃）
            swiftExportConfig: { // Swift导出配置
                imageFormat: 'png',
                imageScale: 2.0,
                codeStyle: 'uikit-autolayout',
                options: ['generateExtensions', 'generateResourceManager']
            },
            // Android导出相关
            androidExportDialogVisible: false, // Android导出弹窗是否可见
            androidExporting: false, // 是否正在导出Android代码
            androidExportConfig: { // Android导出配置
                imageFormat: 'png',
                imageScale: 2.0,
                codeStyle: 'kotlin-constraintlayout',
                options: ['generateExtensions', 'generateResourceManager', 'generateStyles', 'generateColors']
            },
            // 代码预览弹窗相关
            swiftCodePreviewDialogVisible: false, // 代码预览弹窗是否可见
            swiftCodeFiles: {}, // Swift代码文件内容 {filename: content}
            activeCodeTab: '', // 当前激活的代码标签页
            // 面板宽度调整相关
            treePanelWidth: 300, // 左侧树形图面板宽度
            propertyPanelWidth: 350, // 右侧属性面板宽度（固定）
            isResizing: false, // 是否正在调整大小
            resizeStartX: 0, // 调整开始时的X坐标
            resizeStartWidth: 0, // 调整开始时的面板宽度
            // 创建项目相关
            createProjectDialogVisible: false, // 创建项目对话框是否可见
            creatingProject: false, // 是否正在创建项目
            createProjectForm: {
                name: '',
                group_name: '',
                link: ''
            },
            createProjectRules: {
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
    computed: {
        // 获取当前节点的子节点
        childNodes() {
            if (!this.currentNode) return [];
            return this.nodes.filter(node => node.parent_id === this.currentNode.id);
        },
        
        
        // 判断当前节点是否有修改信息
        hasNodeModifications() {
            return this.currentNode && this.currentNode.modifys;
        },
        
        // 节点启用状态（与ignore相反的逻辑）
        nodeEnabled: {
            get() {
                // 如果没有当前节点，返回true（默认启用）
                if (!this.currentNode) return true;
                // 如果ignore为true，则nodeEnabled为false；如果ignore为false或undefined，则nodeEnabled为true
                return !this.propertyForm.ignore;
            },
            set(value) {
                // 设置ignore为nodeEnabled的相反值
                this.propertyForm.ignore = !value;
            }
        },
        
        // 解析用户的控件类型列表
        compTypesList() {
            const compTypesStr = window.initialData?.compTypes || '';
            if (!compTypesStr) {
                // 如果用户没有设置控件类型，返回默认值
                return ['按钮', '图片', '文本', '输入框', '滚动区域'];
            }
            // 按逗号分割字符串，并去除空白项
            return compTypesStr.split(',')
                .map(type => type.trim())
                .filter(type => type);
        },
        
        // 生成API下载URL
        apiDownloadUrl() {
            if (!this.project || !this.project.file_key || !this.project.root_node_id) {
                return '';
            }
            
            // 从初始数据获取MCP token
            const mcpToken = window.initialData?.mcpToken;
            if (!mcpToken) {
                return '';
            }
            
            const baseUrl = window.location.origin;
            const fileKey = encodeURIComponent(this.project.file_key);
            const rootNodeId = encodeURIComponent(this.project.root_node_id);
            const level = this.downloadLevel || 1;
            
            return `${baseUrl}/api/${mcpToken}/optimized_nodes?file_key=${fileKey}&root_node_id=${rootNodeId}&level=${level}`;
        }
    },
    created() {
        this.loading = true;
        
        // 从项目配置中读取图片格式和缩放比例
        this.loadProjectSettings();
        
        this.loadProjectData();
        
        // 初始化当前项目ID
        if (this.project) {
            this.currentProjectId = this.project.id;
            this.selectedProjectId = this.project.id; // 初始化选中的项目ID
            console.log('Initialized currentProjectId:', this.currentProjectId, typeof this.currentProjectId);
            // 加载相同file_key下的所有项目
            this.loadSiblingProjects();
        }
        
        // 添加全局点击事件监听器，用于关闭层级菜单
        document.addEventListener('click', this.handleGlobalClick);
        
        // 加载MCP调用记录
        this.refreshMCPLogs();
    },
    
    mounted() {
        // 加载保存的面板宽度
        this.loadPanelWidths();
        
        // 添加鼠标移动和松开事件监听
        document.addEventListener('mousemove', this.handleResizeMove);
        document.addEventListener('mouseup', this.stopResize);
        
        // 添加键盘事件监听器（批量选择功能）
        document.addEventListener('keydown', this.handleKeyDown);
        document.addEventListener('keyup', this.handleKeyUp);
        
        // 检查是否有活跃的导出任务
        this.checkActiveExportJob();
    },
    
    beforeDestroy() {
        // 移除全局点击事件监听器
        document.removeEventListener('click', this.handleGlobalClick);
        
        // 移除鼠标移动和松开事件监听
        document.removeEventListener('mousemove', this.handleMouseMove);
        document.removeEventListener('mouseup', this.handleMouseUp);
        
        // 移除面板调整相关监听
        document.removeEventListener('mousemove', this.handleResizeMove);
        document.removeEventListener('mouseup', this.stopResize);
        
        // 移除键盘事件监听器
        document.removeEventListener('keydown', this.handleKeyDown);
        document.removeEventListener('keyup', this.handleKeyUp);
    },
    watch: {
        // 监听图片格式变化，自动保存到数据库
        defaultImageFormat(newVal) {
            if (!this.isInitializingSettings && this.project && this.project.id) {
                this.saveProjectSettings();
            }
        },
        // 监听图片缩放比例变化，自动保存到数据库
        defaultImageScale(newVal) {
            if (!this.isInitializingSettings && this.project && this.project.id) {
                this.saveProjectSettings();
            }
        }
    },
    methods: {
        // 键盘事件处理（批量选择功能）
        handleKeyDown(event) {
            if (event.key === 'Shift') {
                this.isShiftPressed = true;
            } else if (event.key === 'Control' || event.key === 'Meta') {
                this.isCtrlPressed = true;
            }
        },
        
        handleKeyUp(event) {
            if (event.key === 'Shift') {
                this.isShiftPressed = false;
            } else if (event.key === 'Control' || event.key === 'Meta') {
                this.isCtrlPressed = false;
            }
        },
        
        // 批量选择辅助方法
        // 获取所有可见的节点ID列表（用于范围选择）- 按层级组织
        getAllVisibleNodeIds() {
            const visibleNodes = [];
            const traverse = (nodes, level = 0) => {
                for (const node of nodes) {
                    // 跳过依赖节点分组和依赖节点
                    if (!node.isRefGroup && !node.isRefNode) {
                        visibleNodes.push({
                            id: node.id,
                            level: level
                        });
                    }
                    // 继续遍历子节点
                    if (node.children && node.children.length > 0) {
                        traverse(node.children, level + 1);
                    }
                }
            };
            traverse(this.treeData);
            // 返回按顺序排列的节点ID数组
            return visibleNodes.map(node => node.id);
        },
        
        // 获取节点及其同层级和上层节点（不包含子树）
        getNodesAtLevelAndAbove(targetNodeId) {
            const targetLevel = this.getNodeLevel(targetNodeId);
            if (targetLevel === -1) return [targetNodeId];
            
            const validNodes = [];
            const traverse = (nodes, level = 0) => {
                for (const node of nodes) {
                    // 跳过依赖节点分组和依赖节点
                    if (!node.isRefGroup && !node.isRefNode) {
                        // 只包含目标层级及以上的节点
                        if (level <= targetLevel) {
                            validNodes.push(node.id);
                        }
                    }
                    // 继续遍历子节点
                    if (node.children && node.children.length > 0) {
                        traverse(node.children, level + 1);
                    }
                }
            };
            traverse(this.treeData);
            return validNodes;
        },
        
        // 获取两个节点之间的范围节点ID列表（限制在目标层级及以上）
        getNodesBetween(startNodeId, endNodeId) {
            // 获取两个节点的层级
            const startLevel = this.getNodeLevel(startNodeId);
            const endLevel = this.getNodeLevel(endNodeId);
            
            // 使用较深的层级作为限制层级（较大的数字）
            const maxLevel = Math.max(startLevel, endLevel);
            
            // 获取所有在限制层级及以上的节点
            const validNodes = [];
            const traverse = (nodes, level = 0) => {
                for (const node of nodes) {
                    // 跳过依赖节点分组和依赖节点
                    if (!node.isRefGroup && !node.isRefNode) {
                        // 只包含目标层级及以上的节点
                        if (level <= maxLevel) {
                            validNodes.push(node.id);
                        }
                    }
                    // 继续遍历子节点
                    if (node.children && node.children.length > 0) {
                        traverse(node.children, level + 1);
                    }
                }
            };
            traverse(this.treeData);
            
            // 在有效节点中找到范围
            const startIndex = validNodes.indexOf(startNodeId);
            const endIndex = validNodes.indexOf(endNodeId);
            
            if (startIndex === -1 || endIndex === -1) {
                return [];
            }
            
            const minIndex = Math.min(startIndex, endIndex);
            const maxIndex = Math.max(startIndex, endIndex);
            
            return validNodes.slice(minIndex, maxIndex + 1);
        },
        
        // 切换节点选中状态
        toggleNodeSelection(nodeId) {
            const index = this.selectedNodes.indexOf(nodeId);
            if (index > -1) {
                this.selectedNodes.splice(index, 1);
            } else {
                this.selectedNodes.push(nodeId);
            }
        },
        
        // 清空选择
        clearSelection() {
            this.selectedNodes = [];
            this.lastSelectedNode = null;
        },
        
        // 检查节点是否被选中
        isNodeSelected(nodeId) {
            return this.selectedNodes.includes(nodeId);
        },
        
        // 获取节点的层级
        getNodeLevel(nodeId) {
            const findNodeLevel = (nodes, targetId, level = 0) => {
                for (const node of nodes) {
                    if (node.id === targetId) {
                        return level;
                    }
                    if (node.children && node.children.length > 0) {
                        const childLevel = findNodeLevel(node.children, targetId, level + 1);
                        if (childLevel !== -1) {
                            return childLevel;
                        }
                    }
                }
                return -1;
            };
            
            return findNodeLevel(this.treeData, nodeId);
        },
        
        // 检查节点是否为顶层节点（不是子树节点）
        isTopLevelNode(nodeId) {
            return this.getNodeLevel(nodeId) === 0;
        },
        
        // 加载项目设置
        loadProjectSettings() {
            // 设置初始化标志，防止触发保存
            this.isInitializingSettings = true;
            
            if (!this.project || !this.project.settings) {
                // 如果没有设置，使用默认值
                this.defaultImageFormat = 'png';
                this.defaultImageScale = 1.0;
                // 初始化完成，允许后续保存
                this.$nextTick(() => {
                    this.isInitializingSettings = false;
                });
                return;
            }
            
            try {
                const settings = typeof this.project.settings === 'string' 
                    ? JSON.parse(this.project.settings) 
                    : this.project.settings;
                
                if (settings.imageFormat) {
                    this.defaultImageFormat = settings.imageFormat;
                }
                if (settings.imageScale !== undefined) {
                    this.defaultImageScale = settings.imageScale;
                }
            } catch (error) {
                console.error('解析项目设置失败:', error);
                // 解析失败时使用默认值
                this.defaultImageFormat = 'png';
                this.defaultImageScale = 1.0;
            }
            
            // 初始化完成，允许后续保存
            this.$nextTick(() => {
                this.isInitializingSettings = false;
            });
        },
        
        // 保存项目设置到数据库
        saveProjectSettings() {
            if (!this.project || !this.project.id) {
                return;
            }
            
            // 防抖处理，避免频繁保存
            if (this.settingsSaveTimer) {
                clearTimeout(this.settingsSaveTimer);
            }
            
            this.settingsSaveTimer = setTimeout(() => {
                const settings = {
                    imageFormat: this.defaultImageFormat,
                    imageScale: this.defaultImageScale
                };
                
                axios.put(`/figma/project/${this.project.id}/settings`, settings)
                    .then(() => {
                        // 静默保存，不显示提示消息
                        console.log('项目设置已保存');
                    })
                    .catch(error => {
                        console.error('保存项目设置失败:', error);
                        // 保存失败时不显示错误提示，避免打扰用户
                    });
            }, 500); // 500ms 防抖延迟
        },
        
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
        
        // 返回项目列表页面
        backToProjectList() {
            window.location.href = '/projects';
        },
        
        // 显示重置确认对话框
        showResetConfirmDialog(mode = 'full') {
            // 关闭重置模式菜单
            this.resetModeMenuVisible = false;
            
            let title, message, resetMethod;
            
            if (mode === 'smart') {
                title = '智能重置节点修改';
                message = '此操作将基于当前Figma节点树结构，智能重置根节点及其子树的修改数据。此操作不可撤销，确定要继续吗？';
                resetMethod = this.resetProjectNodeSettings;
            } else {
                title = '完全重置节点修改';
                message = '此操作将清空当前项目的所有节点修改数据，包括自定义名称、锚点设置、资源模式等配置。此操作不可撤销，确定要继续吗？';
                resetMethod = this.resetProjectModifications;
            }
            
            this.$confirm(
                message, 
                title, 
                {
                    confirmButtonText: '确定重置',
                    cancelButtonText: '取消',
                    type: 'warning',
                    dangerouslyUseHTMLString: false,
                    showClose: true,
                    closeOnClickModal: false,
                    closeOnPressEscape: true,
                    beforeClose: (action, instance, done) => {
                        if (action === 'confirm') {
                            instance.confirmButtonLoading = true;
                            instance.confirmButtonText = '重置中...';
                            resetMethod.call(this).finally(() => {
                                instance.confirmButtonLoading = false;
                                done();
                            });
                        } else {
                            done();
                        }
                    }
                }
            ).catch(() => {
                // 用户取消操作
            });
        },
        
        // 重置项目的所有节点修改
        async resetProjectModifications() {
            if (!this.project || !this.project.id) {
                this.$message.error('项目信息不完整');
                return;
            }
            
            try {
                const response = await axios.delete(`/figma/project/${this.project.id}/modifications`, {
                    headers: {
                        'X-CSRF-Token': window.initialData.csrfToken
                    }
                });
                
                if (response.data.success) {
                    this.$message.success('节点修改数据已重置');
                    
                    // 清空本地的节点修改数据
                    this.nodeModifys = {};
                    
                    // 重新加载项目数据以刷新界面
                    await this.loadProjectData(true);
                } else {
                    this.$message.error(response.data.message || '重置失败');
                }
            } catch (error) {
                console.error('重置节点修改失败:', error);
                if (error.response && error.response.data && error.response.data.message) {
                    this.$message.error(error.response.data.message);
                } else {
                    this.$message.error('重置失败，请稍后重试');
                }
            }
        },
        
        // 智能重置项目根节点及其子树的修改（基于Figma API）
        async resetProjectNodeSettings() {
            if (!this.project || !this.project.id) {
                this.$message.error('项目信息不完整');
                return;
            }
            
            try {
                const response = await axios.delete(`/figma/project/${this.project.id}/settings`, {
                    headers: {
                        'X-CSRF-Token': window.initialData.csrfToken
                    }
                });
                
                if (response.data.success) {
                    this.$message.success(`节点修改数据已智能重置，共重置 ${response.data.deleted_count} 个节点`);
                    
                    // 获取被重置的节点ID列表
                    const resetNodeIds = response.data.reset_node_ids || [];
                    
                    // 只清除被重置节点的本地修改数据
                    resetNodeIds.forEach(resetNodeId => {
                        delete this.nodeModifys[resetNodeId];
                    });
                    
                    // 只更新被重置节点的树形显示
                    this.updateResetNodesDisplay(resetNodeIds);
                    
                    // 如果当前选中的节点被重置了，更新属性面板
                    if (this.currentNode && resetNodeIds.includes(this.currentNode.id)) {
                        this.updatePropertyFormFromNode(this.currentNode);
                    }
                } else {
                    this.$message.error(response.data.message || '智能重置失败');
                }
            } catch (error) {
                console.error('智能重置节点修改失败:', error);
                if (error.response && error.response.data && error.response.data.error) {
                    this.$message.error(error.response.data.error);
                } else {
                    this.$message.error('智能重置失败，请稍后重试');
                }
            }
        },
        
        // 显示子树重置确认对话框
        showResetSubtreeConfirmDialog(nodeId) {
            // 关闭右键菜单
            this.layerMenuVisible = false;
            this.contextMenuSource = null;
            
            // 获取节点信息
            const node = this.nodes.find(n => n.id === nodeId);
            // 获取节点的修改信息
            const nodeModify = this.nodeModifys[nodeId];
            // 确定显示名称：优先使用修改的名称，然后是原始名称，最后是节点ID
            const nodeName = (nodeModify?.rename) || (node?.name) || nodeId;
            
            this.$confirm(
                `此操作将重置节点"${nodeName}"及其所有子节点的修改数据，包括自定义名称、锚点设置、资源模式等配置。此操作不可撤销，确定要继续吗？`, 
                '重置子树修改', 
                {
                    confirmButtonText: '确定重置',
                    cancelButtonText: '取消',
                    type: 'warning',
                    dangerouslyUseHTMLString: false,
                    showClose: true,
                    closeOnClickModal: false,
                    closeOnPressEscape: true,
                    beforeClose: (action, instance, done) => {
                        if (action === 'confirm') {
                            instance.confirmButtonLoading = true;
                            instance.confirmButtonText = '重置中...';
                            this.resetNodeSubtree(nodeId).finally(() => {
                                instance.confirmButtonLoading = false;
                                done();
                            });
                        } else {
                            done();
                        }
                    }
                }
            ).catch(() => {
                // 用户取消操作
            });
        },
        
        // 重置指定节点及其子树的修改
        async resetNodeSubtree(nodeId) {
            if (!this.project || !this.project.id) {
                this.$message.error('项目信息不完整');
                return;
            }
            
            try {
                const response = await axios.delete(`/figma/project/${this.project.id}/node/${nodeId}/settings?subtree=true`, {
                    headers: {
                        'X-CSRF-Token': window.initialData.csrfToken
                    }
                });
                
                if (response.data.message) {
                    this.$message.success(response.data.message);
                    
                    // 获取被重置的节点ID列表
                    const resetNodeIds = response.data.reset_node_ids || [];
                    
                    // 只清除被重置节点的本地修改数据
                    resetNodeIds.forEach(resetNodeId => {
                        delete this.nodeModifys[resetNodeId];
                    });
                    
                    // 只更新被重置节点的树形显示
                    this.updateResetNodesDisplay(resetNodeIds);
                    
                    // 如果当前选中的节点被重置了，更新属性面板
                    if (this.currentNode && resetNodeIds.includes(this.currentNode.id)) {
                        this.updatePropertyFormFromNode(this.currentNode);
                    }
                } else {
                    this.$message.error('重置子树失败');
                }
            } catch (error) {
                console.error('重置子树修改失败:', error);
                if (error.response && error.response.data && error.response.data.error) {
                    this.$message.error(error.response.data.error);
                } else {
                    this.$message.error('重置子树失败，请稍后重试');
                }
            }
        },
        
        // 显示重置模式菜单
        showResetModeMenu(event) {
            event.preventDefault();
            event.stopPropagation();
            
            this.resetModeMenuVisible = true;
            this.resetModeMenuStyle = {
                top: `${event.clientY}px`,
                left: `${event.clientX}px`,
                position: 'fixed'
            };
        },
        
        // 更新被重置节点的显示
        updateResetNodesDisplay(resetNodeIds) {
            if (!resetNodeIds || resetNodeIds.length === 0) {
                console.log('没有需要更新的节点');
                return;
            }
            
            console.log('更新被重置节点的显示:', resetNodeIds);
            
            // 清除被重置节点的修改信息缓存和节点对象的modifys属性
            resetNodeIds.forEach(nodeId => {
                // 清除本地修改缓存
                delete this.nodeModifys[nodeId];
                
                // 清除节点对象的modifys属性
                const node = this.nodes.find(n => n.id === nodeId);
                if (node) {
                    node.modifys = null;
                    console.log(`清除节点 ${nodeId} 的修改信息`);
                }
                
                // 清除树形数据中的modifys属性
                this.clearTreeNodeModifys(nodeId);
            });
            
            // 更新每个被重置节点的树形显示
            resetNodeIds.forEach(nodeId => {
                this.updateTreeNodeLabel(nodeId);
            });
            
            // 如果当前选中的节点被重置了，重置属性面板的显示状态
            if (this.currentNode && resetNodeIds.includes(this.currentNode.id)) {
                // 清除当前节点对象的modifys属性
                this.currentNode.modifys = null;
                
                // 重置自定义设置状态
                this.showCustomSettings = false;
                this.showLayoutSettings = false;
                this.showExportSettings = false;
                
                // 重置属性表单为默认值
                this.propertyForm = {
                    img_ext: 'png',
                    components: [],
                    rename: '',
                    ignore: false,
                    res_mode: 'attach',
                    img_name: '',
                    img_id: '',
                    horizontal: 'CENTER',
                    vertical: "CENTER",
                    parent_id: ''
                };
                
                // 强制更新当前节点对象，触发响应式更新
                this.currentNode = {...this.currentNode};
                
                console.log('当前选中节点被重置，已更新属性面板显示');
            }
            
            // 强制更新Vue组件，确保所有变化都被正确渲染
            this.$forceUpdate();
            
            console.log(`成功更新 ${resetNodeIds.length} 个节点的显示`);
        },
        
        // 清除树形数据中指定节点的修改信息
        clearTreeNodeModifys(nodeId, parentNodes = this.treeData) {
            for (let i = 0; i < parentNodes.length; i++) {
                if (parentNodes[i].id === nodeId) {
                    // 找到节点，清除修改信息
                    parentNodes[i].modifys = null;
                    
                    // 恢复原始名称作为显示标签
                    const originalName = parentNodes[i].name || parentNodes[i].id;
                    parentNodes[i].label = originalName;
                    
                    console.log(`清除树形节点 ${nodeId} 的修改信息，恢复原始名称: ${originalName}`);
                    return true;
                }
                
                // 递归检查子节点
                if (parentNodes[i].children && parentNodes[i].children.length) {
                    if (this.clearTreeNodeModifys(nodeId, parentNodes[i].children)) {
                        return true;
                    }
                }
            }
            
            return false;
        },
        
        // 加载相同分组下的所有项目
        loadSiblingProjects() {
            if (!this.project) return;
            
            this.loadingSiblingProjects = true;
            
            // 构建请求参数：如果有分组名称，使用分组名称；否则使用file_key
            const params = {};
            if (this.project.group_name) {
                params.group_name = this.project.group_name;
            } else {
                params.file_key = this.project.file_key;
            }
            
            axios.get('/figma/projects-by-group', { params })
            .then(response => {
                if (response.data.success) {
                    this.siblingProjects = response.data.data;
                    console.log('Loaded sibling projects:', this.siblingProjects);
                    console.log('Current project ID:', this.currentProjectId);
                    console.log('Sibling projects count:', this.siblingProjects.length);
                    console.log('Group query params:', params);
                    
                    // 如果只有一个项目（当前项目），给用户提示
                    if (this.siblingProjects.length <= 1) {
                        console.log('Only one project found, no switching needed');
                    }
                } else {
                    this.$message.error('获取项目列表失败');
                }
            })
            .catch(error => {
                console.error('获取同组项目失败:', error);
                this.$message.error('获取项目列表失败');
            })
            .finally(() => {
                this.loadingSiblingProjects = false;
            });
        },
        
        // 切换项目
        switchProject(projectId) {
            console.log('switchProject called:', {
                projectId,
                projectIdType: typeof projectId,
                currentProjectId: this.currentProjectId,
                currentProjectIdType: typeof this.currentProjectId,
                shouldSwitch: projectId && projectId != this.currentProjectId
            });
            
            if (!projectId) {
                console.log('Invalid projectId');
                // 重置选中项目为当前项目
                this.selectedProjectId = this.currentProjectId;
                return;
            }
            
            // 使用 != 而不是 !== 来处理数字和字符串的比较
            if (projectId != this.currentProjectId) {
                console.log('Switching to project:', projectId);
                // 显示切换提示
                this.$message({
                    message: '正在切换项目...',
                    type: 'info',
                    duration: 1000
                });
                // 跳转到新项目的dashboard
                // 注意：这里不更新 currentProjectId，让页面跳转后自然更新
                window.location.href = `/dashboard?project=${projectId}`;
            } else {
                console.log('Selected same project - no switching needed');
                // 重置选中项目为当前项目（因为用户选择了相同项目）
                this.selectedProjectId = this.currentProjectId;
                // 可选：给用户一个提示
                this.$message({
                    message: '当前已经是该项目',
                    type: 'info',
                    duration: 1500
                });
            }
        },
        
        // 格式化日期（用于下拉选项显示）
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
        
        // 显示创建项目对话框
        showCreateProjectDialog() {
            // 重置表单
            this.createProjectForm = {
                name: '',
                group_name: '',
                link: ''
            };
            
            // 从当前项目列表中获取默认组名
            this.loadDefaultGroupNameFromSiblingProjects();
            
            this.createProjectDialogVisible = true;
            // 关闭下拉框
            this.$nextTick(() => {
                // 手动关闭下拉框
                const selectComponent = this.$el.querySelector('.project-select .el-select');
                if (selectComponent && selectComponent.__vue__) {
                    selectComponent.__vue__.blur();
                }
            });
        },
        
        // 从当前项目列表中加载默认组名
        loadDefaultGroupNameFromSiblingProjects() {
            // 从 siblingProjects 中获取任意一个有组名的项目的组名
            if (this.siblingProjects && this.siblingProjects.length > 0) {
                // 查找第一个有组名的项目
                for (let i = 0; i < this.siblingProjects.length; i++) {
                    const project = this.siblingProjects[i];
                    if (project.group_name && project.group_name.trim() !== '') {
                        this.createProjectForm.group_name = project.group_name.trim();
                        return; // 找到第一个就返回
                    }
                }
            }
        },
        
        // 创建项目
        createProject() {
            this.$refs.createProjectForm.validate((valid) => {
                if (valid) {
                    this.creatingProject = true;
                    
                    // 解析Figma链接
                    this.parseFigmaLinkAndCreate();
                } else {
                    console.log('表单验证失败');
                    return false;
                }
            });
        },
        
        // 解析Figma链接并创建项目
        parseFigmaLinkAndCreate() {
            const link = this.createProjectForm.link;
            
            // 解析Figma链接（支持 file 和 design 两种格式）
            const figmaUrlPattern = /figma\.com\/(?:file|design)\/([a-zA-Z0-9]+)\/[^?]*(?:\?.*node-id=([^&]+))?/;
            const match = link.match(figmaUrlPattern);
            
            if (!match) {
                this.$message.error('无效的Figma链接格式');
                this.creatingProject = false;
                return;
            }
            
            const fileKey = match[1];
            let nodeId = match[2] || '0:1'; // 默认根节点
            
            // 处理URL编码的node-id
            if (nodeId.includes('%3A')) {
                nodeId = decodeURIComponent(nodeId);
            }
            
            // 直接创建项目，组名已经在打开对话框或输入链接时加载了
            // 构建查询参数
            const params = new URLSearchParams({
                name: this.createProjectForm.name,
                figma_url: link
            });
            if (this.createProjectForm.group_name) {
                params.append('group_name', this.createProjectForm.group_name);
            }
            
            fetch(`/figma/node/${fileKey}/${nodeId}?${params.toString()}`, {
                method: 'GET',
                headers: {
                    'X-CSRF-Token': window.initialData?.csrfToken || ''
                }
            })
            .then(response => {
                // 检查响应状态
                if (!response.ok) {
                    return response.text().then(text => {
                        let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
                        try {
                            const errorData = JSON.parse(text);
                            if (errorData.error) {
                                errorMessage = errorData.error;
                            }
                        } catch (e) {
                            // 如果不是JSON，使用原始文本
                            if (text) {
                                errorMessage = text.substring(0, 200);
                            }
                        }
                        throw new Error(errorMessage);
                    });
                }
                
                // 检查内容类型
                const contentType = response.headers.get('content-type');
                if (!contentType || !contentType.includes('application/json')) {
                    return response.text().then(text => {
                        throw new Error('服务器返回的不是JSON格式: ' + text.substring(0, 200));
                    });
                }
                
                return response.json();
            })
            .then(data => {
                if (data.error) {
                    throw new Error(data.error);
                }
                
                // 后端已支持 group_name 参数，直接使用返回的数据
                this.$message.success('项目创建成功！');
                this.createProjectDialogVisible = false;
                this.creatingProject = false;
                
                // 切换到新创建的项目
                const projectId = data.project ? data.project.id : data.id;
                if (projectId) {
                    this.$message({
                        message: '正在切换到新项目...',
                        type: 'info',
                        duration: 1000
                    });
                    setTimeout(() => {
                        window.location.href = `/dashboard?project=${projectId}`;
                    }, 500);
                }
            })
            .catch(error => {
                console.error('创建项目失败:', error);
                this.$message.error('创建项目失败: ' + error.message);
                this.creatingProject = false;
            });
        },
        
        // 更新项目分组名称
        updateProjectGroupName(projectId, groupName) {
            const formData = new FormData();
            formData.append('name', this.createProjectForm.name);
            formData.append('group_name', groupName);
            formData.append('figma_url', this.createProjectForm.link);
            
            return fetch(`/api/projects/${projectId}`, {
                method: 'POST',
                headers: {
                    'X-CSRF-Token': window.initialData?.csrfToken || ''
                },
                body: formData
            })
            .then(response => response.json())
            .then(data => {
                if (data.error) {
                    throw new Error(data.error);
                }
                return { project: data.project };
            });
        },
        
        // 复制项目 Key 信息 (file_key:root_node_id)
        copyProjectKeyInfo(fileKey, rootNodeId) {
            if (!fileKey || !rootNodeId) return;
            
            const textToCopy = `${fileKey}:${rootNodeId}`;
            
            // 使用 Clipboard API 复制
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(textToCopy).then(() => {
                    this.$message.success('已复制到剪贴板: ' + textToCopy);
                }).catch(err => {
                    console.error('复制失败:', err);
                    this.fallbackCopyProjectKeyInfo(textToCopy);
                });
            } else {
                // 降级方案：使用传统方法
                this.fallbackCopyProjectKeyInfo(textToCopy);
            }
        },
        
        // 降级复制方法
        fallbackCopyProjectKeyInfo(text) {
            const textArea = document.createElement('textarea');
            textArea.value = text;
            textArea.style.position = 'fixed';
            textArea.style.left = '-999999px';
            textArea.style.top = '-999999px';
            document.body.appendChild(textArea);
            textArea.focus();
            textArea.select();
            
            try {
                const successful = document.execCommand('copy');
                if (successful) {
                    this.$message.success('已复制到剪贴板: ' + text);
                } else {
                    this.$message.error('复制失败，请手动复制');
                }
            } catch (err) {
                console.error('复制失败:', err);
                this.$message.error('复制失败，请手动复制');
            }
            
            document.body.removeChild(textArea);
        },
        
        // 清除项目图片缓存
        clearProjectImageCache() {
            if (!this.project || !this.project.id) return;
            
            this.$confirm('确定要清除所有图片缓存吗？', '提示', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                type: 'warning'
            }).then(() => {
                this.loading = true;
                axios.post(`/figma/project/${this.project.id}/clear-cache`)
                    .then(() => {
                        this.$message.success('缓存清除成功');
                        // 清空预览图片列表
                        this.previewImages = [];
                        // 如果有当前节点，重新加载它
                        if (this.currentNode) {
                            // 强制刷新图片，添加时间戳
                            const timestamp = new Date().getTime();
                            this.addPreviewImage({
                                ...this.currentNode,
                                _timestamp: timestamp // 添加时间戳确保不使用缓存
                            });
                        }
                        this.loading = false;
                    })
                    .catch(error => {
                        console.error('清除缓存失败:', error);
                        this.$message.error('清除缓存失败');
                        this.loading = false;
                    });
            }).catch(() => {
                // 用户取消操作
            });
        },
        
        // 下载优化后的JSON数据
        downloadOptimizedJson(level = null) {
            if (!this.project || !this.project.file_key || !this.project.root_node_id) {
                this.$message.warning('项目信息不完整，无法下载');
                return;
            }
            
            // 从初始数据获取MCP token
            const mcpToken = window.initialData?.mcpToken;
            if (!mcpToken) {
                this.$message.error('未找到MCP Token，请先在个人资料中生成MCP Token');
                return;
            }
            
            // 如果传入的是事件对象，则忽略它；如果是有效的数字，则使用；否则使用当前选择的level
            let downloadLevel;
            if (level !== null && typeof level === 'number' && !isNaN(level)) {
                // 传入的是有效的数字
                downloadLevel = level;
            } else if (level !== null && typeof level === 'object' && level.type) {
                // 传入的是事件对象，忽略它
                downloadLevel = this.downloadLevel || 1;
            } else {
                // 其他情况，使用当前选择的level
                downloadLevel = this.downloadLevel || 1;
            }
            
            this.performDownload(mcpToken, downloadLevel);
        },
        
        // 选择下载级别并下载
        selectLevelAndDownload(level) {
            this.downloadLevel = level;
            this.downloadOptimizedJson(level);
            this.downloadMenuVisible = false;
        },
        
        // 选择下载级别并复制链接
        selectLevelAndCopyLink(level) {
            this.downloadLevel = level;
            this.copyDownloadLink();
        },
        
        // 执行下载
        performDownload(mcpToken, level = null) {
            const fileKey = encodeURIComponent(this.project.file_key);
            const rootNodeId = encodeURIComponent(this.project.root_node_id);
            const downloadLevel = level !== null ? level : (this.downloadLevel || 1);
            const downloadUrl = `/api/${mcpToken}/optimized_nodes?file_key=${fileKey}&root_node_id=${rootNodeId}&level=${downloadLevel}`;
            
            // 显示加载状态
            const loading = this.$loading({
                lock: true,
                text: '正在生成优化数据...',
                spinner: 'el-icon-loading',
                background: 'rgba(0, 0, 0, 0.7)'
            });
            
            // 发起下载请求
            fetch(downloadUrl)
                .then(response => {
                    loading.close();
                    
                    if (!response.ok) {
                        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
                    }
                    
                    return response.json();
                })
                .then(data => {
                    // 现在API直接返回优化数据，不包装在响应对象中
                    // 创建下载链接 - 保存纯优化数据，与复制链接访问的格式一致
                    const jsonStr = JSON.stringify(data, null, 2);
                    const blob = new Blob([jsonStr], { type: 'application/json' });
                    const url = URL.createObjectURL(blob);
                    
                    // 创建临时下载链接
                    const a = document.createElement('a');
                    a.href = url;
                    a.download = `${this.project.name || 'figma-project'}_optimized.json`;
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);
                    
                    // 释放URL对象
                    URL.revokeObjectURL(url);
                    
                    this.$message.success('JSON数据下载成功');
                })
                .catch(error => {
                    loading.close();
                    console.error('下载失败:', error);
                    
                    let errorMessage = '下载失败';
                    if (error.message.includes('401') || error.message.includes('Unauthorized')) {
                        errorMessage = 'MCP Token无效，请在个人资料中重新生成MCP Token';
                    } else if (error.message.includes('404')) {
                        errorMessage = 'API接口不存在，请检查服务器配置';
                    } else if (error.message) {
                        errorMessage = error.message;
                    }
                    
                    this.$message.error(errorMessage);
                });
        },
        
        // 处理下载按钮左键点击（显示菜单）
        handleDownloadButtonClick(event) {
            if (!this.apiDownloadUrl) {
                this.$message.warning('需要MCP Token才能下载');
                return;
            }
            
            event.preventDefault();
            event.stopPropagation();
            
            this.showDownloadMenu(event);
        },
        
        // 处理下载按钮右键菜单
        handleDownloadButtonContextMenu(event) {
            if (!this.apiDownloadUrl) {
                this.$message.warning('需要MCP Token才能下载');
                return;
            }
            
            event.preventDefault();
            event.stopPropagation();
            
            this.showDownloadMenu(event);
        },
        
        // 显示下载菜单（统一处理左键和右键）
        showDownloadMenu(event) {
            // 先显示菜单以获取其高度
            this.downloadMenuVisible = true;
            
            // 使用nextTick确保DOM已更新
            this.$nextTick(() => {
                const menu = document.querySelector('.download-context-menu');
                if (!menu) return;
                
                const menuHeight = menu.offsetHeight || 200; // 预估高度200px（增加了级别选择器）
                const menuWidth = menu.offsetWidth || 200;
                const viewportHeight = window.innerHeight;
                const viewportWidth = window.innerWidth;
                
                // 计算菜单位置
                let top = event.clientY;
                let left = event.clientX;
                
                // 检查底部空间，如果不够则向上显示
                if (top + menuHeight > viewportHeight) {
                    top = event.clientY - menuHeight;
                }
                
                // 检查右侧空间，如果不够则向左显示
                if (left + menuWidth > viewportWidth) {
                    left = event.clientX - menuWidth;
                }
                
                // 确保不超出屏幕边界
                top = Math.max(5, Math.min(top, viewportHeight - menuHeight - 5));
                left = Math.max(5, Math.min(left, viewportWidth - menuWidth - 5));
                
                // 设置菜单位置
                this.downloadMenuStyle = {
                    top: top + 'px',
                    left: left + 'px'
                };
            });
            
            // 点击其他地方隐藏菜单
            const closeMenu = () => {
                this.downloadMenuVisible = false;
                document.removeEventListener('click', closeMenu);
            };
            
            setTimeout(() => {
                document.addEventListener('click', closeMenu);
            }, 100);
        },
        
        // 拷贝下载链接
        copyDownloadLink() {
            if (!this.apiDownloadUrl) {
                this.$message.warning('链接不可用');
                return;
            }
            
            // apiDownloadUrl已经是完整的URL（包含域名），直接使用
            const fullUrl = this.apiDownloadUrl;
            
            // 使用现代Clipboard API
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(fullUrl)
                    .then(() => {
                        this.$message.success('链接已复制到剪贴板');
                        this.downloadMenuVisible = false;
                    })
                    .catch(err => {
                        console.error('复制失败:', err);
                        // 降级到传统方法
                        this.fallbackCopyToClipboard(fullUrl);
                    });
            } else {
                // 降级到传统方法
                this.fallbackCopyToClipboard(fullUrl);
            }
        },
        
        // 降级的复制到剪贴板方法（兼容旧浏览器）
        fallbackCopyToClipboard(text) {
            const textArea = document.createElement('textarea');
            textArea.value = text;
            textArea.style.position = 'fixed';
            textArea.style.top = '0';
            textArea.style.left = '0';
            textArea.style.width = '2em';
            textArea.style.height = '2em';
            textArea.style.padding = '0';
            textArea.style.border = 'none';
            textArea.style.outline = 'none';
            textArea.style.boxShadow = 'none';
            textArea.style.background = 'transparent';
            
            document.body.appendChild(textArea);
            textArea.focus();
            textArea.select();
            
            try {
                const successful = document.execCommand('copy');
                if (successful) {
                    this.$message.success('链接已复制到剪贴板');
                    this.downloadMenuVisible = false;
                } else {
                    this.$message.error('复制失败，请手动复制');
                }
            } catch (err) {
                console.error('复制失败:', err);
                this.$message.error('复制失败，请手动复制');
            }
            
            document.body.removeChild(textArea);
        },
        
    // 选择根节点
    selectRootNode() {
        // 找到根节点（最顶层的节点）
        const rootNodes = this.nodes.filter(node => !node.parent_id);
        if (rootNodes.length > 0) {
            const rootNode = rootNodes[0];
            console.log('自动选择根节点:', rootNode.id);
            
            // 将根节点添加到展开的节点列表中
            this.expandedKeys = [rootNode.id];
            
            // 选择根节点
            this.handleNodeSelect(rootNode.id);
            
            // 在树中选中根节点
            this.$nextTick(() => {
                if (this.$refs.nodeTree) {
                    this.$refs.nodeTree.setCurrentKey(rootNode.id);
                    
                    // 展开根节点
                    if (rootNode.children && rootNode.children.length > 0) {
                        this.$refs.nodeTree.store.nodesMap[rootNode.id].expanded = true;
                    }
                }
            });
        }
    },
    
    // 加载项目数据
    loadProjectData(forceRefresh = false) {
        if (!this.project || !this.project.id) return;
        
        this.loading = true;
        
        // 保存当前展开的节点和选中的节点
        const currentExpandedKeys = [...this.expandedKeys];
        const currentSelectedNodeId = this.currentNode?.id;
        
        // 清空预览图片列表
        this.previewImages = [];
        this.rootNodeBounds = null;
        
        // 重置缩放级别
        this.zoomLevel = 1;
        
        // 1. 从Figma API获取节点树，如果强制刷新则使用刷新API
        const nodeTreeUrl = forceRefresh ? 
            `/figma/project/${this.project.id}/node-tree/refresh` : 
            `/figma/project/${this.project.id}/node-tree`;
            
        axios.get(nodeTreeUrl)
            .catch(error => {
                // 如果是404错误，说明项目不存在或接口不存在
                if (error.response && error.response.status === 404) {
                    console.error('获取节点树失败（404）:', error);
                    // 404错误是严重错误，需要抛出
                    throw error;
                }
                // 其他错误继续抛出
                throw error;
            })
            .then(response => {
                if (!response) {
                    throw new Error('获取节点树失败：响应为空');
                }
                this.nodes = response.data.nodes || [];
                
                // 2. 获取项目的节点修改信息（如果失败，使用空对象）
                return axios.get(`/figma/project/${this.project.id}/node-modifys`)
                    .catch(error => {
                        // 如果是404错误，说明项目不存在或没有修改信息，使用空对象
                        if (error.response && error.response.status === 404) {
                            console.warn('获取节点修改信息失败（404），使用空对象');
                            return { data: { modifys: {} } };
                        }
                        // 其他错误继续抛出
                        throw error;
                    });
            })
            .then(response => {
                this.nodeModifys = response.data.modifys || {};
                
                // 3. 获取依赖节点列表（如果失败，使用空数组）
                return axios.get(`/figma/project/${this.project.id}/ref-nodes`)
                    .catch(error => {
                        // 如果是404错误，说明接口不存在或项目不存在，使用空数组
                        if (error.response && error.response.status === 404) {
                            console.warn('获取依赖节点列表失败（404），使用空数组');
                            return { data: { data: { ref_nodes: [] } } };
                        }
                        // 其他错误继续抛出
                        throw error;
                    });
            })
            .then(response => {
                this.refNodes = response.data.data.ref_nodes || [];
                
                // 4. 获取依赖节点的详细信息
                return this.loadRefNodeDetails();
            })
            .then(() => {
                // 5. 将修改信息叠加到节点树
                this.applyModifysToNodes();
                
                // 6. 初始化树形数据
                this.initTreeData();
                
                // 5. 预加载所有节点的占位区域
                this.preloadNodeAreas();
                
                // 6. 恢复展开的节点
                this.$nextTick(() => {
                    // 确保树已经渲染完成后再设置展开状态
                    // 如果有之前保存的展开状态，则使用它，否则保持当前状态（包含根节点）
                    if (currentExpandedKeys && currentExpandedKeys.length > 0) {
                        this.expandedKeys = currentExpandedKeys;
                    }
                    
                    // 确保根节点始终被展开
                    const rootNodes = this.nodes.filter(node => !node.parent_id);
                    if (rootNodes.length > 0) {
                        const rootNodeId = rootNodes[0].id;
                        if (!this.expandedKeys.includes(rootNodeId)) {
                            this.expandedKeys.push(rootNodeId);
                        }
                        
                        // 强制展开根节点
                        this.$nextTick(() => {
                            if (this.$refs.nodeTree && this.$refs.nodeTree.store.nodesMap[rootNodeId]) {
                                this.$refs.nodeTree.store.nodesMap[rootNodeId].expanded = true;
                            }
                        });
                    }
                    
                    // 确保依赖节点分组和所有子节点始终被展开
                    this.ensureRefNodesExpanded();
                });
                
                // 7. 如果当前有选中节点，重新加载它，否则选择根节点
                if (currentSelectedNodeId) {
                    const nodeId = currentSelectedNodeId;
                    this.currentNode = this.nodes.find(node => node.id === nodeId);
                    if (this.currentNode) {
                        this.loadNodeSettings(this.currentNode);
                        this.addPreviewImage(this.currentNode);
                        
                        // 确保当前节点仍然被选中
                        this.$nextTick(() => {
                            if (this.$refs.nodeTree) {
                                this.$refs.nodeTree.setCurrentKey(nodeId);
                            }
                        });
                    } else {
                        // 如果找不到当前节点，选择根节点
                        this.selectRootNode();
                    }
                } else {
                    // 如果没有当前选中的节点，选择根节点
                    this.selectRootNode();
                }
                
                // 如果是强制刷新，显示成功提示
                if (forceRefresh) {
                    this.$message.success('节点树数据已刷新');
                }
                
                // 重新加载项目设置（项目数据可能已更新）
                this.loadProjectSettings();
            })
            .catch(error => {
                console.error('加载项目数据失败:', error);
                // 如果是404错误，提供更详细的错误信息
                if (error.response && error.response.status === 404) {
                    const errorMsg = error.response.data?.error || '项目或接口不存在';
                    console.error('404错误详情:', {
                        url: error.config?.url,
                        status: error.response.status,
                        error: errorMsg
                    });
                    this.$message.error(`加载项目数据失败: ${errorMsg}`);
                } else {
                    this.$message.error(error.response?.data?.error || '加载项目数据失败');
                }
            })
            .finally(() => {
                this.loading = false;
            });
    },
        
        // 将修改信息叠加到节点树
        applyModifysToNodes() {
            // 遍历所有节点，将修改信息叠加到节点上
            for (const node of this.nodes) {
                const nodeId = node.id;
                // 在应用修改信息之前，先保存原始名称
                if (!node.originalName) {
                    node.originalName = node.name || node.id;
                }
                if (this.nodeModifys[nodeId]) {
                    // 将修改信息合并到节点对象
                    Object.assign(node, { modifys: this.nodeModifys[nodeId] });
                }
            }
        },
        
        // 加载依赖节点的详细信息
        async loadRefNodeDetails() {
            this.refNodeDetails = {};
            
            if (!this.refNodes || this.refNodes.length === 0) {
                return Promise.resolve();
            }
            
            try {
                // 调用新的接口获取依赖节点的详细信息
                const nodeIdsParam = this.refNodes.join(',');
                const response = await axios.get(`/figma/project/${this.project.id}/ref-nodes/details?node_ids=${encodeURIComponent(nodeIdsParam)}`)
                    .catch(error => {
                        // 如果是404错误，说明接口不存在或项目不存在，静默处理
                        if (error.response && error.response.status === 404) {
                            console.warn('依赖节点详情接口返回404，可能接口不存在或项目已删除');
                            return { data: { success: false, error: '接口不存在' } };
                        }
                        // 其他错误继续抛出
                        throw error;
                    });
                
                if (response && response.data && response.data.success) {
                    const nodeDetails = response.data.data.node_details;
                    
                    // 处理每个依赖节点的详细信息
                    for (const nodeId of this.refNodes) {
                        if (nodeDetails && nodeDetails[nodeId]) {
                            const nodeDetail = nodeDetails[nodeId];
                            
                            if (nodeDetail.rootNode && nodeDetail.treeData) {
                                // 如果有完整的树结构数据
                                this.refNodeDetails[nodeId] = {
                                    ...nodeDetail.rootNode,
                                    isRefNode: true,
                                    refNodeTreeData: nodeDetail.treeData // 保存完整的树结构数据
                                };
                            } else {
                                // 如果只有基本信息
                                this.refNodeDetails[nodeId] = {
                                    ...nodeDetail,
                                    isRefNode: true
                                };
                            }
                        } else {
                            // 如果没有返回数据，创建占位符
                            this.refNodeDetails[nodeId] = {
                                id: nodeId,
                                name: `依赖节点 ${nodeId}`,
                                type: 'REFERENCE',
                                isRefNode: true,
                                notFound: true
                            };
                        }
                    }
                } else {
                    console.warn('获取依赖节点详细信息失败:', response?.data?.error || '未知错误');
                    // 为所有依赖节点创建错误占位符
                    for (const nodeId of this.refNodes) {
                        this.refNodeDetails[nodeId] = {
                            id: nodeId,
                            name: `依赖节点 ${nodeId}`,
                            type: 'REFERENCE',
                            isRefNode: true,
                            error: true
                        };
                    }
                }
            } catch (error) {
                // 如果是404错误，静默处理，不显示错误消息
                if (error.response && error.response.status === 404) {
                    console.warn('获取依赖节点详细信息失败（404）:', error);
                } else {
                    console.error('获取依赖节点详细信息失败:', error);
                }
                // 为所有依赖节点创建错误占位符
                for (const nodeId of this.refNodes) {
                    this.refNodeDetails[nodeId] = {
                        id: nodeId,
                        name: `依赖节点 ${nodeId}`,
                        type: 'REFERENCE',
                        isRefNode: true,
                        error: true
                    };
                }
            }
            
            return Promise.resolve();
        },
        
    // 初始化树形数据
    initTreeData() {
        if (!this.nodes.length) return;
        
        // 找到根节点
        const rootNodes = this.nodes.filter(node => !node.parent_id);
        const mainTree = this.buildTree(rootNodes);
        
        // 构建依赖节点树
        const refTree = this.buildRefNodeTree();
        
        // 合并主树和依赖节点树
        this.treeData = [...mainTree];
        
        // 如果有依赖节点，添加依赖节点分组
        if (refTree.length > 0) {
            this.treeData.push({
                id: '__ref_nodes__',
                label: '依赖节点',
                type: 'REF_GROUP',
                children: refTree,
                isRefGroup: true
            });
        }
        
        // 确保根节点始终在展开的节点列表中
        if (rootNodes.length > 0) {
            const rootNodeId = rootNodes[0].id;
            if (!this.expandedKeys.includes(rootNodeId)) {
                this.expandedKeys.push(rootNodeId);
            }
        }
        
        // 默认展开依赖节点分组（但不展开第一级子节点）
        if (refTree.length > 0 && !this.expandedKeys.includes('__ref_nodes__')) {
            this.expandedKeys.push('__ref_nodes__');
            // 刷新后不再自动展开第一级子节点
        }
        
        },
        
        // 辅助方法：根据ID查找节点数据
        findNodeById(nodeId) {
            const findInTree = (nodes, id) => {
                if (!nodes || !Array.isArray(nodes)) return null;
                
                for (let node of nodes) {
                    if (node.id === id) return node;
                    if (node.children && node.children.length > 0) {
                        const found = findInTree(node.children, id);
                        if (found) return found;
                    }
                }
                return null;
            };
            
            return findInTree(this.treeData, nodeId);
        },
        
        // 确保依赖节点分组被展开（但不展开第一级子节点）
        ensureRefNodesExpanded() {
            // 检查是否有依赖节点
            const refTree = this.buildRefNodeTree();
            if (refTree.length > 0) {
                // 确保依赖节点分组被展开
                if (!this.expandedKeys.includes('__ref_nodes__')) {
                    this.expandedKeys.push('__ref_nodes__');
                }
                
                // 刷新后不再自动展开第一级子节点
                
                // 强制在树组件中展开依赖节点分组
                this.$nextTick(() => {
                    if (this.$refs.nodeTree && this.$refs.nodeTree.store.nodesMap['__ref_nodes__']) {
                        this.$refs.nodeTree.store.nodesMap['__ref_nodes__'].expanded = true;
                    }
                });
            }
        },
        
        // 只展开依赖节点树的第一级（不递归）
        expandRefNodeTreeFirstLevel(refTree) {
            if (!refTree || refTree.length === 0) return;
            
            // 只展开第一级节点，不递归展开子节点
            for (const node of refTree) {
                if (node.id && !this.expandedKeys.includes(node.id)) {
                    this.expandedKeys.push(node.id);
                }
                // 不递归展开子节点，只展开第一级
            }
        },
        
        // 递归展开依赖节点树（保留用于其他场景，但默认不使用）
        expandRefNodeTree(refTree) {
            if (!refTree || refTree.length === 0) return;
            
            for (const node of refTree) {
                if (node.id && !this.expandedKeys.includes(node.id)) {
                    this.expandedKeys.push(node.id);
                }
                // 递归展开子节点
                if (node.children && node.children.length > 0) {
                    this.expandRefNodeTree(node.children);
                }
            }
        },
        
        // 检查节点子树是否展开
        isNodeSubtreeExpanded(nodeId) {
            if (!nodeId || !this.$refs.nodeTree) return false;
            
            // 查找节点
            const findNode = (nodes) => {
                for (const node of nodes) {
                    if (node.id === nodeId) {
                        return node;
                    }
                    if (node.children && node.children.length > 0) {
                        const found = findNode(node.children);
                        if (found) return found;
                    }
                }
                return null;
            };
            
            const targetNode = findNode(this.treeData);
            if (!targetNode || !targetNode.children || targetNode.children.length === 0) {
                return false;
            }
            
            // 检查所有子节点是否都在展开列表中
            const checkChildrenExpanded = (node) => {
                if (!node.children || node.children.length === 0) return true;
                
                for (const child of node.children) {
                    if (!this.expandedKeys.includes(child.id)) {
                        return false;
                    }
                    if (!checkChildrenExpanded(child)) {
                        return false;
                    }
                }
                return true;
            };
            
            return checkChildrenExpanded(targetNode);
        },
        
        // 切换节点子树的展开/收起
        toggleNodeSubtree(nodeId) {
            if (!nodeId) return;
            
            // 查找节点
            const findNode = (nodes) => {
                for (const node of nodes) {
                    if (node.id === nodeId) {
                        return node;
                    }
                    if (node.children && node.children.length > 0) {
                        const found = findNode(node.children);
                        if (found) return found;
                    }
                }
                return null;
            };
            
            const targetNode = findNode(this.treeData);
            if (!targetNode) {
                this.$message.warning('未找到节点');
                return;
            }
            
            // 递归收集子树的所有节点ID
            const collectSubtreeIds = (node) => {
                const ids = [];
                if (node.children && node.children.length > 0) {
                    for (const child of node.children) {
                        ids.push(child.id);
                        const childIds = collectSubtreeIds(child);
                        ids.push(...childIds);
                    }
                }
                return ids;
            };
            
            const subtreeIds = collectSubtreeIds(targetNode);
            
            if (subtreeIds.length === 0) {
                this.$message.info('该节点没有子节点');
                this.hideLayerMenu();
                return;
            }
            
            // 检查是否已展开（通过检查树组件的实际状态）
            let isExpanded = false;
            if (this.$refs.nodeTree && this.$refs.nodeTree.store) {
                // 检查第一个子节点是否展开
                if (targetNode.children && targetNode.children.length > 0) {
                    const firstChildId = targetNode.children[0].id;
                    const firstChildNode = this.$refs.nodeTree.store.nodesMap[firstChildId];
                    if (firstChildNode) {
                        isExpanded = firstChildNode.expanded || false;
                    }
                }
            }
            
            // 如果无法从树组件获取状态，则使用expandedKeys判断
            if (!isExpanded && subtreeIds.length > 0) {
                isExpanded = this.expandedKeys.includes(subtreeIds[0]);
            }
            
            if (isExpanded) {
                // 收起：从展开列表中移除所有子树节点ID
                this.expandedKeys = this.expandedKeys.filter(id => !subtreeIds.includes(id));
                
                // 强制更新树组件
                this.$nextTick(() => {
                    if (this.$refs.nodeTree && this.$refs.nodeTree.store) {
                        subtreeIds.forEach(id => {
                            const node = this.$refs.nodeTree.store.nodesMap[id];
                            if (node) {
                                node.expanded = false;
                            }
                        });
                        // 强制刷新树组件
                        this.$refs.nodeTree.$forceUpdate();
                    }
                });
                this.$message.success('已收起子树');
            } else {
                // 展开：将子树节点ID添加到展开列表
                subtreeIds.forEach(id => {
                    if (!this.expandedKeys.includes(id)) {
                        this.expandedKeys.push(id);
                    }
                });
                
                // 强制更新树组件
                this.$nextTick(() => {
                    if (this.$refs.nodeTree && this.$refs.nodeTree.store) {
                        subtreeIds.forEach(id => {
                            const node = this.$refs.nodeTree.store.nodesMap[id];
                            if (node) {
                                node.expanded = true;
                            }
                        });
                        // 强制刷新树组件
                        this.$refs.nodeTree.$forceUpdate();
                    }
                });
                this.$message.success('已展开子树');
            }
            
            this.hideLayerMenu();
        },
        
        // 构建依赖节点树
        buildRefNodeTree() {
            if (!this.refNodes || this.refNodes.length === 0) {
                return [];
            }
            
            console.log('构建依赖节点树，依赖节点数量:', this.refNodes.length);
            console.log('依赖节点详细信息:', this.refNodeDetails);
            
            // 收集所有依赖节点的数据，包括子节点
            const allRefNodeData = [];
            for (const nodeId of this.refNodes) {
                const refNodeDetail = this.refNodeDetails[nodeId];
                if (refNodeDetail && refNodeDetail.refNodeTreeData) {
                    // 如果有完整的节点树数据，使用它
                    console.log(`节点 ${nodeId} 有完整树数据，节点数量:`, refNodeDetail.refNodeTreeData.length);
                    // 确保依赖节点数据也保存原始名称
                    refNodeDetail.refNodeTreeData.forEach(node => {
                        if (!node.originalName) {
                            node.originalName = node.name || node.id;
                        }
                    });
                    allRefNodeData.push(...refNodeDetail.refNodeTreeData);
                } else if (refNodeDetail) {
                    // 如果只有单个节点数据，添加它
                    console.log(`节点 ${nodeId} 只有单个节点数据`);
                    // 确保依赖节点数据也保存原始名称
                    if (!refNodeDetail.originalName) {
                        refNodeDetail.originalName = refNodeDetail.name || refNodeDetail.id;
                    }
                    allRefNodeData.push(refNodeDetail);
                }
            }
            
            // 如果没有详细数据，返回基本的依赖节点列表
            if (allRefNodeData.length === 0) {
                return this.refNodes.map(nodeId => ({
                    id: nodeId,
                    label: `依赖节点 ${nodeId}`,
                    type: 'REFERENCE',
                    isRefNode: true,
                    children: []
                }));
            }
            
            // 构建依赖节点的树形结构
            const refRootNodes = [];
            
            for (const nodeId of this.refNodes) {
                // 查找每个依赖节点作为根节点
                const rootNode = allRefNodeData.find(node => node.id === nodeId);
                if (rootNode) {
                    const treeNode = this.buildRefNodeTreeRecursive(rootNode, allRefNodeData);
                    console.log(`构建依赖节点 ${nodeId} 的树结构:`, treeNode);
                    refRootNodes.push(treeNode);
                } else {
                    // 如果没有找到数据，创建占位符
                    console.log(`依赖节点 ${nodeId} 没有找到数据，创建占位符`);
                    refRootNodes.push({
                        id: nodeId,
                        label: `依赖节点 ${nodeId}`,
                        type: 'REFERENCE',
                        isRefNode: true,
                        notFound: true,
                        children: []
                    });
                }
            }
            
            console.log('最终构建的依赖节点树:', refRootNodes);
            return refRootNodes;
        },
        
        // 递归构建依赖节点树
        buildRefNodeTreeRecursive(node, allNodes) {
            const nodeName = node.modifys?.rename || node.modifys?.customName || node.name || node.id;
            // 保存原始名称（优先使用originalName，如果没有则使用name）
            const originalName = node.originalName || node.name || node.id;
            
            // 查找子节点（过滤掉不可见的节点）
            const children = allNodes.filter(n => {
                return n.parent_id === node.id && n.visible !== false;
            });
            
            const childrenTree = children.map(child => this.buildRefNodeTreeRecursive(child, allNodes));
            
            // 为有子节点的依赖节点添加展开键
            if (childrenTree.length > 0) {
                // 确保有子节点的依赖节点能够展开
                // 默认不展开，让用户手动展开
                //console.log(`依赖节点 ${node.id} 有 ${childrenTree.length} 个子节点`);
            }
            
            return {
                id: node.id,
                label: nodeName,
                name: originalName, // 保存原始名称
                type: node.type || 'REFERENCE',
                isRefNode: true, // 所有依赖节点树中的节点都标记为依赖节点
                isRefChild: !this.refNodes.includes(node.id), // 标记是否为依赖节点的子节点（不在依赖节点列表中的节点）
                notFound: node.notFound,
                error: node.error,
                modifys: node.modifys,
                visible: node.visible,
                absoluteRenderBounds: node.absoluteRenderBounds,
                children: childrenTree
            };
        },
        
        // 构建树形结构
        buildTree(nodes) {
        return nodes
            .filter(node => {
                // 过滤掉visible为false的节点
                return node.visible !== false;
            })
            .map(node => {
                const children = this.nodes.filter(n => n.parent_id === node.id);
                
                // 使用自定义名称（如果存在）
                const nodeName = node.modifys?.rename || node.modifys?.customName || node.name || node.id;
                // 保存原始名称（优先使用originalName，如果没有则使用name）
                const originalName = node.originalName || node.name || node.id;
                
                // 如果节点没有absoluteBoundingBox，添加一个默认的
                if (!node.absoluteRenderBounds) {
                    //console.log(`节点 ${node.id} 没有absoluteBoundingBox，添加默认值`);
                    // 使用绝对位置，不依赖父子关系
                    node.absoluteRenderBounds = {
                        x: 0,
                        y: 0,
                        width: 100,  // 默认宽度
                        height: 100  // 默认高度
                    };
                }
                
                return {
                    id: node.id,
                    label: nodeName,
                    name: originalName, // 保存原始名称
                    children: children.length ? this.buildTree(children) : [],
                    // 添加节点类型和修改信息，用于在树中显示不同的图标或样式
                    type: node.type,
                    modifys: node.modifys,
                    visible: node.visible,
                    absoluteRenderBounds: node.absoluteRenderBounds
                };
            });
        },
        
    // 节点选择处理
    handleNodeSelect(nodeId) {
        console.log('选择节点:', nodeId);
        
        // 记录上一次选择的节点ID
        const previousNodeId = this.currentNode ? this.currentNode.id : null;
        
        // 查找新选择的节点（先在主节点中查找，再在依赖节点中查找）
        this.currentNode = this.nodes.find(node => node.id === nodeId);
        
        // 如果在主节点中没找到，尝试在依赖节点中查找
        if (!this.currentNode) {
            this.currentNode = this.findRefNodeById(nodeId);
            if (this.currentNode) {
                console.log('在依赖节点中找到节点:', this.currentNode);
                // 标记这是一个依赖节点
                this.currentNode.isRefNodeSelected = true;
            }
        } else {
            console.log('在主节点中找到节点:', this.currentNode);
        }
        
        // 始终更新选中节点的高层级预览样式，即使是相同节点
        this.updateSelectionOverlay();
        
        // 先清空普通缩略图区域
        const minimapImage = document.querySelector('.minimap-container:not(.filtered-minimap) .minimap-image');
        const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
        
        // 先清空过滤缩略图区域
        const filteredMinimapImage = document.querySelector('.minimap-container.filtered-minimap .minimap-image');
        const filteredMinimapContainer = document.querySelector('.minimap-container.filtered-minimap');
        
        if (minimapImage) {
            minimapImage.style.display = 'none';
        }
        
        if (filteredMinimapImage) {
            filteredMinimapImage.style.display = 'none';
        }
        
        // 如果是依赖节点，立即加载依赖节点预览（无论previewImages是否为空）
        if (this.currentNode && this.currentNode.isRefNodeSelected) {
            console.log('检测到依赖节点，调用 loadRefNodePreview()');
            this.loadRefNodePreview();
            // 依赖节点不需要主节点的预览图加载逻辑，但需要执行其他必要逻辑
            this.loadNodeSettings(this.currentNode);
            this.updateNodeJsonDisplay();
            // 依赖节点也需要显示缩略图，等待预览图加载完成后自动更新（在 loadRefNodePreview 中处理）
            // 隐藏层级菜单
            this.layerMenuVisible = false;
            this.contextMenuSource = null;
            return;
        }
        
        // 如果没有选择有效节点或者没有预览图，隐藏缩略图容器
        if (!this.currentNode || this.previewImages.length === 0) {
            console.log('没有选择有效节点或没有预览图，隐藏缩略图');
            if (minimapContainer) {
                minimapContainer.style.display = 'none';
            }
            if (filteredMinimapContainer) {
                filteredMinimapContainer.style.display = 'none';
            }
            
            // 重置过滤图片URL
            this.filteredNodeImage = null;
            
            if (!this.currentNode) {
                return; // 如果没有有效节点，直接返回
            }
        }
        
        this.loadNodeSettings(this.currentNode);
        
        // 更新节点JSON显示
        this.updateNodeJsonDisplay();
        
        // 隐藏层级菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
        
        // 检查是否是相同节点（上一次选择的节点）
        if (previousNodeId === nodeId) {
            console.log('选择了相同的节点，保留现有预览图');
            
            // 如果是相同节点，不清理预览图，只更新缩略图
            if (minimapImage && this.previewImages.length > 0) {
                minimapImage.onload = function() {
                    minimapImage.style.display = 'block';
                    // 确保图片清晰显示
                    minimapImage.style.imageRendering = 'crisp-edges';
                };
                minimapImage.src = this.getCurrentNodeImage(1.0); // 使用标准清晰度
                
                // 确保缩略图容器显示（如果有预览图）
                if (minimapContainer) {
                    minimapContainer.style.display = 'block';
                }
            }
            
            // 更新过滤缩略图，仅在已展开状态下更新
            if (this.isFilteredPreviewExpanded) {
                console.log('切换节点，过滤预览已展开，更新过滤缩略图');
                this.updateFilteredMinimap(1.0);
            } else {
                console.log('切换节点，过滤预览未展开，不更新过滤缩略图');
            }
        } else {
            // 如果是不同节点，按原逻辑处理
            console.log('选择了不同的节点，重新加载预览图');
            
            // 如果已存在该节点的预览图，先从列表中移除
            const existingIndex = this.previewImages.findIndex(img => img.nodeId === nodeId);
            if (existingIndex !== -1) {
                // 从预览列表中移除，以便重新加载
                this.previewImages.splice(existingIndex, 1);
            }
            
            // 添加到预览图片列表（重新请求），使用标准清晰度
            this.addPreviewImage(this.currentNode, false, 1.0);
            
            // 确保预览图加载完成后显示缩略图
            this.$nextTick(() => {
                // 创建一个新的Image对象来预加载图片
                const imageUrl = `/figma/image/${this.project.id}/${this.currentNode.id}?scale=1.0`;
                const preloadImg = new Image();
                preloadImg.onload = () => {
                    console.log('预览图片加载完成:', imageUrl);
                    
                    // 更新普通缩略图
                    const minimapImage = document.querySelector('.minimap-container:not(.filtered-minimap) .minimap-image');
                    const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
                    
                    if (minimapImage && minimapContainer && this.currentNode) {
                        minimapImage.src = imageUrl;
                        minimapImage.style.display = 'block';
                        minimapContainer.style.display = 'block';
                    }
                    
                    // 更新过滤缩略图
                    // 仅在过滤预览展开时更新过滤缩略图
                    if (this.isFilteredPreviewExpanded) {
                        console.log('加载完成，过滤预览已展开，更新过滤缩略图');
                        this.updateFilteredMinimap(1.0);
                    } else {
                        console.log('加载完成，过滤预览未展开，不更新过滤缩略图');
                    }
                };
                preloadImg.src = imageUrl;
            });
        }
        
        // 隐藏层级菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
    },
    
    // 更新过滤缩略图
    updateFilteredMinimap(scale = 1.0) {
        console.log('开始更新过滤缩略图');
        
        // 如果过滤预览未展开，不进行更新
        if (!this.isFilteredPreviewExpanded) {
            console.log('过滤预览未展开，跳过更新过滤缩略图');
            return;
        }
        
        // 获取过滤后的图片URL
        const filteredImageUrl = this.getFilteredNodeImage(scale);
        console.log('过滤缩略图URL:', filteredImageUrl);
        
        // 立即设置filteredNodeImage，确保v-if条件可以通过
        this.filteredNodeImage = filteredImageUrl;
        
        // 获取过滤缩略图容器和图片元素
        const filteredMinimapContainer = document.querySelector('.minimap-container.filtered-minimap');
        const filteredMinimapImage = filteredMinimapContainer ? filteredMinimapContainer.querySelector('.minimap-image') : null;
        
        console.log('过滤缩略图容器:', filteredMinimapContainer);
        console.log('过滤缩略图图片元素:', filteredMinimapImage);

        if (!filteredMinimapContainer) {
            console.error('找不到过滤缩略图容器');
            return;
        }
        if (!filteredMinimapImage) {
            console.error('找不到过滤缩略图图片元素');
            return;
        }

        
        // 立即显示容器，确保它可见
        filteredMinimapContainer.style.display = 'block';
        
        // 创建一个新的Image对象来预加载过滤后的图片
        const preloadFilteredImg = new Image();
        preloadFilteredImg.onload = () => {
            console.log('过滤缩略图加载完成:', filteredImageUrl);
            
            // 确保图片已加载完成后再显示
            this.$nextTick(() => {
                filteredMinimapImage.src = filteredImageUrl;
                filteredMinimapImage.style.display = 'block';
                
                // 确保图片清晰显示
                filteredMinimapImage.style.imageRendering = 'crisp-edges';
                
                console.log('过滤缩略图已显示');
            });
        };
        
        // 处理加载失败的情况
        preloadFilteredImg.onerror = (error) => {
            console.error('过滤缩略图加载失败:', filteredImageUrl, error);
            
            // 显示错误信息而不是隐藏容器
            filteredMinimapContainer.style.display = 'block';
            
            // 添加错误信息
            const errorDiv = document.createElement('div');
            errorDiv.className = 'minimap-error';
            errorDiv.textContent = '过滤图片加载失败';
            errorDiv.style.color = 'red';
            errorDiv.style.padding = '10px';
            errorDiv.style.textAlign = 'center';
            
            // 清除现有内容并添加错误信息
            const contentDiv = filteredMinimapContainer.querySelector('.minimap-content');
            if (contentDiv) {
                // 保留loading图标
                const loadingDiv = contentDiv.querySelector('.minimap-loading');
                contentDiv.innerHTML = '';
                if (loadingDiv) contentDiv.appendChild(loadingDiv);
                contentDiv.appendChild(errorDiv);
            }
            
            // 不重置filteredNodeImage，保持容器可见
            // this.filteredNodeImage = null;
        };
        
        // 开始加载图片
        console.log('开始加载过滤缩略图:', filteredImageUrl);
        preloadFilteredImg.src = filteredImageUrl;
    },
    
    // 切换过滤预览展开/收起状态
    toggleFilteredPreview() {
        console.log('手动切换过滤预览状态');
        this.isFilteredPreviewExpanded = !this.isFilteredPreviewExpanded;
        
        // 如果是展开状态且还没有加载批量图片，则加载
        // 手动展开时允许加载所有图片
        if (this.isFilteredPreviewExpanded && Object.keys(this.filteredNodeImages).length === 0) {
            console.log('手动展开过滤预览，加载所有过滤图片');
            this.loadFilteredNodeImages();
        }
    },
    
    // 切换当前预览展开/收起状态
    toggleCurrentPreview() {
        this.isCurrentPreviewExpanded = !this.isCurrentPreviewExpanded;
    },
    
    // 加载批量过滤预览图片
    async loadFilteredNodeImages(scale = 1.0) {
        console.log('开始加载批量过滤预览图片');
        
        // 如果正在加载，则不重复加载
        if (this.isLoadingFilteredImages) {
            console.log('正在加载批量过滤预览图片，跳过重复请求');
            return;
        }
        
        // 仅在手动展开过滤预览或切换节点时（且已展开状态）才加载所有图片
        if (!this.isFilteredPreviewExpanded) {
            console.log('过滤预览未展开，不加载批量图片');
            return;
        }
        
        this.isLoadingFilteredImages = true;
        
        try {
            // 获取批量过滤预览图片
            const images = await this.getFilteredNodeImages(scale);
            
            if (Object.keys(images).length > 0) {
                this.filteredNodeImages = images;
                
                // 转换为列表格式，包含节点信息
                this.filteredImagesList = Object.entries(images).map(([nodeId, imagePath]) => {
                    const node = this.nodes.find(n => n.id === nodeId);
                    return {
                        nodeId: nodeId,
                        imagePath: imagePath,
                        nodeName: node ? (node.name || nodeId) : nodeId,
                        nodeType: node ? node.type : 'UNKNOWN'
                    };
                });
                
                // 重置当前索引
                this.currentFilteredImageIndex = 0;
                
                console.log(`成功加载 ${this.filteredImagesList.length} 个过滤预览图片`);
            } else {
                console.log('没有获取到过滤预览图片');
                this.filteredNodeImages = {};
                this.filteredImagesList = [];
                this.currentFilteredImageIndex = 0;
            }
        } catch (error) {
            console.error('加载批量过滤预览图片失败:', error);
            this.filteredNodeImages = {};
            this.filteredImagesList = [];
            this.currentFilteredImageIndex = 0;
        } finally {
            this.isLoadingFilteredImages = false;
        }
    },
    
    // 切换到上一张过滤预览图片
    prevFilteredImage() {
        if (this.filteredImagesList.length > 0) {
            this.currentFilteredImageIndex = (this.currentFilteredImageIndex - 1 + this.filteredImagesList.length) % this.filteredImagesList.length;
        }
    },
    
    // 切换到下一张过滤预览图片
    nextFilteredImage() {
        if (this.filteredImagesList.length > 0) {
            this.currentFilteredImageIndex = (this.currentFilteredImageIndex + 1) % this.filteredImagesList.length;
        }
    },
    
    // 跳转到指定的过滤预览图片
    goToFilteredImage(index) {
        if (index >= 0 && index < this.filteredImagesList.length) {
            this.currentFilteredImageIndex = index;
        }
    },
        
    // 处理树节点点击事件
    handleTreeNodeClick(nodeId, event) {
        // 清除任何文本选择（解决多选时出现文本选中效果的问题）
        if (window.getSelection) {
            window.getSelection().removeAllRanges();
        } else if (document.selection) {
            document.selection.empty();
        }
        
        // 实时检查键盘状态（解决远程连接时键盘事件丢失的问题）
        const isShiftPressed = event ? event.shiftKey : this.isShiftPressed;
        const isCtrlPressed = event ? (event.ctrlKey || event.metaKey) : this.isCtrlPressed;
        
        console.log('树节点被点击:', nodeId, 'Shift:', isShiftPressed, 'Ctrl:', isCtrlPressed, '节点层级:', this.getNodeLevel(nodeId));
        
        if (isShiftPressed && this.lastSelectedNode) {
            // Shift + 点击：范围选择（在目标层级及以上进行）
            const rangeNodes = this.getNodesBetween(this.lastSelectedNode, nodeId);
            console.log('范围选择节点:', rangeNodes, '起始层级:', this.getNodeLevel(this.lastSelectedNode), '结束层级:', this.getNodeLevel(nodeId));
            
            // 清空当前选择，然后选择范围内的所有节点
            this.selectedNodes = [...rangeNodes];
            this.lastSelectedNode = nodeId;
            
            // 设置当前节点为范围的最后一个节点
            this.handleNodeSelect(nodeId);
        } else if (isCtrlPressed) {
            // Ctrl + 点击：切换选择（将节点加入或移出选择列表）
            const nodeLevel = this.getNodeLevel(nodeId);
            
            // 检查当前选择中是否有更深层级的节点
            let canSelect = true;
            let deepestSelectedLevel = -1;
            
            for (const selectedId of this.selectedNodes) {
                const selectedLevel = this.getNodeLevel(selectedId);
                if (selectedLevel > deepestSelectedLevel) {
                    deepestSelectedLevel = selectedLevel;
                }
            }
            
            // 如果当前节点层级比已选择的最深层级还深，且当前节点未被选中，不允许选择
            if (deepestSelectedLevel !== -1 && nodeLevel > deepestSelectedLevel && !this.isNodeSelected(nodeId)) {
                canSelect = false;
            }
            
            if (canSelect) {
                // 切换节点选择状态
                this.toggleNodeSelection(nodeId);
                this.lastSelectedNode = nodeId;
                
                // 总是设置为当前节点（无论是选中还是取消选中）
                this.handleNodeSelect(nodeId);
                
                // 如果取消选择后没有选中节点了，清空lastSelectedNode
                if (this.selectedNodes.length === 0) {
                    this.lastSelectedNode = null;
                }
            } else {
                // 如果层级不符合要求，给出提示
                console.log('不能选择比当前选择更深层级的节点');
                this.$message.info('不能选择比当前选择更深层级的节点');
            }
        } else {
            // 普通点击：单选（保持原有逻辑）
            // 检查是否是重复点击相同节点
            if (this.currentNode && this.currentNode.id === nodeId) {
                console.log('忽略重复点击相同节点');
                return;
            }
            
            // 清空之前的选择，选择当前节点
            this.selectedNodes = [nodeId];
            this.lastSelectedNode = nodeId;
            this.handleNodeSelect(nodeId);
        }
        
        console.log('当前选中节点:', this.selectedNodes);
    },
    
    // 处理全局点击事件，用于关闭层级菜单
    handleGlobalClick(event) {
        // 如果点击的不是层级菜单内的元素，则关闭层级菜单
        if (this.layerMenuVisible) {
            const layerMenu = document.querySelector('.layer-menu');
            // 检查点击的元素是否在层级菜单内，或者是否有.layer-menu-item类
            const isClickInMenu = layerMenu && (
                layerMenu.contains(event.target) || 
                event.target.closest('.layer-menu-item') || 
                event.target.closest('.layer-menu')
            );
            
            if (!isClickInMenu) {
                console.log('点击在层级菜单外部，关闭菜单');
                this.layerMenuVisible = false;
                this.contextMenuSource = null;
            }
        }
        
        // 如果点击的不是重置模式菜单内的元素，则关闭重置模式菜单
        if (this.resetModeMenuVisible) {
            const resetModeMenu = document.querySelector('.reset-mode-menu');
            const isClickInResetMenu = resetModeMenu && (
                resetModeMenu.contains(event.target) || 
                event.target.closest('.reset-mode-menu')
            );
            
            if (!isClickInResetMenu) {
                console.log('点击在重置模式菜单外部，关闭菜单');
                this.resetModeMenuVisible = false;
            }
        }
    },
    
    // 处理鼠标按下事件，开始拖拽
    handleMouseDown(event) {
        // 只处理左键点击
        if (event.button !== 0) return;
        
        // 如果鼠标在节点上，不启动拖拽
        if (this.isOverNode) return;
        
        // 记录点击开始时间，用于区分点击和拖拽
        this.clickStartTime = Date.now();
        
        // 记录鼠标按下的位置，用于拖拽
        this.dragStartX = event.clientX;
        this.dragStartY = event.clientY;
        
        // 标记可能开始拖拽，但还不确定是点击还是拖拽
        this.dragPossible = true;
        
        // 不立即设置isDragging为true，等待鼠标移动一定距离后再确认为拖拽
    },
    
    // 处理鼠标进入节点事件
    handleNodeMouseEnter(event) {
        // 设置鼠标悬停在节点上的标志
        this.isOverNode = true;
        
        // 更改光标样式为指针
        if (event.target) {
            event.target.style.cursor = 'pointer';
        }
        
        // 如果有可能开始拖拽，取消它
        this.dragPossible = false;
    },
    
    // 处理鼠标离开节点事件
    handleNodeMouseLeave(event) {
        // 清除鼠标悬停在节点上的标志
        this.isOverNode = false;
        
        // 恢复光标样式
        if (event.target) {
            event.target.style.cursor = '';
        }
    },
    
    // 处理预览区域的鼠标移动事件，用于拖拽预览区域
    handlePreviewMouseMove(event) {
        // 如果鼠标在节点上，不执行拖拽
        if (this.isOverNode) return;
        
        // 如果鼠标左键被按下且可能是拖拽
        if (event.buttons === 1 && this.dragPossible) {
            // 计算鼠标移动的距离
            const deltaX = event.clientX - this.dragStartX;
            const deltaY = event.clientY - this.dragStartY;
            
            // 如果移动距离超过阈值，确认为拖拽
            if (Math.abs(deltaX) > 5 || Math.abs(deltaY) > 5) {
                this.isDragging = true;
                
                // 更新拖拽开始位置
                this.dragStartX = event.clientX;
                this.dragStartY = event.clientY;
                
                // 更新预览偏移量
                this.previewOffset.x += deltaX;
                this.previewOffset.y += deltaY;
                
                // 更新所有预览图片的位置
                this.updatePreviewImagesPosition();
                
                // 添加拖拽样式
                if (this.$refs.previewContainer) {
                    this.$refs.previewContainer.style.cursor = 'grabbing';
                }
            }
        }
    },
    
    // 处理预览区域的鼠标松开事件
    handlePreviewMouseUp(event) {
        // 只处理左键释放
        if (event.button !== 0) return;

         // 调用原来的点击处理函数
        this.handlePreviewClick(event);
        
        // 结束拖拽状态
        this.isDragging = false;
        this.dragPossible = false;
        
        // 恢复光标样式，但只有在不在节点上时才设置为grab
        if (this.$refs.previewContainer && !this.isOverNode) {
            this.$refs.previewContainer.style.cursor = 'grab';
        }
    },
    
    // 处理鼠标移动事件（全局）
    handleMouseMove(event) {
        // 如果鼠标在节点上，不执行拖拽
        if (this.isOverNode) return;
        
        // 只有当isDragging为true时才执行拖拽
        if (this.isDragging) {
            // 阻止默认行为
            event.preventDefault();
            
            // 计算鼠标移动的距离
            const deltaX = event.clientX - this.dragStartX;
            const deltaY = event.clientY - this.dragStartY;
            
            // 更新拖拽开始位置
            this.dragStartX = event.clientX;
            this.dragStartY = event.clientY;
            
            // 更新预览偏移量
            this.previewOffset.x += deltaX;
            this.previewOffset.y += deltaY;
            
            // 更新所有预览图片的位置
            this.updatePreviewImagesPosition();
        }
    },
    
    // 处理鼠标松开事件（全局）
    handleMouseUp(event) {
        // 结束拖拽
        this.isDragging = false;
        this.dragPossible = false;
        
        // 恢复光标样式，但只有在不在节点上时才设置为grab
        if (this.$refs.previewContainer && !this.isOverNode) {
            this.$refs.previewContainer.style.cursor = 'grab';
        }
        
        // 移除鼠标移动和松开事件监听
        document.removeEventListener('mousemove', this.handleMouseMove);
        document.removeEventListener('mouseup', this.handleMouseUp);
    },
    
    // 更新所有预览图片的位置（考虑拖拽偏移）
    updatePreviewImagesPosition() {
        // 更新预加载节点的样式
        this.updatePreloadedNodesStyle();
        
        // 更新预览图片的样式
        this.updatePreviewImagesScale();
        
        // 更新依赖节点预览图的位置
        this.updateRefPreviewImagesPosition();
        
        // 更新选中节点的高层级预览样式
        if (this.currentNode) {
            this.updateSelectionOverlay();
        }
    },
    
    // 获取选中节点高层级预览层的样式
    updateSelectionOverlay() {
        if (!this.currentNode || !this.rootNodeBounds) {
            this.selectionOverlayStyle = null;
            return;
        }
        
        let node = null;
        let isRefNode = false;
        
        // 查找当前选中节点（先在主节点中查找，再在依赖节点中查找）
        node = this.nodes.find(n => n.id === this.currentNode.id);
        
        if (!node && this.currentNode.isRefNodeSelected) {
            // 如果是依赖节点，使用当前节点的数据
            node = this.currentNode;
            isRefNode = true;
            console.log('更新依赖节点选中样式:', node);
        }
        
        if (!node || !node.absoluteRenderBounds) {
            this.selectionOverlayStyle = null;
            return;
        }
        
        // 根节点的宽高
        const rootWidth = this.rootNodeBounds.width;
        const rootHeight = this.rootNodeBounds.height;
        
        // 计算相对于根节点的位置
        const relX = node.absoluteRenderBounds.x - this.rootNodeBounds.x;
        const relY = node.absoluteRenderBounds.y - this.rootNodeBounds.y;
        
        // 计算相对位置的百分比（相对于根节点尺寸）
        const percentX = relX / rootWidth * 100;
        const percentY = relY / rootHeight * 100;
        
        console.log(`选中节点 ${node.id} 相对位置:`, relX, relY, '百分比:', percentX.toFixed(2) + '%', percentY.toFixed(2) + '%');
        
    // 使用绝对定位，确保只使用绝对位置，只减去根节点的位置
    // 计算节点在容器中的位置：容器中心 + 相对于根节点中心的偏移量 + 拖拽偏移量
    this.selectionOverlayStyle = {
        position: 'absolute',
        left: `calc(50% + ${(relX - rootWidth/2) * this.zoomLevel + this.previewOffset.x}px)`,
        top: `calc(50% + ${(relY - rootHeight/2) * this.zoomLevel + this.previewOffset.y}px)`,
        width: `${node.absoluteRenderBounds.width * this.zoomLevel}px`,
        height: `${node.absoluteRenderBounds.height * this.zoomLevel}px`,
        transformOrigin: '0 0', // 从左上角开始变换
        pointerEvents: 'none' // 不拦截鼠标事件
    };
    },
    
    // 获取当前节点的显示名称
    getCurrentNodeDisplayName() {
        if (!this.currentNode) return '';
        
        // 优先使用自定义名称，然后是节点名称，最后是节点ID
        if (this.currentNode.modifys?.rename || this.currentNode.modifys?.customName) {
            return this.currentNode.modifys.rename || this.currentNode.modifys.customName;
        }
        if (this.currentNode.name) {
            return this.currentNode.name;
        }
        return this.currentNode.id || '未命名节点';
    },
    
    // 格式化尺寸显示（保留1位小数）
    formatSize(value) {
        if (value === null || value === undefined) return '0';
        return Number(value).toFixed(1);
    },
    
    // 格式化位置显示（保留1位小数）
    formatPosition(value) {
        if (value === null || value === undefined) return '0';
        return Number(value).toFixed(1);
    },
    
    // 处理鼠标滚轮事件，用于缩放预览
    handleMouseWheel(event) {
        // 阻止默认滚动行为
        event.preventDefault();
        
        // 根据滚轮方向决定是放大还是缩小
        // deltaY < 0 表示向上滚动（放大），deltaY > 0 表示向下滚动（缩小）
        
        // 计算缩放因子 - 根据滚轮滚动的幅度调整缩放步长
        const scaleFactor = 0.05; // 基础缩放步长
        const delta = -Math.sign(event.deltaY) * scaleFactor; // 根据滚轮方向确定缩放方向
        
        // 计算新的缩放级别
        let newZoomLevel = this.zoomLevel * (1 + delta);
        
        // 限制缩放范围
        newZoomLevel = Math.max(this.minZoom, Math.min(this.maxZoom, newZoomLevel));
        
        // 应用新的缩放级别
        if (newZoomLevel !== this.zoomLevel) {
            this.zoomLevel = newZoomLevel;
            this.updatePreviewImagesScale();
        }
    },
        
    // 处理缩略图右键菜单
    handleMinimapContextMenu(event, type) {
        event.preventDefault();
        event.stopPropagation();
        
        // 设置菜单来源和位置
        this.contextMenuSource = 'minimap';
        this.contextMenuMinimapType = type; // 'current' 或 'filtered'
        this.layerMenuVisible = true;
        this.layerMenuStyle = {
            top: `${event.clientY}px`,
            left: `${event.clientX}px`,
            position: 'fixed'
        };
        
        // 根据类型设置相关数据
        if (type === 'current' && this.currentNode) {
            this.contextMenuNodeId = this.currentNode.id;
        } else if (type === 'filtered' && this.filteredImagesList.length > 0) {
            this.contextMenuNodeId = this.filteredImagesList[this.currentFilteredImageIndex].nodeId;
        }
    },
    
    // 处理右键菜单
    handleContextMenu(event) {
        // 阻止默认右键菜单
        event.preventDefault();
        
        // 获取点击位置相对于预览容器的坐标
        if (!this.previewImages.length || !this.$refs.previewContainer || !this.rootNodeBounds) {
            console.log('没有预览图片、预览容器或根节点边界不存在');
            return;
        }
        
        const containerRect = this.$refs.previewContainer.getBoundingClientRect();
        const clickX = event.clientX - containerRect.left;
        const clickY = event.clientY - containerRect.top;
        
        // 容器中心点
        const containerCenterX = containerRect.width / 2;
        const containerCenterY = containerRect.height / 2;
        
        // 根节点的宽高
        const rootWidth = this.rootNodeBounds.width;
        const rootHeight = this.rootNodeBounds.height;
        
        // 计算点击位置相对于根节点左上角的坐标
        const rootLeftX = containerCenterX - (rootWidth/2) * this.zoomLevel;
        const rootTopY = containerCenterY - (rootHeight/2) * this.zoomLevel;
        
        // 计算点击位置相对于根节点左上角的位置，并除以缩放比例得到实际坐标
        const relativeX = (clickX - rootLeftX) / this.zoomLevel;
        const relativeY = (clickY - rootTopY) / this.zoomLevel;
        
        // 转换为绝对坐标
        const absoluteX = relativeX + this.rootNodeBounds.x;
        const absoluteY = relativeY + this.rootNodeBounds.y;
        
        // 找出所有包含点击位置的节点
        const matchingNodes = [];
        for (const image of this.previewImages) {
            const node = this.nodes.find(n => n.id === image.nodeId);
            if (!node || !node.absoluteRenderBounds) {
                continue;
            }
            
            // 判断点击位置是否在节点范围内
            if (
                absoluteX >= node.absoluteRenderBounds.x && 
                absoluteX <= node.absoluteRenderBounds.x + node.absoluteRenderBounds.width && 
                absoluteY >= node.absoluteRenderBounds.y && 
                absoluteY <= node.absoluteRenderBounds.y + node.absoluteRenderBounds.height
            ) {
                matchingNodes.push(node);
            }
        }
        
        // 只有找到匹配的节点时才显示菜单
        if (matchingNodes.length > 0) {
            // 显示层级菜单
            this.layerMenuVisible = true;
            this.contextMenuSource = 'preview'; // 标记菜单来源为预览图
            this.layerMenuStyle = {
                top: `${event.clientY}px`,
                left: `${event.clientX}px`,
                position: 'fixed' // 使用固定定位，相对于视口
            };
            
            // 过滤预览图片列表，只保留匹配的节点
            this.filteredPreviewImages = this.previewImages.filter(img => 
                matchingNodes.some(node => node.id === img.nodeId)
            );
            
            // 设置右键点击的节点ID为第一个匹配节点
            if (this.filteredPreviewImages.length > 0) {
                this.contextMenuNodeId = this.filteredPreviewImages[0].nodeId;
            }
            
            console.log('右键菜单中的节点数量:', this.filteredPreviewImages.length);
            this.filteredPreviewImages.forEach(img => {
                console.log('节点:', img.nodeId, img.nodeName);
            });
        } else {
            // 如果没有找到匹配节点，显示所有预览图片作为备选
            console.log('未找到匹配节点，显示所有预览图片');
            this.layerMenuVisible = true;
            this.contextMenuSource = 'preview'; // 标记菜单来源为预览图
            this.layerMenuStyle = {
                top: `${event.clientY}px`,
                left: `${event.clientX}px`,
                position: 'fixed' // 使用固定定位，相对于视口
            };
            this.filteredPreviewImages = [...this.previewImages];
            
            // 清空右键点击的节点ID
            this.contextMenuNodeId = null;
        }
        
        // 不再设置拖拽状态，因为现在使用左键拖拽
    },
    
    // 获取节点的忽略状态
    getNodeIgnoreState(nodeId) {
        // 从nodeModifys中获取节点的ignore状态
        if (this.nodeModifys[nodeId] && this.nodeModifys[nodeId].ignore !== undefined) {
            return this.nodeModifys[nodeId].ignore;
        }
        return false; // 默认为未忽略
    },
    
    // 判断节点的modifys是否为空或只包含默认值
    hasNodeModifys(modifys) {
        if (!modifys || typeof modifys !== 'object') {
            return false;
        }
        
        // 如果modifys为空对象，返回false
        const keys = Object.keys(modifys);
        if (keys.length === 0) {
            return false;
        }
        
        // 检查是否只包含默认值
        const defaultValues = {
            ignore: false,
            components: [],
            horizontal: 'CENTER',
            vertical: "CENTER",
            img_ext: 'png',
            img_name: '',
            rename: '',
            res_mode: 'attach'
        };
        
        // 检查所有字段是否都是默认值
        for (const key of keys) {
            const value = modifys[key];
            const defaultValue = defaultValues[key];
            
            // 如果字段不在默认值列表中，检查是否有实际值
            if (defaultValue === undefined) {
                // 对于 img_id 和 parent_id 等字段，如果有值则视为有修改
                if (key === 'img_id' || key === 'parent_id') {
                    if (value && value.toString().trim() !== '') {
                        return true;
                    }
                } else if (key === 'nodeType') {
                    // nodeType 如果不是 'default'，视为有修改
                    if (value && value !== 'default') {
                        return true;
                    }
                } else {
                    // 其他未知字段，如果有值则视为有修改
                    if (value !== null && value !== undefined && value !== '') {
                        return true;
                    }
                }
                continue;
            }
            
            // 比较值是否与默认值不同
            if (key === 'components') {
                // 对于数组，检查是否为空数组
                if (Array.isArray(value)) {
                    if (value.length > 0) {
                        return true;
                    }
                } else if (value !== null && value !== undefined) {
                    // 如果不是数组但有值，视为有修改
                    return true;
                }
            } else if (value !== defaultValue) {
                return true;
            }
        }
        
        // 所有字段都是默认值，视为没有修改
        return false;
    },
    
    // 刷新属性面板
    refreshPropertyPanel() {
        if (this.currentNode) {
            // 保存当前属性面板的展开状态
            const currentSectionExpanded = {...this.sectionExpanded};
            
            // 重新加载节点设置
            this.loadNodeSettings(this.currentNode, currentSectionExpanded);
        }
    },
    
    // 切换节点的忽略状态
    toggleNodeIgnore(nodeId) {
        // 确保nodeModifys中有该节点的记录
        if (!this.nodeModifys[nodeId]) {
            this.nodeModifys[nodeId] = {};
        }
        
        // 切换ignore状态
        const currentState = this.getNodeIgnoreState(nodeId);
        this.nodeModifys[nodeId].ignore = !currentState;
        
        // 保存修改
        this.saveNodeModifys();
        
        // 提示用户
        this.$message({
            message: `已${!currentState ? '忽略' : '取消忽略'}节点`,
            type: 'success',
            duration: 1500
        });
        
        // 关闭右键菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
    },
    
    // 显示重命名输入框
    showRenameInput(nodeId) {
        // 获取当前节点名称
        let currentName = '';
        if (this.nodeModifys[nodeId] && this.nodeModifys[nodeId].rename) {
            currentName = this.nodeModifys[nodeId].rename;
        } else {
            // 从节点列表中查找原始名称
            const node = this.nodes.find(n => n.id === nodeId);
            if (node && node.name) {
                currentName = node.name;
            }
        }
        
        // 设置重命名表单
        this.renameForm = {
            nodeId: nodeId,
            newName: currentName
        };
        
        // 显示重命名对话框
        this.renameDialogVisible = true;
        
        // 关闭右键菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
        
        // 下一帧聚焦输入框
        this.$nextTick(() => {
            if (this.$refs.renameInput) {
                this.$refs.renameInput.focus();
            }
        });
    },
    
    // 关闭重命名对话框
    closeRenameDialog() {
        this.renameDialogVisible = false;
        this.renameProcessing = false; // 重置处理标志
        this.renameForm = {
            nodeId: null,
            newName: ''
        };
    },
    
    // 确认重命名
    confirmRename() {
        // 防止重复调用
        if (this.renameProcessing) {
            return;
        }
        
        const { nodeId, newName } = this.renameForm;
        if (!nodeId || !newName.trim()) {
            this.$message.warning('节点名称不能为空');
            return;
        }
        
        // 设置处理标志，防止重复调用
        this.renameProcessing = true;
        
        // 确保nodeModifys中有该节点的记录
        if (!this.nodeModifys[nodeId]) {
            this.nodeModifys[nodeId] = {};
        }
        
        // 设置新名称
        this.nodeModifys[nodeId].rename = newName.trim();
        
        // 保存修改
        this.saveNodeModifys();
        
        // 提示用户
        this.$message({
            message: '节点重命名成功',
            type: 'success',
            duration: 1500
        });
        
        // 关闭对话框
        this.closeRenameDialog();
    },
    
    // 设置节点水平约束
    setNodeHorizontal(nodeId, horizontal) {
        // 确保nodeModifys中有该节点的记录
        if (!this.nodeModifys[nodeId]) {
            this.nodeModifys[nodeId] = {};
        }
        
        // 设置水平约束
        this.nodeModifys[nodeId].horizontal = horizontal;
        
        // 保存修改
        this.saveNodeModifys();
        
        // 提示用户
        this.$message({
            message: `已设置水平约束为: ${horizontal}`,
            type: 'success',
            duration: 1500
        });
        
        // 关闭右键菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
    },
    
    // 设置节点垂直约束
    setNodeVertical(nodeId, vertical) {
        // 确保nodeModifys中有该节点的记录
        if (!this.nodeModifys[nodeId]) {
            this.nodeModifys[nodeId] = {};
        }
        
        // 设置垂直约束
        this.nodeModifys[nodeId].vertical = vertical;
        
        // 保存修改
        this.saveNodeModifys();
        
        // 提示用户
        this.$message({
            message: `已设置垂直约束为: ${vertical}`,
            type: 'success',
            duration: 1500
        });
        
        // 关闭右键菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
    },
    
    // 设置节点资源模式
    setNodeResMode(nodeId, resMode) {
        // 确保nodeModifys中有该节点的记录
        if (!this.nodeModifys[nodeId]) {
            this.nodeModifys[nodeId] = {};
        }
        
        // 设置资源模式
        this.nodeModifys[nodeId].res_mode = resMode;
        
        // 保存修改
        this.saveNodeModifys();
        
        // 提示用户
        this.$message({
            message: `已设置资源模式为: ${resMode}`,
            type: 'success',
            duration: 1500
        });
        
        // 关闭右键菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
    },
    
    // 设置节点组件类型
    setNodeComponent(nodeId, compType) {
        // 确保nodeModifys中有该节点的记录
        if (!this.nodeModifys[nodeId]) {
            this.nodeModifys[nodeId] = {};
        }
        
        // 确保components是数组
        if (!this.nodeModifys[nodeId].components) {
            this.nodeModifys[nodeId].components = [];
        }
        
        // 检查是否已存在该组件类型
        const existingIndex = this.nodeModifys[nodeId].components.indexOf(compType);
        if (existingIndex === -1) {
            // 不存在则添加
            this.nodeModifys[nodeId].components.push(compType);
        } else {
            // 已存在则移除（切换功能）
            this.nodeModifys[nodeId].components.splice(existingIndex, 1);
        }
        
        // 保存修改
        this.saveNodeModifys();
        
        // 提示用户
        this.$message({
            message: existingIndex === -1 ? 
                `已添加组件类型: ${compType}` : 
                `已移除组件类型: ${compType}`,
            type: 'success',
            duration: 1500
        });
        
        // 关闭右键菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
    },
    
    // 批量保存节点修改
    batchSaveNodeModifys(nodeIds) {
        if (!nodeIds || nodeIds.length === 0) {
            console.error('没有要保存的节点');
            return Promise.resolve();
        }
        
        // 批量保存所有节点的修改
        const savePromises = [];
        
        for (const nodeId of nodeIds) {
            const nodeModify = this.nodeModifys[nodeId];
            if (nodeModify) {
                const promise = axios.post(`/figma/project/${this.project.id}/node/${nodeId}/settings`, nodeModify)
                    .then(() => {
                        console.log(`节点 ${nodeId} 修改保存成功`);
                        
                        // 立即更新节点对象的modifys属性
                        const targetNode = this.nodes.find(node => node.id === nodeId);
                        if (targetNode) {
                            targetNode.modifys = { ...nodeModify };
                        }
                        
                        // 更新树形显示
                        this.updateTreeNodeLabel(nodeId);
                        
                        // 如果是当前选中的节点，更新属性面板
                        if (this.currentNode && this.currentNode.id === nodeId) {
                            if (targetNode) {
                                this.currentNode = targetNode;
                            } else {
                                this.currentNode.modifys = { ...nodeModify };
                            }
                            this.updatePropertyFormFromNode(this.currentNode);
                            
                            if (this.hasNodeModifys(this.currentNode.modifys)) {
                                this.showCustomSettings = true;
                                this.showLayoutSettings = true;
                                this.showExportSettings = true;
                            }
                        }
                    })
                    .catch(error => {
                        console.error(`保存节点 ${nodeId} 修改失败:`, error);
                        throw error;
                    });
                savePromises.push(promise);
            }
        }
        
        // 返回Promise，等待所有保存操作完成
        return Promise.all(savePromises)
            .then(() => {
                console.log(`批量保存完成，共保存 ${savePromises.length} 个节点`);
            })
            .catch(error => {
                console.error('批量保存节点修改失败:', error);
                this.$message.error('批量保存节点修改失败');
                throw error;
            });
    },
    
    // 批量操作方法
    // 批量切换节点忽略状态
    batchToggleNodeIgnore() {
        const nodesToProcess = this.selectedNodes.length > 0 ? this.selectedNodes : [this.contextMenuNodeId];
        
        // 检查当前状态（如果所有节点都被忽略，则取消忽略；否则设置忽略）
        const allIgnored = nodesToProcess.every(nodeId => this.nodeModifys[nodeId]?.ignore);
        const newIgnoreState = !allIgnored;
        
        nodesToProcess.forEach(nodeId => {
            if (!this.nodeModifys[nodeId]) {
                this.nodeModifys[nodeId] = {};
            }
            this.nodeModifys[nodeId].ignore = newIgnoreState;
        });
        
        // 使用批量保存方法
        this.batchSaveNodeModifys(nodesToProcess);
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
        
        const action = newIgnoreState ? '忽略' : '启用';
        this.$message.success(`已${action} ${nodesToProcess.length} 个节点`);
    },
    
    // 批量设置节点水平约束
    batchSetNodeHorizontal(horizontal) {
        const nodesToProcess = this.selectedNodes.length > 0 ? this.selectedNodes : [this.contextMenuNodeId];
        
        nodesToProcess.forEach(nodeId => {
            if (!this.nodeModifys[nodeId]) {
                this.nodeModifys[nodeId] = {};
            }
            this.nodeModifys[nodeId].horizontal = horizontal;
        });
        
        // 使用批量保存方法
        this.batchSaveNodeModifys(nodesToProcess);
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
        
        this.$message.success(`已为 ${nodesToProcess.length} 个节点设置水平约束: ${horizontal}`);
    },
    
    // 批量设置节点垂直约束
    batchSetNodeVertical(vertical) {
        const nodesToProcess = this.selectedNodes.length > 0 ? this.selectedNodes : [this.contextMenuNodeId];
        
        nodesToProcess.forEach(nodeId => {
            if (!this.nodeModifys[nodeId]) {
                this.nodeModifys[nodeId] = {};
            }
            this.nodeModifys[nodeId].vertical = vertical;
        });
        
        // 使用批量保存方法
        this.batchSaveNodeModifys(nodesToProcess);
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
        
        this.$message.success(`已为 ${nodesToProcess.length} 个节点设置垂直约束: ${vertical}`);
    },
    
    // 批量设置节点资源模式
    batchSetNodeResMode(resMode) {
        const nodesToProcess = this.selectedNodes.length > 0 ? this.selectedNodes : [this.contextMenuNodeId];
        
        nodesToProcess.forEach(nodeId => {
            if (!this.nodeModifys[nodeId]) {
                this.nodeModifys[nodeId] = {};
            }
            this.nodeModifys[nodeId].res_mode = resMode;
        });
        
        // 使用批量保存方法
        this.batchSaveNodeModifys(nodesToProcess);
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
        
        this.$message.success(`已为 ${nodesToProcess.length} 个节点设置资源模式: ${resMode}`);
    },
    
    // 批量设置节点组件
    batchSetNodeComponent(compType) {
        const nodesToProcess = this.selectedNodes.length > 0 ? this.selectedNodes : [this.contextMenuNodeId];
        
        nodesToProcess.forEach(nodeId => {
            if (!this.nodeModifys[nodeId]) {
                this.nodeModifys[nodeId] = {};
            }
            
            // 初始化组件列表
            if (!this.nodeModifys[nodeId].components) {
                this.nodeModifys[nodeId].components = [];
            }
            
            // 添加组件类型（如果不存在）
            if (!this.nodeModifys[nodeId].components.includes(compType)) {
                this.nodeModifys[nodeId].components.push(compType);
            }
        });
        
        // 使用批量保存方法
        this.batchSaveNodeModifys(nodesToProcess);
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
        
        this.$message.success(`已为 ${nodesToProcess.length} 个节点添加组件: ${compType}`);
    },
    
    // 批量重置节点修改
    batchResetNodeModifications() {
        const nodesToProcess = this.selectedNodes.length > 0 ? this.selectedNodes : [this.contextMenuNodeId];
        
        if (nodesToProcess.length === 1) {
            // 单个节点，调用原有的重置子树方法
            this.showResetSubtreeConfirmDialog(nodesToProcess[0]);
            return;
        }
        
        // 批量重置确认
        this.$confirm(`确定要重置这 ${nodesToProcess.length} 个节点的所有修改吗？`, '批量重置确认', {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning'
        }).then(() => {
            // 重置选中节点的修改
            nodesToProcess.forEach(nodeId => {
                if (this.nodeModifys[nodeId]) {
                    delete this.nodeModifys[nodeId];
                }
            });
            
            this.batchSaveNodeModifys(nodesToProcess);
            this.layerMenuVisible = false;
            this.contextMenuSource = null;
            
            this.$message.success(`已重置 ${nodesToProcess.length} 个节点的修改`);
        }).catch(() => {
            // 用户取消
        });
    },
    
    // 保存节点修改
    saveNodeModifys() {
        // 获取当前选中的节点ID
        const nodeId = this.contextMenuNodeId || (this.currentNode ? this.currentNode.id : null);
        if (!nodeId) {
            console.error('没有选中的节点，无法保存修改');
            this.$message.error('没有选中的节点，无法保存修改');
            return;
        }
        
        // 获取当前节点的修改信息
        const nodeModify = this.nodeModifys[nodeId];
        if (!nodeModify) {
            console.error('没有找到节点的修改信息');
            this.$message.error('没有找到节点的修改信息');
            return;
        }
        
        // 发送请求保存单个节点的修改信息到服务器
        axios.post(`/figma/project/${this.project.id}/node/${nodeId}/settings`, nodeModify)
            .then(() => {
                console.log('节点修改保存成功');
                
                // 立即更新当前节点对象的modifys属性
                const targetNode = this.nodes.find(node => node.id === nodeId);
                if (targetNode) {
                    // 将修改信息合并到节点对象
                    targetNode.modifys = { ...nodeModify };
                }
                
                // 只更新当前节点的树形显示，不重新构建整个树
                this.updateTreeNodeLabel(nodeId);
                
                // 如果修改的是当前选中的节点，更新当前节点对象和属性面板
                const isCurrentNode = this.currentNode && this.currentNode.id === nodeId;
                if (isCurrentNode) {
                    // 更新当前节点对象
                    if (targetNode) {
                        this.currentNode = targetNode;
                    } else {
                        // 如果找不到targetNode，至少更新当前节点的modifys
                        this.currentNode.modifys = { ...nodeModify };
                    }
                    // 更新属性面板的表单数据（无论targetNode是否存在）
                    this.updatePropertyFormFromNode(this.currentNode);
                    
                    // 如果节点有修改信息，显示自定义设置
                    if (this.hasNodeModifys(this.currentNode.modifys)) {
                        this.showCustomSettings = true;
                        this.showLayoutSettings = true;
                        this.showExportSettings = true;
                    }
                }
            })
            .catch(error => {
                console.error('保存节点修改失败:', error);
                this.$message.error('保存节点修改失败');
            });
    },
    
    // 另存为节点图片
    async saveNodeAsImage() {
        // 获取当前选中的节点，如果没有则使用第一个匹配的节点
        let targetNodeId = this.currentNode ? this.currentNode.id : null;
        if (!targetNodeId && this.filteredPreviewImages.length > 0) {
            targetNodeId = this.filteredPreviewImages[0].nodeId;
        }
        
        if (!targetNodeId) {
            this.$message.warning('请先选择一个节点');
            return;
        }
        
        // 如果正在导出，不重复触发
        if (this.exportingNodeId === targetNodeId) {
            return;
        }
        
        // 获取节点信息
        const node = this.nodes.find(n => n.id === targetNodeId);
        const nodeName = (node && node.name) ? node.name : targetNodeId;
        
        // 获取图片格式（优先使用节点特定的格式，否则使用控制栏指定的格式）
        let imageFormat = this.defaultImageFormat || 'png';
        if (this.nodeModifys[targetNodeId] && this.nodeModifys[targetNodeId].img_ext) {
            imageFormat = this.nodeModifys[targetNodeId].img_ext;
        }
        
        // 获取缩放比例（使用控制栏指定的缩放比例）
        const scale = this.defaultImageScale || 1.0;
        
        // 构建下载URL，预览中下载时不使用缓存（useCache=false）
        const imageSrc = `/figma/image/${this.project.id}/${targetNodeId}?scale=${scale}&format=${imageFormat}&useCache=false`;
        
        // 设置导出状态
        this.exportingNodeId = targetNodeId;
        
        try {
            // 使用 fetch 下载图片，可以检测下载完成
            const response = await fetch(imageSrc);
            if (!response.ok) {
                throw new Error('下载失败');
            }
            
            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            
            // 创建下载链接
            const link = document.createElement('a');
            link.href = url;
            link.download = `${nodeName}.${imageFormat}`;
            link.style.display = 'none';
            document.body.appendChild(link);
            
            // 触发下载
            link.click();
            
            // 清理
            document.body.removeChild(link);
            window.URL.revokeObjectURL(url);
            
            // 提示用户
            this.$message({
                message: '图片下载完成',
                type: 'success',
                duration: 1500
            });
        } catch (error) {
            console.error('下载图片失败:', error);
            this.$message({
                message: '图片下载失败，请重试',
                type: 'error',
                duration: 2000
            });
        } finally {
            // 清除导出状态
            this.exportingNodeId = null;
        }
        
        // 关闭菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
    },
    
    // 保存缩略图图片
    saveMinimapImage() {
        let imageSrc = null;
        let nodeName = '';
        let imageFormat = this.defaultImageFormat || 'png'; // 优先使用控制栏指定的格式
        let nodeId = null;
        
        if (this.contextMenuMinimapType === 'current') {
            // 当前预览
            if (!this.currentNode) {
                this.$message.warning('当前没有选中的节点');
                return;
            }
            
            nodeId = this.currentNode.id;
            nodeName = this.currentNode.name || this.currentNode.id;
            
            // 优先使用节点特定的格式，否则使用控制栏指定的格式
            if (this.nodeModifys[this.currentNode.id] && this.nodeModifys[this.currentNode.id].img_ext) {
                imageFormat = this.nodeModifys[this.currentNode.id].img_ext;
            }
            
            // 构建包含格式和缩放参数的URL（缩略图右键下载时不使用缓存）
            const scale = this.defaultImageScale || 1.0;
            imageSrc = `/figma/image/${this.project.id}/${nodeId}?scale=${scale}&format=${imageFormat}&useCache=false`;
        } else if (this.contextMenuMinimapType === 'filtered') {
            // 过滤预览
            if (this.filteredImagesList.length === 0) {
                this.$message.warning('没有可用的过滤预览图片');
                return;
            }
            
            const currentImage = this.filteredImagesList[this.currentFilteredImageIndex];
            nodeId = currentImage.nodeId;
            nodeName = currentImage.nodeName || currentImage.nodeId;
            
            // 优先使用节点特定的格式，否则使用控制栏指定的格式
            if (this.nodeModifys[nodeId] && this.nodeModifys[nodeId].img_ext) {
                imageFormat = this.nodeModifys[nodeId].img_ext;
            }
            
            // 构建包含格式和缩放参数的URL（过滤预览需要excludeModified参数，缩略图右键下载时不使用缓存）
            const scale = this.defaultImageScale || 1.0;
            imageSrc = `/figma/image/${this.project.id}/${nodeId}?scale=${scale}&excludeModified=true&format=${imageFormat}&useCache=false`;
        }
        
        if (!imageSrc) {
            this.$message.error('无法获取图片');
            return;
        }
        
        // 创建下载链接
        const link = document.createElement('a');
        link.href = imageSrc;
        link.download = `${nodeName}_minimap.${imageFormat}`;
        link.style.display = 'none';
        document.body.appendChild(link);
        
        // 触发下载
        link.click();
        
        // 清理
        document.body.removeChild(link);
        
        // 提示用户
        this.$message({
            message: '缩略图已开始下载',
            type: 'success',
            duration: 1500
        });
        
        // 关闭菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
        this.contextMenuMinimapType = null;
    },
    
    // 显示节点树节点的右键菜单
    showTreeNodeContextMenu(event, nodeId) {
        // 阻止默认行为和事件冒泡
        event.preventDefault();
        event.stopPropagation();
        
        // 实时检查Ctrl键状态（解决远程连接时键盘事件丢失的问题）
        const isCtrlPressed = event.ctrlKey || event.metaKey || this.isCtrlPressed;
        
        // 如果右键的节点不在当前选择中，且没有按住Ctrl键，则单选该节点
        if (!this.isNodeSelected(nodeId) && !isCtrlPressed) {
            this.selectedNodes = [nodeId];
            this.lastSelectedNode = nodeId;
            this.handleNodeSelect(nodeId);
        }
        
        // 设置当前右键的节点ID
        this.contextMenuNodeId = nodeId;
        this.contextMenuSource = 'tree'; // 标记菜单来源为节点树
        
        // 检查是否是依赖节点分组或空白区域
        const treeNode = this.findTreeNodeById(nodeId);
        this.contextMenuIsRefGroup = treeNode && treeNode.isRefGroup;
        this.contextMenuIsRefNode = treeNode && treeNode.isRefNode && !treeNode.isRefChild;
        this.contextMenuIsRefChild = treeNode && treeNode.isRefChild;
        this.contextMenuIsEmptyArea = nodeId === '__empty_area__';
        
        // 显示菜单并设置位置
        this.layerMenuVisible = true;
        this.layerMenuStyle = {
            top: `${event.clientY}px`,
            left: `${event.clientX}px`,
            position: 'fixed'
        };
    },
    
    // 查找树节点
    findTreeNodeById(nodeId) {
        const findInTree = (nodes) => {
            for (const node of nodes) {
                if (node.id === nodeId) {
                    return node;
                }
                if (node.children && node.children.length > 0) {
                    const found = findInTree(node.children);
                    if (found) return found;
                }
            }
            return null;
        };
        return findInTree(this.treeData);
    },
    
    // 在依赖节点数据中查找节点
    findRefNodeById(nodeId) {
        // 遍历所有依赖节点的树数据
        for (const refNodeId of this.refNodes) {
            const refNodeDetail = this.refNodeDetails[refNodeId];
            if (refNodeDetail && refNodeDetail.refNodeTreeData) {
                // 在树数据中查找节点
                const foundNode = refNodeDetail.refNodeTreeData.find(node => node.id === nodeId);
                if (foundNode) {
                    return {
                        ...foundNode,
                        isRefNode: true,
                        refRootNodeId: refNodeId // 记录所属的依赖节点根ID
                    };
                }
            }
        }
        return null;
    },
    
    // 加载依赖节点预览
    async loadRefNodePreview() {
        if (!this.currentNode || !this.currentNode.isRefNodeSelected) {
            return;
        }
        console.log('加载依赖节点预览:', this.currentNode.id);
        
        try {
            // 获取依赖节点所属的根节点ID
            const refRootNodeId = this.currentNode.refRootNodeId;
            if (!refRootNodeId) {
                console.error('依赖节点缺少根节点ID');
                return;
            }
            
            // 获取依赖节点的根节点详细信息
            const refRootDetail = this.refNodeDetails[refRootNodeId];
            if (!refRootDetail || !refRootDetail.refNodeTreeData) {
                console.error('依赖节点根节点详细信息不存在');
                return;
            }
            
            // 找到依赖节点树的根节点作为预览的根节点边界
            const refRootNode = refRootDetail.refNodeTreeData.find(node => node.id === refRootNodeId);
            if (!refRootNode || !refRootNode.absoluteRenderBounds) {
                console.error('依赖节点根节点边界信息不存在');
                return;
            }
            
            // 设置依赖节点的根节点边界（用于预览定位）
            this.refNodeRootBounds = refRootNode.absoluteRenderBounds;
            
            // 创建依赖节点的预览图片列表（与主节点方式完全一致）
            this.refPreviewImages = [];
            
            // 为依赖节点树中的所有节点创建预览占位符
            for (const node of refRootDetail.refNodeTreeData) {
                if (node.absoluteRenderBounds) {
                    // 依赖节点相对于依赖节点根进行定位，保持内部结构
                    const relX = node.absoluteRenderBounds.x - this.refNodeRootBounds.x;
                    const relY = node.absoluteRenderBounds.y - this.refNodeRootBounds.y;
                    
                    // 完全模仿主节点的创建方式，不包含 style
                    const previewImageData = {
                        nodeId: node.id,
                        src: '', // 暂时为空，需要时再加载
                        nodeName: node.name || node.id,
                        loading: false,
                        isRefNode: true,
                        refRootNodeId: refRootNodeId,
                        bounds: node.absoluteRenderBounds // 使用 bounds 字段，与主节点一致
                    };
                    
                    this.refPreviewImages.push(previewImageData);
                }
            }
            
            console.log('依赖节点预览图片列表:', this.refPreviewImages);
            console.log('依赖节点预览图片数量:', this.refPreviewImages.length);
            console.log('refNodeRootBounds:', this.refNodeRootBounds);
            console.log('rootNodeBounds:', this.rootNodeBounds);
            
            // 为每个依赖节点加载图片
            for (const refImage of this.refPreviewImages) {
                // 构建图片URL（与主节点一致）
                const imageUrl = `/figma/image/${this.project.id}/${refImage.nodeId}?scale=1`;
                console.log(`[依赖节点] 加载图片 ${refImage.nodeId}:`, imageUrl);
                
                // 设置loading状态
                this.$set(refImage, 'loading', true);
                
                // 创建Image对象预加载图片
                const preloadImg = new Image();
                preloadImg.onload = () => {
                    console.log(`[依赖节点] 图片加载完成 ${refImage.nodeId}:`, imageUrl);
                    this.$set(refImage, 'src', imageUrl);
                    this.$set(refImage, 'loading', false);
                    
                    // 如果当前选中的节点就是这个依赖节点，更新缩略图
                    if (this.currentNode && this.currentNode.id === refImage.nodeId && this.currentNode.isRefNodeSelected) {
                        this.$nextTick(() => {
                            const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
                            const minimapImage = minimapContainer ? minimapContainer.querySelector('.minimap-image') : null;
                            if (minimapImage && minimapContainer) {
                                minimapContainer.style.display = 'block';
                                minimapImage.style.display = 'none';
                                minimapImage.onload = function() {
                                    minimapImage.style.display = 'block';
                                    minimapImage.style.imageRendering = 'crisp-edges';
                                };
                                minimapImage.src = imageUrl;
                            }
                        });
                    }
                };
                preloadImg.onerror = () => {
                    console.error(`[依赖节点] 图片加载失败 ${refImage.nodeId}:`, imageUrl);
                    this.$set(refImage, 'loading', false);
                };
                preloadImg.src = imageUrl;
            }
            
            // 在下一个渲染周期更新样式（完全模仿主节点的处理方式）
            this.$nextTick(() => {
                console.log('[依赖节点] $nextTick 回调执行，开始更新样式');
                this.updateRefPreviewImagesPosition();
                
                // 如果当前选中的节点已经有图片了，立即更新缩略图
                if (this.currentNode && this.currentNode.isRefNodeSelected) {
                    const currentRefImage = this.refPreviewImages.find(img => img.nodeId === this.currentNode.id);
                    if (currentRefImage && currentRefImage.src) {
                        const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
                        const minimapImage = minimapContainer ? minimapContainer.querySelector('.minimap-image') : null;
                        if (minimapImage && minimapContainer) {
                            minimapContainer.style.display = 'block';
                            minimapImage.style.display = 'none';
                            minimapImage.onload = function() {
                                minimapImage.style.display = 'block';
                                minimapImage.style.imageRendering = 'crisp-edges';
                            };
                            minimapImage.src = currentRefImage.src;
                        }
                    }
                }
            });
            
        } catch (error) {
            console.error('加载依赖节点预览失败:', error);
        }
    },
    
    // 更新依赖节点预览图位置（完全模仿主节点的 updatePreviewImagesScale）
    updateRefPreviewImagesPosition() {
        console.log('[依赖节点] updateRefPreviewImagesPosition 被调用');
        console.log('[依赖节点] refPreviewImages:', this.refPreviewImages);
        console.log('[依赖节点] refPreviewImages.length:', this.refPreviewImages?.length);
        console.log('[依赖节点] refNodeRootBounds:', this.refNodeRootBounds);
        console.log('[依赖节点] rootNodeBounds:', this.rootNodeBounds);
        
        if (!this.refPreviewImages || this.refPreviewImages.length === 0) {
            console.log('[依赖节点] 条件检查失败: refPreviewImages为空或长度为0');
            return;
        }
        
        if (!this.refNodeRootBounds) {
            console.log('[依赖节点] 条件检查失败: refNodeRootBounds不存在');
            return;
        }
        
        console.log('[依赖节点] 开始更新样式，节点数量:', this.refPreviewImages.length);
        
        // 使用主树根尺寸，确保与预览框一致
        const baseRootWidth = this.rootNodeBounds?.width || this.refNodeRootBounds.width;
        const baseRootHeight = this.rootNodeBounds?.height || this.refNodeRootBounds.height;
        
        console.log('[依赖节点] 基准根尺寸:', baseRootWidth, baseRootHeight);
        console.log('[依赖节点] zoomLevel:', this.zoomLevel);
        console.log('[依赖节点] previewOffset:', this.previewOffset);
        
        // 遍历所有依赖节点预览图（完全模仿主节点的方式）
        this.refPreviewImages.forEach((image, index) => {
            console.log(`[依赖节点] 处理第 ${index} 个节点:`, image.nodeId);
            console.log(`[依赖节点] 节点数据:`, image);
            
            if (!image.bounds) {
                console.log(`[依赖节点] ${image.nodeId} 没有边界框，跳过`);
                return;
            }
            
            console.log(`[依赖节点] ${image.nodeId} bounds:`, image.bounds);
            
            // 计算相对于主树根的位置（若主树根不存在则回退到依赖根）
            const baseRoot = this.rootNodeBounds || this.refNodeRootBounds;
            const relX = image.bounds.x - baseRoot.x;
            const relY = image.bounds.y - baseRoot.y;
            
            console.log(`[依赖节点] ${image.nodeId} 相对位置:`, relX, relY);
            console.log(`[依赖节点] ${image.nodeId} baseRoot:`, baseRoot);
            
            // 检查是否是当前选中的节点
            const isSelected = image.nodeId === this.currentNode?.id;
            
            // 创建样式对象（与主节点完全一致的方式）
            const style = {
                position: 'absolute',
                left: `calc(50% + ${(relX - baseRootWidth/2) * this.zoomLevel + this.previewOffset.x}px)`,
                top: `calc(50% + ${(relY - baseRootHeight/2) * this.zoomLevel + this.previewOffset.y}px)`,
                width: `${image.bounds.width}px`,
                height: `${image.bounds.height}px`,
                transform: `scale(${this.zoomLevel})`,
                transformOrigin: '0 0',
                zIndex: index + 1
            };
            
            // 只有选中的节点才显示虚线边框和背景色
            if (isSelected) {
                style.border = '2px dashed #409EFF';
                style.backgroundColor = 'rgba(64, 158, 255, 0.1)';
            }
            
            console.log(`[依赖节点] ${image.nodeId} 计算样式:`, style);
            console.log(`[依赖节点] ${image.nodeId} 设置样式前，image.style:`, image.style);
            
            // 使用 $set 更新样式（与主节点完全一致）
            this.$set(image, 'style', style);
            
            console.log(`[依赖节点] ${image.nodeId} 设置样式后，image.style:`, image.style);
            console.log(`[依赖节点] ${image.nodeId} 设置样式后，image对象:`, image);
        });
        
        console.log('[依赖节点] 所有节点样式更新完成');
        console.log('[依赖节点] 最终 refPreviewImages:', this.refPreviewImages);
    },
    
    // 添加依赖节点
    async addRefNode() {
        try {
            const { value: nodeId } = await this.$prompt('请输入要添加的节点ID:', '添加依赖节点', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                inputValidator: (value) => {
                    if (!value || value.trim() === '') {
                        return '节点ID不能为空';
                    }
                    
                    const trimmedValue = value.trim();
                    
                    // 校验格式：只允许数字、冒号(:)和连字符(-)
                    const validPattern = /^[0-9:\-]+$/;
                    if (!validPattern.test(trimmedValue)) {
                        return '节点ID只能包含数字、冒号(:)和连字符(-)';
                    }
                    
                    // 统一格式：将连字符转换为冒号（与后端保持一致）
                    const normalizedNodeId = trimmedValue.replace(/-/g, ':');
                    
                    // 检查节点是否在主节点树中
                    const nodeInMainTree = this.nodes.some(node => {
                        // 统一节点ID格式后进行比较
                        const nodeIdNormalized = (node.id || '').replace(/-/g, ':');
                        return nodeIdNormalized === normalizedNodeId;
                    });
                    
                    if (nodeInMainTree) {
                        return '该节点已包含在主节点树中，不能添加为依赖节点';
                    }
                    
                    // 检查是否已经是依赖节点
                    const isRefNode = this.refNodes.some(refNodeId => {
                        // 统一节点ID格式后进行比较
                        const refNodeIdNormalized = (refNodeId || '').replace(/-/g, ':');
                        return refNodeIdNormalized === normalizedNodeId;
                    });
                    
                    if (isRefNode) {
                        return '该节点已经是依赖节点';
                    }
                    
                    return true;
                }
            });
            
            const formData = new FormData();
            formData.append('node_id', nodeId.trim());
            
            const response = await axios.post(`/figma/project/${this.project.id}/ref-nodes`, formData);
            
            if (response.data.success) {
                this.$message.success('依赖节点添加成功');
                // 重新加载项目数据
                this.loadProjectData();
            } else {
                this.$message.error(response.data.error || '添加依赖节点失败');
            }
        } catch (error) {
            if (error !== 'cancel') {
                console.error('添加依赖节点失败:', error);
                this.$message.error('添加依赖节点失败: ' + (error.response?.data?.error || error.message));
            }
        }
        this.hideLayerMenu();
    },
    
    // 移除依赖节点
    async removeRefNode(nodeId) {
        try {
            await this.$confirm(`确定要移除依赖节点 "${nodeId}" 吗？`, '确认移除', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                type: 'warning'
            });
            
            const response = await axios.delete(`/figma/project/${this.project.id}/ref-nodes/${nodeId}`);
            
            if (response.data.success) {
                this.$message.success('依赖节点移除成功');
                // 重新加载项目数据
                this.loadProjectData();
            } else {
                this.$message.error(response.data.error || '移除依赖节点失败');
            }
        } catch (error) {
            if (error !== 'cancel') {
                console.error('移除依赖节点失败:', error);
                this.$message.error('移除依赖节点失败: ' + (error.response?.data?.error || error.message));
            }
        }
        this.hideLayerMenu();
    },
    
    // 显示树形区域空白处的右键菜单
    showTreeEmptyAreaContextMenu(event) {
        // 阻止默认行为和事件冒泡
        event.preventDefault();
        event.stopPropagation();
        
        // 设置为空白区域
        this.contextMenuNodeId = '__empty_area__';
        this.contextMenuSource = 'tree';
        this.contextMenuIsRefGroup = false;
        this.contextMenuIsRefNode = false;
        this.contextMenuIsEmptyArea = true;
        
        // 显示菜单并设置位置
        this.layerMenuVisible = true;
        this.layerMenuStyle = {
            top: `${event.clientY}px`,
            left: `${event.clientX}px`,
            position: 'fixed'
        };
    },
    
    // 显示特定节点的层级菜单（保留用于预览区域的右键菜单，如果需要）
    showLayerMenu(event, nodeId) {
        // 阻止默认行为和事件冒泡
        event.preventDefault();
        event.stopPropagation();
        
        // 确保不会触发拖拽
        this.isDragging = false;
        
        this.contextMenuNodeId = nodeId;
        this.layerMenuVisible = true;
        this.layerMenuStyle = {
            top: `${event.clientY}px`,
            left: `${event.clientX}px`,
            position: 'fixed' // 使用固定定位，相对于视口
        };
        
        // 获取点击位置相对于预览容器的坐标
        const containerRect = this.$refs.previewContainer.getBoundingClientRect();
        const clickX = event.clientX - containerRect.left;
        const clickY = event.clientY - containerRect.top;
        
        // 容器中心点
        const containerCenterX = containerRect.width / 2;
        const containerCenterY = containerRect.height / 2;
        
        // 根节点的宽高
        const rootWidth = this.rootNodeBounds.width;
        const rootHeight = this.rootNodeBounds.height;
        
        // 计算点击位置相对于根节点左上角的坐标
        const rootLeftX = containerCenterX - (rootWidth/2) * this.zoomLevel;
        const rootTopY = containerCenterY - (rootHeight/2) * this.zoomLevel;
        
        // 计算点击位置相对于根节点左上角的位置，并除以缩放比例得到实际坐标
        const relativeX = (clickX - rootLeftX) / this.zoomLevel;
        const relativeY = (clickY - rootTopY) / this.zoomLevel;
        
        // 转换为绝对坐标
        const absoluteX = relativeX + this.rootNodeBounds.x;
        const absoluteY = relativeY + this.rootNodeBounds.y;
        
        // 找出所有包含点击位置的节点
        const matchingNodes = [];
        for (const image of this.previewImages) {
            const node = this.nodes.find(n => n.id === image.nodeId);
            if (!node || !node.absoluteRenderBounds) {
                continue;
            }
            
            // 判断点击位置是否在节点范围内
            if (
                absoluteX >= node.absoluteRenderBounds.x && 
                absoluteX <= node.absoluteRenderBounds.x + node.absoluteRenderBounds.width && 
                absoluteY >= node.absoluteRenderBounds.y && 
                absoluteY <= node.absoluteRenderBounds.y + node.absoluteRenderBounds.height
            ) {
                matchingNodes.push(node);
            }
        }
        
        // 如果找到匹配的节点，显示所有匹配节点
        if (matchingNodes.length > 0) {
            // 过滤预览图片列表，保留所有匹配的节点
            this.filteredPreviewImages = this.previewImages.filter(img => 
                matchingNodes.some(node => node.id === img.nodeId)
            );
        } else {
            // 如果没有找到匹配节点，则只显示当前节点
            this.filteredPreviewImages = this.previewImages.filter(img => img.nodeId === nodeId);
        }
    },
        
        // 预加载所有节点的占位区域
        preloadNodeAreas() {
            // 创建一个辅助函数来检查节点及其所有祖先节点是否都可见且未被忽略
            const isNodeAndAncestorsVisible = (nodeId) => {
                const node = this.nodes.find(n => n.id === nodeId);
                if (!node) return false;
                
                // 检查当前节点是否可见
                if (node.visible === false) {
                    return false;
                }
                
                // 检查当前节点是否被忽略
                if (node.modifys && node.modifys.ignore === true) {
                    return false;
                }
                
                // 如果有父节点，递归检查父节点
                if (node.parent_id) {
                    return isNodeAndAncestorsVisible(node.parent_id);
                }
                
                // 如果是根节点且通过了检查，返回true
                return true;
            };
            
            // 找出所有可见且未被忽略的节点（包括检查其祖先节点）
            const visibleNodes = this.nodes.filter(node => {
                // 检查节点是否有边界框信息
                if (!node.absoluteRenderBounds) {
                    return false;
                }
                
                // 检查节点及其所有祖先节点是否都可见且未被忽略
                return isNodeAndAncestorsVisible(node.id);
            });
            
            console.log('预加载节点数量:', visibleNodes.length, '(已过滤不可见、被忽略的节点及其子树)');
            
            // 找出根节点（最顶层的节点）
            const rootNodes = visibleNodes.filter(node => !node.parent_id);
            if (rootNodes.length > 0) {
                const rootNode = rootNodes[0];
                this.rootNodeBounds = JSON.parse(JSON.stringify(rootNode.absoluteRenderBounds)); // 深拷贝
                console.log('设置根节点边界:', this.rootNodeBounds);
            }
            
            // 清空预加载节点列表
            this.preloadedNodes = [];
            
            // 为每个节点创建占位区域
            visibleNodes.forEach(node => {
                this.preloadedNodes.push({
                    nodeId: node.id,
                    nodeName: node.name || node.id,
                    bounds: node.absoluteRenderBounds,
                    loaded: false // 标记为未加载实际图片
                });
            });
            
            // 根据节点在树中的顺序对预加载节点进行排序
            this.sortPreloadedNodesByTreeOrder();
            
            // 更新所有占位区域的样式
            this.$nextTick(() => {
                this.updatePreloadedNodesStyle();
                this.fitToScreen(); // 适应窗口显示
            });
        },
        
        // 根据节点在树中的顺序对预加载节点进行排序
        sortPreloadedNodesByTreeOrder() {
            // 创建一个节点ID到树中顺序的映射
            const nodeOrderMap = new Map();
            
            // 递归遍历树节点，记录每个节点的顺序
            const traverseTree = (nodes, level = 0, index = 0) => {
                let currentIndex = index;
                for (let i = 0; i < nodes.length; i++) {
                    const node = nodes[i];
                    // 记录节点顺序，层级越深，顺序值越大，同层级按索引排序
                    nodeOrderMap.set(node.id, level * 10000 + currentIndex);
                    currentIndex++;
                    
                    // 递归处理子节点
                    if (node.children && node.children.length > 0) {
                        currentIndex = traverseTree(node.children, level + 1, currentIndex);
                    }
                }
                return currentIndex;
            };
            
            // 从根节点开始遍历
            traverseTree(this.treeData);
            
            // 根据节点顺序排序预加载节点
            this.preloadedNodes.sort((a, b) => {
                const orderA = nodeOrderMap.get(a.nodeId) || 0;
                const orderB = nodeOrderMap.get(b.nodeId) || 0;
                return orderB - orderA; // 树中顺序靠前的节点，在z轴上层级更高
            });
        },
        
    // 更新所有预加载节点的样式
    updatePreloadedNodesStyle() {
        if (!this.preloadedNodes.length || !this.rootNodeBounds) {
            return;
        }
        
        // 根节点的宽高
        const rootWidth = this.rootNodeBounds.width;
        const rootHeight = this.rootNodeBounds.height;
        
        // 更新所有预加载节点的样式
        this.preloadedNodes.forEach((preloadedNode, index) => {
            if (!preloadedNode.bounds) {
                return;
            }
            
            // 计算相对于根节点的位置
            const relX = preloadedNode.bounds.x - this.rootNodeBounds.x;
            const relY = preloadedNode.bounds.y - this.rootNodeBounds.y;
            
            // 使用绝对定位，确保只使用绝对位置，只减去根节点的位置
            // 计算节点在容器中的位置：容器中心 + 相对于根节点中心的偏移量 + 拖拽偏移量
            const style = {
                position: 'absolute',
                left: `calc(50% + ${(relX - rootWidth/2) * this.zoomLevel + this.previewOffset.x}px)`,
                top: `calc(50% + ${(relY - rootHeight/2) * this.zoomLevel + this.previewOffset.y}px)`,
                width: `${preloadedNode.bounds.width}px`,
                height: `${preloadedNode.bounds.height}px`,
                transform: `scale(${this.zoomLevel})`,
                transformOrigin: '0 0', // 从左上角开始变换
                zIndex: index + 1 // 使用索引作为z-index基础值，确保按树节点顺序叠放
            };
            
            // 更新样式
            this.$set(preloadedNode, 'style', style);
        });
    },

    // 从节点数据更新属性表单，不重新加载
    updatePropertyFormFromNode(node) {
        if (!node) return;
        
        // 获取节点的modifys，如果不存在则使用空对象
        const modifys = node.modifys || {};
        
        // 更新属性表单的数据
        this.propertyForm = {
            img_ext: modifys.img_ext || 'png',
            components: modifys.components || [],
            rename: modifys.rename || '',
            ignore: modifys.ignore || false,
            res_mode: modifys.res_mode || 'attach',
            img_name: modifys.img_name || '',
            img_id: modifys.img_id || '',
            horizontal: modifys.horizontal || 'CENTER',
            vertical: modifys.vertical || 'CENTER',
            parent_id: modifys.parent_id || ''
        };
        
        // 强制Vue更新视图
        this.$forceUpdate();
    },

    // 根据节点的modify数据决定使用哪个节点ID来获取预览
    getPreviewNodeId(node) {
        if (!node) return null;
        
        // 检查节点是否有modify数据
        if (node.modifys) {
            // 检查ignore是否不为false（即节点未被忽略）
            if (node.modifys.ignore !== true) {
                // 检查img_id是否不为空
                if (node.modifys.img_id && node.modifys.img_id.trim() !== '') {
                    console.log(`节点 ${node.id} 有img_id配置: ${node.modifys.img_id}`);
                    return node.modifys.img_id;
                }
            }
        }
        
        // 如果不满足条件，使用原始节点ID
        return node.id;
    },
        
        // 添加预览图片（实际加载图片）
        addPreviewImage(node, forceRefresh = false, scale = 1.0) { // 默认不强制刷新，使用1.0标准清晰度
            if (!node) return;
            
            // 检查节点的modify数据，决定使用哪个节点ID来获取预览
            const previewNodeId = this.getPreviewNodeId(node);
            console.log(`节点 ${node.id} 预览使用节点ID: ${previewNodeId}`);
            
            // 优先尝试使用Figma Plugin API获取单个节点预览
            if (window.figmaPluginBridge && window.figmaPluginBridge.isAvailable()) {
                console.log('使用Figma Plugin API获取单个节点预览:', previewNodeId);
                this.addPreviewImageFromPlugin(node, forceRefresh, scale, previewNodeId);
                return;
            }
            
            // 回退到后端API
            console.log('使用后端API获取单个节点预览:', previewNodeId);
            
            // 构建图片URL（不添加时间戳），增加scale参数提高清晰度
            const imageUrl = `/figma/image/${this.project.id}/${previewNodeId}?scale=${scale}`;
            console.log('图片URL:', imageUrl);
            
            // 使用后端API的逻辑
            this.addPreviewImageFromBackend(node, forceRefresh, scale, previewNodeId);
        },

        // 使用Figma Plugin API添加预览图片
        async addPreviewImageFromPlugin(node, forceRefresh = false, scale = 1.0, previewNodeId = null) {
            if (!node) return;
            
            // 如果没有指定previewNodeId，使用节点自身的ID
            const nodeIdToPreview = previewNodeId || node.id;
            
            try {
                // 调用Figma Plugin API获取节点预览
                const result = await window.figmaPluginBridge.previewNode(
                    nodeIdToPreview, 
                    scale, 
                    this.defaultImageFormat.toUpperCase()
                );
                
                if (!result || !result.imageUrl) {
                    console.warn('Figma Plugin返回的数据格式不正确，回退到后端API');
                    this.addPreviewImageFromBackend(node, forceRefresh, scale, previewNodeId);
                    return;
                }
                
                const imageUrl = result.imageUrl; // base64 data URL
                console.log('从Figma Plugin获取到图片URL:', imageUrl.substring(0, 50) + '...');
                
                // 处理节点边界框信息
                this.processNodeBounds(node);
                
                // 添加或更新预览图片
                this.updatePreviewImagesList(node, imageUrl);
                
                // 在下一个渲染周期更新显示
                this.$nextTick(() => {
                    this.updatePreviewImagesScale();
                    this.updateMinimapDisplay(node, imageUrl);
                });
                
            } catch (error) {
                console.warn('Figma Plugin API调用失败，回退到后端API:', error);
                this.addPreviewImageFromBackend(node, forceRefresh, scale, previewNodeId);
            }
        },

        // 使用后端API添加预览图片（原始逻辑）
        addPreviewImageFromBackend(node, forceRefresh = false, scale = 1.0, previewNodeId = null) {
            if (!node) return;
            
            // 如果没有指定previewNodeId，使用节点自身的ID
            const nodeIdToPreview = previewNodeId || node.id;
            
            // 构建图片URL
            const imageUrl = `/figma/image/${this.project.id}/${nodeIdToPreview}?scale=${scale}`;
            console.log('后端API图片URL:', imageUrl);
            
            // 处理节点边界框信息
            this.processNodeBounds(node);
            
            // 添加或更新预览图片
            this.updatePreviewImagesList(node, imageUrl);
            
            // 在下一个渲染周期更新显示
            this.$nextTick(() => {
                this.updatePreviewImagesScale();
                
                // 创建一个新的Image对象来预加载图片
                const preloadImg = new Image();
                preloadImg.onload = () => {
                    console.log('后端API预览图片加载完成:', imageUrl);
                    this.updateImageLoadingState(node.id, false);
                    this.updateMinimapDisplay(node, imageUrl);
                };
                
                preloadImg.onerror = () => {
                    console.error('后端API预览图片加载失败:', imageUrl);
                    this.updateImageLoadingState(node.id, false);
                };
                
                // 开始加载图片
                preloadImg.src = imageUrl;
            });
        },

        // 处理节点边界框信息
        processNodeBounds(node) {
            // 如果节点没有absoluteBoundingBox，但是是根节点，则创建一个默认的边界框
            if (!node.absoluteRenderBounds && this.previewImages.length === 0) {
                //console.log('根节点没有absoluteBoundingBox，创建默认边界框');
                node.absoluteRenderBounds = {
                    x: 0,
                    y: 0,
                    width: 800,  // 默认宽度
                    height: 600  // 默认高度
                };
            }
            
            // 如果是第一个节点（根节点）或者还没有设置rootNodeBounds，记录其位置作为参考
            if (node.absoluteRenderBounds && (this.previewImages.length === 0 || !this.rootNodeBounds)) {
                this.rootNodeBounds = JSON.parse(JSON.stringify(node.absoluteRenderBounds)); // 深拷贝
                console.log('设置根节点边界:', this.rootNodeBounds);
                
                // 适应窗口显示
                this.$nextTick(() => {
                    this.fitToScreen();
                });
            }
        },

        // 更新预览图片列表
        updatePreviewImagesList(node, imageUrl) {
            const previewImageData = {
                nodeId: node.id,
                src: imageUrl,
                nodeName: node.name || node.id,
                loading: true
            };
            
            // 如果有边界框信息，添加到数据中
            if (node.absoluteRenderBounds) {
                previewImageData.bounds = node.absoluteRenderBounds;
            }
            
            // 检查是否已存在该节点的预览图
            const existingIndex = this.previewImages.findIndex(img => img.nodeId === node.id);
            if (existingIndex !== -1) {
                this.$set(this.previewImages, existingIndex, previewImageData);
            } else {
                this.previewImages.push(previewImageData);
                // 根据节点在树中的顺序对预览图片进行排序
                this.sortPreviewImagesByTreeOrder();
            }
            
            // 更新对应预加载节点的loaded状态
            const preloadedNodeIndex = this.preloadedNodes.findIndex(pNode => pNode.nodeId === node.id);
            if (preloadedNodeIndex !== -1) {
                this.$set(this.preloadedNodes[preloadedNodeIndex], 'loaded', true);
            }
            
            // 对于Plugin API，立即设置为加载完成状态（因为是base64数据）
            if (imageUrl.startsWith('data:')) {
                this.updateImageLoadingState(node.id, false);
            }
        },

        // 更新图片加载状态
        updateImageLoadingState(nodeId, loading) {
            const imgIndex = this.previewImages.findIndex(img => img.nodeId === nodeId);
            if (imgIndex !== -1) {
                this.$set(this.previewImages[imgIndex], 'loading', loading);
            }
        },

        // 更新缩略图显示
        updateMinimapDisplay(node, imageUrl) {
            const minimapContainer = document.querySelector('.minimap-container');
            const minimapImage = document.querySelector('.minimap-image');
            
            if (minimapContainer) {
                if (this.previewImages.length > 0 && this.currentNode) {
                    minimapContainer.style.display = 'block';
                    
                    // 更新缩略图
                    if (minimapImage && this.currentNode.id === node.id) {
                        minimapImage.src = imageUrl;
                        minimapImage.style.display = 'block';
                        minimapImage.style.imageRendering = 'crisp-edges';
                    }
                } else {
                    minimapContainer.style.display = 'none';
                }
            }
        },
        
        // 根据节点在树中的顺序对预览图片进行排序
        sortPreviewImagesByTreeOrder() {
            // 创建一个节点ID到树中顺序的映射
            const nodeOrderMap = new Map();
            
            // 递归遍历树节点，记录每个节点的顺序
            const traverseTree = (nodes, level = 0, index = 0) => {
                let currentIndex = index;
                for (let i = 0; i < nodes.length; i++) {
                    const node = nodes[i];
                    // 记录节点顺序，层级越深，顺序值越大，同层级按索引排序
                    nodeOrderMap.set(node.id, level * 10000 + currentIndex);
                    currentIndex++;
                    
                    // 递归处理子节点
                    if (node.children && node.children.length > 0) {
                        currentIndex = traverseTree(node.children, level + 1, currentIndex);
                    }
                }
                return currentIndex;
            };
            
            // 从根节点开始遍历
            traverseTree(this.treeData);
            
            // 根据节点顺序排序预览图片
            this.previewImages.sort((a, b) => {
                const orderA = nodeOrderMap.get(a.nodeId) || 0;
                const orderB = nodeOrderMap.get(b.nodeId) || 0;
                return orderB - orderA; // 树中顺序靠前的节点，在z轴上层级更高
            });
        },
        
        // 显示自定义设置对话框
        showCustomizeDialog() {
            this.showCustomSettings = true;
        },
        
        // 放大预览
        zoomIn() {
            if (this.zoomLevel < this.maxZoom) {
                this.zoomLevel += this.zoomStep;
                this.updatePreviewImagesScale();
            }
        },
        
        // 缩小预览
        zoomOut() {
            if (this.zoomLevel > this.minZoom) {
                this.zoomLevel -= this.zoomStep;
                this.updatePreviewImagesScale();
            }
        },
        
    // 适应窗口
    fitToScreen() {
        if (!this.rootNodeBounds || !this.$refs.previewContainer) return;
        
        // 获取预览容器的尺寸
        const containerWidth = this.$refs.previewContainer.clientWidth;
        const containerHeight = this.$refs.previewContainer.clientHeight;
        
        console.log('容器尺寸:', containerWidth, containerHeight);
        
        // 获取根节点的尺寸
        const nodeWidth = this.rootNodeBounds.width;
        const nodeHeight = this.rootNodeBounds.height;
        
        console.log('根节点尺寸:', nodeWidth, nodeHeight);
        
        // 计算合适的缩放比例，考虑边距
        const padding = 40; // 边距
        const scaleX = (containerWidth - padding) / nodeWidth;
        const scaleY = (containerHeight - padding) / nodeHeight;
        
        // 取较小值，确保完全可见
        this.zoomLevel = Math.min(scaleX, scaleY, 1); // 最大不超过1倍
        
        console.log('计算的缩放比例:', this.zoomLevel);
        
        // 重置拖拽偏移量
        this.previewOffset = { x: 0, y: 0 };
        
        // 更新所有预览图片的缩放
        this.updatePreviewImagesScale();
    },
    
    // 定位聚焦到选中节点
        focusToSelectedNode() {
        if (!this.currentNode) return;
        
        let node = null;
        let isRefNode = false;
        
        // 查找当前选中节点（先在主节点中查找，再在依赖节点中查找）
        node = this.nodes.find(n => n.id === this.currentNode.id);
        
        if (!node && this.currentNode.isRefNodeSelected) {
            // 如果是依赖节点，使用当前节点的数据
            node = this.currentNode;
            isRefNode = true;
        }
        
        if (!node || !node.absoluteRenderBounds) {
            console.log('无法找到节点边界信息，无法聚焦');
            return;
        }
        
        // 获取节点的边界信息
        const nodeBounds = node.absoluteRenderBounds;
        
        // 对于依赖节点，需要检查依赖节点根边界是否存在
        if (isRefNode && !this.refNodeRootBounds) {
            console.log('依赖节点根边界信息不存在，无法聚焦');
            return;
        }
        
        // 检查主树根节点边界是否存在
        if (!this.rootNodeBounds) {
            console.log('主树根节点边界信息不存在，无法聚焦');
            return;
        }
        
        console.log('聚焦到节点:', node.id, node.absoluteRenderBounds);
        console.log('是否为依赖节点:', isRefNode);
        
        // 计算节点中心相对于主树根节点的位置
        // 注意：无论是主节点还是依赖节点，都应该基于主树根节点来计算相对位置
        const nodeCenterX = nodeBounds.x + nodeBounds.width / 2;
        const nodeCenterY = nodeBounds.y + nodeBounds.height / 2;
        const rootCenterX = this.rootNodeBounds.x + this.rootNodeBounds.width / 2;
        const rootCenterY = this.rootNodeBounds.y + this.rootNodeBounds.height / 2;
        
        // 计算相对偏移量
        const relativeX = nodeCenterX - rootCenterX;
        const relativeY = nodeCenterY - rootCenterY;
        
        console.log('节点中心:', nodeCenterX, nodeCenterY);
        console.log('根节点中心:', rootCenterX, rootCenterY);
        console.log('相对偏移:', relativeX, relativeY);
        
        // 计算需要的偏移量，使节点中心位于屏幕中心
        // 由于预览图片的定位是：calc(50% + 相对偏移 * 缩放 + 拖拽偏移)
        // 要让节点中心位于屏幕中心，需要设置拖拽偏移为：-相对偏移 * 缩放
        this.previewOffset = {
            x: -relativeX * this.zoomLevel,
            y: -relativeY * this.zoomLevel
        };
        
        console.log('设置预览偏移:', this.previewOffset);
        
        // 更新所有预览图片的位置
        this.updatePreviewImagesScale();
        
        // 显示聚焦提示
        this.$message({
            message: `已聚焦到节点: ${node.name || node.id}`,
            type: 'success',
            duration: 2000
        });
    },
        
    // 获取选中节点高层级预览层的样式
    getSelectionOverlayStyle() {
        if (!this.currentNode || !this.rootNodeBounds) {
            return {};
        }
        
        // 查找当前选中节点
        const node = this.nodes.find(n => n.id === this.currentNode.id);
        if (!node || !node.absoluteRenderBounds) {
            return {};
        }
        
        // 根节点的宽高
        const rootWidth = this.rootNodeBounds.width;
        const rootHeight = this.rootNodeBounds.height;
        
        // 计算相对于根节点的位置
        const relX = node.absoluteRenderBounds.x - this.rootNodeBounds.x;
        const relY = node.absoluteRenderBounds.y - this.rootNodeBounds.y;
        
    // 使用绝对定位，确保只使用绝对位置，只减去根节点的位置
    // 计算节点在容器中的位置：容器中心 + 相对于根节点中心的偏移量 + 拖拽偏移量
    return {
        position: 'absolute',
        left: `calc(50% + ${(relX - rootWidth/2) * this.zoomLevel + this.previewOffset.x}px)`,
        top: `calc(50% + ${(relY - rootHeight/2) * this.zoomLevel + this.previewOffset.y}px)`,
        width: `${node.absoluteRenderBounds.width * this.zoomLevel}px`,
        height: `${node.absoluteRenderBounds.height * this.zoomLevel}px`,
        transformOrigin: '0 0' // 从左上角开始变换
    };
    },
        
    // 更新所有预览图片的缩放
    updatePreviewImagesScale() {
        console.log('[主树] updatePreviewImagesScale 被调用');
        
        // 更新预加载节点的样式
        this.updatePreloadedNodesStyle();
        
        // 更新依赖节点预览图的位置和缩放
        this.updateRefPreviewImagesPosition();
        
        if (!this.previewImages.length || !this.rootNodeBounds) {
            console.log('[主树] 没有预览图或根节点边界，无法更新缩放');
            return;
        }
            
            console.log('[主树] 更新预览图缩放，当前缩放比例:', this.zoomLevel);
            console.log('[主树] previewImages数量:', this.previewImages.length);
            console.log('[主树] previewImages:', this.previewImages);
            
            // 根节点的宽高
            const rootWidth = this.rootNodeBounds.width;
            const rootHeight = this.rootNodeBounds.height;
            
            console.log('[主树] 根节点尺寸:', rootWidth, rootHeight);
            console.log('[主树] zoomLevel:', this.zoomLevel);
            console.log('[主树] previewOffset:', this.previewOffset);
            
            // 更新所有预览图片的样式
            this.previewImages.forEach((image, index) => {
                console.log(`[主树] 处理第 ${index} 个节点:`, image.nodeId);
                console.log(`[主树] 节点数据:`, image);
                
                let node = this.nodes.find(n => n.id === image.nodeId);
                let bounds = null;
                let baseRootBounds = this.rootNodeBounds;
                let isRefFallback = false;
                if (!node || !node.absoluteRenderBounds) {
                    // 主树未找到该节点，尝试在依赖预览中找到同ID的bounds作为回退
                    const refImg = this.refPreviewImages.find(r => r.nodeId === image.nodeId && r.bounds);
                    if (refImg) {
                        bounds = refImg.bounds;
                        isRefFallback = true;
                        console.log(`[主树] ${image.nodeId} 使用依赖节点bounds作为回退`);
                    } else {
                        console.log(`[主树] ${image.nodeId} 没有边界框，跳过`);
                        return;
                    }
                } else {
                    bounds = node.absoluteRenderBounds;
                }

                console.log(`[主树] ${image.nodeId} bounds:`, bounds);

                // 计算相对于根节点的位置
                const relX = bounds.x - baseRootBounds.x;
                const relY = bounds.y - baseRootBounds.y;
                
                // 计算相对位置的百分比（相对于根节点尺寸）
                const percentX = relX / rootWidth * 100;
                const percentY = relY / rootHeight * 100;
                
                console.log(`[主树] ${image.nodeId}${isRefFallback ? ' (ref)' : ''} 相对位置:`, relX, relY, '百分比:', percentX.toFixed(2) + '%', percentY.toFixed(2) + '%');
                
                // 使用绝对定位，确保只使用绝对位置，只减去根节点的位置
                // 计算节点在容器中的位置：容器中心 + 相对于根节点中心的偏移量 + 拖拽偏移量
                const style = {
                    position: 'absolute',
                    left: `calc(50% + ${(relX - rootWidth/2) * this.zoomLevel + this.previewOffset.x}px)`,
                    top: `calc(50% + ${(relY - rootHeight/2) * this.zoomLevel + this.previewOffset.y}px)`,
                    width: `${bounds.width}px`,
                    height: `${bounds.height}px`,
                    transform: `scale(${this.zoomLevel})`,
                    transformOrigin: '0 0', // 从左上角开始变换
                    zIndex: index + 1 // 使用索引作为z-index基础值，确保按树节点顺序叠放
                };
                
                // 不修改选中节点的层级，始终按节点树正序遍历的顺序排列
                
                console.log(`[主树] ${image.nodeId} 计算样式:`, style);
                console.log(`[主树] ${image.nodeId} 设置样式前，image.style:`, image.style);
                
                // 更新样式
                this.$set(image, 'style', style);
                
                console.log(`[主树] ${image.nodeId} 设置样式后，image.style:`, image.style);
                console.log(`[主树] ${image.nodeId} 设置样式后，image对象:`, image);
            });
            
            console.log('[主树] 所有节点样式更新完成');
            console.log('[主树] 最终 previewImages:', this.previewImages);
            
            // 如果有选中的节点，更新选中节点的高层级预览样式
            if (this.currentNode) {
                this.$nextTick(() => {
                    // 调用方法更新选中节点的预览层样式
                    this.updateSelectionOverlay();
                });
            }
        },
        
    // 处理树节点点击事件（从左侧树点击）
    // 此方法已移除，使用683行的handleTreeNodeClick方法
    
    // 处理节点点击事件（从占位区域点击）
    handleNodeClick(nodeId) {
        console.log('节点被点击:', nodeId);
        // 查找节点路径并展开树，确保在树中定位到节点
        this.findNodePathAndExpand(nodeId);
        // 直接调用handleNodeSelect方法处理节点选择
        this.handleNodeSelect(nodeId);
    },
    
    // 处理预览区域点击事件
    handlePreviewClick(event) {
        console.log('预览区域被点击');
            
            // 如果层级菜单可见，忽略点击事件，防止穿透
            if (this.layerMenuVisible) {
                console.log('层级菜单可见，忽略预览区域点击');
                return;
            }
            
            if (!this.previewImages.length || !this.$refs.previewContainer || !this.rootNodeBounds) {
                console.log('没有预览图片、预览容器或根节点边界不存在');
                return;
            }
            
            // 获取点击位置相对于预览容器的坐标
            const containerRect = this.$refs.previewContainer.getBoundingClientRect();
            const clickX = event.clientX - containerRect.left;
            const clickY = event.clientY - containerRect.top;
            
            console.log('点击位置:', clickX, clickY);
            console.log('预览图片数量:', this.previewImages.length);
            console.log('预览容器尺寸:', containerRect.width, containerRect.height);
            
            // 容器中心点
            const containerCenterX = containerRect.width / 2;
            const containerCenterY = containerRect.height / 2;
            
            // 根节点的宽高
            const rootWidth = this.rootNodeBounds.width;
            const rootHeight = this.rootNodeBounds.height;
            
            // 计算点击位置相对于根节点左上角的坐标
            // 首先，计算根节点左上角在屏幕上的位置，考虑预览偏移量
            const rootLeftX = containerCenterX - (rootWidth/2) * this.zoomLevel + this.previewOffset.x;
            const rootTopY = containerCenterY - (rootHeight/2) * this.zoomLevel + this.previewOffset.y;
            
            console.log('根节点左上角屏幕位置（考虑偏移）:', rootLeftX, rootTopY);
            console.log('预览偏移量:', this.previewOffset.x, this.previewOffset.y);
            
            // 然后，计算点击位置相对于根节点左上角的位置，并除以缩放比例得到实际坐标
            const relativeX = (clickX - rootLeftX) / this.zoomLevel;
            const relativeY = (clickY - rootTopY) / this.zoomLevel;
            
            // 这里的relativeX和relativeY是相对于根节点左上角(0,0)的坐标，而不是相对于根节点原点的坐标
            // 因此，需要加上根节点的x,y值，得到绝对坐标系中的位置，以便与节点的absoluteBoundingBox进行比较
            const absoluteX = relativeX + this.rootNodeBounds.x;
            const absoluteY = relativeY + this.rootNodeBounds.y;
            
            console.log('转换为绝对坐标:', absoluteX, absoluteY);
            
            console.log('相对于根节点的坐标:', relativeX, relativeY);
            
            // 找出所有包含点击位置的节点
            const matchingNodes = [];
            for (const image of this.previewImages) {
                const node = this.nodes.find(n => n.id === image.nodeId);
                if (!node || !node.absoluteRenderBounds) {
                    console.log('跳过节点，无边界框:', image.nodeId);
                    continue;
                }
                
                // 计算节点相对于根节点的位置
                const nodeX = node.absoluteRenderBounds.x - this.rootNodeBounds.x;
                const nodeY = node.absoluteRenderBounds.y - this.rootNodeBounds.y;
                const nodeWidth = node.absoluteRenderBounds.width;
                const nodeHeight = node.absoluteRenderBounds.height;
                
                console.log('检查节点:', node.id, 'x:', nodeX, 'y:', nodeY, 'w:', nodeWidth, 'h:', nodeHeight);
                
                // 判断点击位置是否在节点范围内
                // 使用绝对坐标进行比较
                if (
                    absoluteX >= node.absoluteRenderBounds.x && 
                    absoluteX <= node.absoluteRenderBounds.x + nodeWidth && 
                    absoluteY >= node.absoluteRenderBounds.y && 
                    absoluteY <= node.absoluteRenderBounds.y + nodeHeight
                ) {
                    console.log('匹配节点:', node.id);
                    matchingNodes.push(node);
                }
            }
            
            console.log('匹配的节点:', matchingNodes);
            
            // 如果找到匹配的节点
            if (matchingNodes.length > 0) {
                // 按照Z轴顺序排序（在previewImages数组中的索引越大，Z轴越高）
                // 倒序排列，使得最高层的节点（索引越大）在数组前面
                matchingNodes.sort((a, b) => {
                    const indexA = this.previewImages.findIndex(img => img.nodeId === a.id);
                    const indexB = this.previewImages.findIndex(img => img.nodeId === b.id);
                    return indexB - indexA; // 倒序，使得Z轴最高的节点在数组前面
                });
                
                // 将匹配节点的ID数组提取出来
                const matchingNodeIds = matchingNodes.map(node => node.id);
                
                // 检查是否是在同一位置再次点击（在短时间内）
                const now = Date.now();
                const samePosition = Math.abs(clickX - this.lastClickPosition.x) < 5 && Math.abs(clickY - this.lastClickPosition.y) < 5;
                const shortTimeSpan = (now - this.lastClickPosition.timestamp) < 1000; // 1秒内的点击认为是同一位置
                const sameNodes = JSON.stringify(matchingNodeIds) === JSON.stringify(this.lastClickPosition.matchingNodeIds);
                
                // 如果是同一位置的重复点击，并且匹配到的节点相同，则循环选择
                if (samePosition && shortTimeSpan && sameNodes && matchingNodes.length > 1) {
                    console.log('在同一位置重复点击，循环选择节点');
                    
                    // 计算下一个要选择的节点索引
                    const nextIndex = (this.lastClickPosition.currentIndex + 1) % matchingNodes.length;
                    const nextNodeId = matchingNodes[nextIndex].id;
                    
                    // 更新当前索引
                    this.lastClickPosition.currentIndex = nextIndex;
                    this.lastClickPosition.timestamp = now;
                    
                    // 查找节点路径并展开树
                    this.findNodePathAndExpand(nextNodeId);
                    
                    // 选择节点
                    this.handleNodeSelect(nextNodeId);
                } else {
                    // 如果是新的点击位置或者匹配到的节点不同，则选择最顶层的节点
                    console.log('新的点击位置或新的节点列表，选择最顶层节点');
                    
                    // 默认选择Z轴最高的节点（现在是数组中的第一个）
                    const selectedNodeId = matchingNodes[0].id;
                    
                    // 更新点击位置记录
                    this.lastClickPosition = {
                        x: clickX,
                        y: clickY,
                        matchingNodeIds: matchingNodeIds,
                        currentIndex: 0, // 记录选中的是第一个节点（最顶层）
                        timestamp: now
                    };
                    
                    // 查找节点路径并展开树
                    this.findNodePathAndExpand(selectedNodeId);
                    
                    // 选择节点
                    this.handleNodeSelect(selectedNodeId);
                }
            } else {
                // 没有匹配节点，重置点击位置记录
                this.lastClickPosition = {
                    x: null,
                    y: null,
                    matchingNodeIds: [],
                    currentIndex: -1,
                    timestamp: 0
                };
            }
        },
        
    // 查找节点路径并展开树
    findNodePathAndExpand(nodeId) {
        // 查找节点的完整路径
        const path = this.findNodePath(nodeId);
        if (!path.length) return;
        
        console.log('节点路径:', path);
        
        // 更新展开的节点列表（包括整个路径，确保所有父节点都展开）
        this.expandedKeys = [...path];
        
        // 确保树组件已经渲染
        this.$nextTick(() => {
            if (this.$refs.nodeTree) {
                // 设置当前选中的节点
                this.$refs.nodeTree.setCurrentKey(nodeId);
                
                // 确保所有父节点都展开
                path.forEach(id => {
                    const node = this.$refs.nodeTree.store.nodesMap[id];
                    if (node) {
                        node.expanded = true;
                    }
                });
                
                // 滚动到选中的节点
                const node = this.$refs.nodeTree.getNode(nodeId);
                if (node) {
                    // 使用Element UI的树控件方法滚动到节点位置
                    // 注意：Element UI的Tree组件没有内置scrollToNode方法，需要手动实现滚动
                    const nodeElement = this.$refs.nodeTree.$el.querySelector(`[data-key="${nodeId}"]`);
                    if (nodeElement) {
                        nodeElement.scrollIntoView({ behavior: 'smooth', block: 'center' });
                    }
                }
            }
        });
    },
        
        // 查找节点的完整路径
        findNodePath(nodeId, currentNodes = null, path = [], isRefNode = false) {
            // 如果没有指定节点列表，先尝试在主节点中查找
            if (!currentNodes) {
                const mainNode = this.nodes.find(n => n.id === nodeId);
                if (mainNode) {
                    return this.findNodePath(nodeId, this.nodes, path, false);
                }
                
                // 如果在主节点中找不到，尝试在依赖节点中查找
                for (const refNodeId of this.refNodes) {
                    const refNodeDetail = this.refNodeDetails[refNodeId];
                    if (refNodeDetail && refNodeDetail.refNodeTreeData) {
                        const refNode = refNodeDetail.refNodeTreeData.find(n => n.id === nodeId);
                        if (refNode) {
                            // 在依赖节点树中查找路径
                            const refPath = this.findNodePath(nodeId, refNodeDetail.refNodeTreeData, path, true);
                            // 如果是依赖节点，在路径前面添加依赖节点分组ID
                            if (refPath.length > 0) {
                                refPath.unshift('__ref_nodes__');
                            }
                            return refPath;
                        }
                    }
                }
                return [];
            }
            
            // 找到当前节点
            const node = currentNodes.find(n => n.id === nodeId);
            if (!node) return [];
            
            // 将当前节点添加到路径
            path.unshift(nodeId);
            
            // 如果没有父节点，返回路径
            if (!node.parent_id) return path;
            
            // 递归查找父节点
            return this.findNodePath(node.parent_id, currentNodes, path, isRefNode);
        },
        
    // 删除节点修改信息
    removeNodeModifications() {
        if (!this.currentNode) return;
        
        // 添加二次确认弹窗
        this.$confirm('确定要删除该节点的自定义设置吗？', '提示', {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning'
        }).then(() => {
            // 发送请求删除节点设置
            axios.delete(`/figma/project/${this.project.id}/node/${this.currentNode.id}/settings`)
                .then(() => {
                    // 保存当前展开的节点和选中的节点
                    const currentExpandedKeys = [...this.expandedKeys];
                    const currentSelectedNodeId = this.currentNode.id;
                    
                    // 保存当前属性面板的展开状态
                    const currentSectionExpanded = {...this.sectionExpanded};
                    
                    // 更新本地数据
                    if (this.currentNode.modifys) {
                        // 保存节点引用
                        const nodeId = this.currentNode.id;
                        
                        // 删除修改信息
                        delete this.currentNode.modifys;
                        // 强制Vue更新当前节点对象
                        this.currentNode = {...this.currentNode};
                        
                        // 使用更新标签方法更新节点显示，而不是重新初始化整个树
                        // 找到对应的树节点并更新
                        if (this.$refs.nodeTree) {
                            const treeNode = this.$refs.nodeTree.getNode(nodeId);
                            if (treeNode) {
                                // 更新节点标签为原始名称（优先使用originalName，如果没有则使用name）
                                const originalName = this.currentNode.originalName || this.currentNode.name || this.currentNode.id;
                                
                                // 使用Vue.set确保响应式更新
                                this.$set(treeNode.data, 'label', originalName);
                                this.$set(treeNode.data, 'name', originalName);
                                this.$set(treeNode.data, 'modifys', null);
                                
                                // 强制Vue更新树节点显示
                                this.$forceUpdate();
                            }
                        }
                        
                        // 同时更新nodes数组中的数据
                        const nodeInArray = this.nodes.find(n => n.id === nodeId);
                        if (nodeInArray && nodeInArray.modifys) {
                            delete nodeInArray.modifys;
                        }
                    }
                    
                    // 重置表单为默认值
                    this.propertyForm = {
                        img_ext: 'png',
                        components: [],
                        rename: '',
                        ignore: false,
                        res_mode: 'attach',
                        img_name: '',
                        img_id: '',
                        horizontal: 'CENTER',
                        vertical: "CENTER",
                        parent_id: ''
                    };
                    
                    // 隐藏自定义设置
                    this.showCustomSettings = false;
                    
                    // 刷新属性面板
                    this.$nextTick(() => {
                        // 重新加载节点设置，确保属性面板与后端数据同步
                        // 传递保存的展开状态
                        this.loadNodeSettings(this.currentNode, currentSectionExpanded);
                        
                        // 修改：不调用findNodePathAndExpand，直接设置展开状态和选中节点
                        setTimeout(() => {
                            // 恢复节点树的展开状态
                            this.expandedKeys = currentExpandedKeys;
                            
                            // 确保树视图更新
                            this.$nextTick(() => {
                                if (this.$refs.nodeTree) {
                                    // 设置当前选中的节点
                                    this.$refs.nodeTree.setCurrentKey(currentSelectedNodeId);
                                    
                                    // 滚动到选中的节点
                                    const nodeElement = this.$refs.nodeTree.$el.querySelector(`[data-key="${currentSelectedNodeId}"]`);
                                    if (nodeElement) {
                                        nodeElement.scrollIntoView({ behavior: 'smooth', block: 'center' });
                                    }
                                }
                            });
                        }, 100); // 给树视图一点时间更新
                    });
                    
                    this.$message.success('已删除自定义设置');
                })
                .catch(error => {
                    console.error('删除节点设置失败:', error);
                    this.$message.error('删除节点设置失败');
                });
        }).catch(() => {
            // 用户取消操作
        });
    },
        
        // 根据figma节点布局属性计算默认约束
        calculateDefaultConstraints(node) {
            if (!node) return { horizontal: 'CENTER', vertical: 'CENTER' };
            
            const layoutMode = node.layoutMode;
            const layoutSizingHorizontal = node.layoutSizingHorizontal;
            const layoutSizingVertical = node.layoutSizingVertical;
            
            // 如果是自动布局容器
            if (layoutMode === 'HORIZONTAL' || layoutMode === 'VERTICAL') {
                // 水平自动布局
                if (layoutMode === 'HORIZONTAL') {
                    if (layoutSizingVertical === 'FILL') {
                        return { horizontal: 'CENTER', vertical: 'TOP_BOTTOM' };
                    } else if (layoutSizingHorizontal === 'FILL') {
                        return { horizontal: 'LEFT_RIGHT', vertical: 'CENTER' };
                    }
                }
                // 垂直自动布局
                else if (layoutMode === 'VERTICAL') {
                    if (layoutSizingHorizontal === 'FILL') {
                        return { horizontal: 'LEFT_RIGHT', vertical: 'CENTER' };
                    } else if (layoutSizingVertical === 'FILL') {
                        return { horizontal: 'CENTER', vertical: 'TOP_BOTTOM' };
                    }
                }
                
                // 默认自动布局使用居中
                return { horizontal: 'CENTER', vertical: 'CENTER' };
            }
            
            // 检查是否有拉伸约束
            if (layoutSizingHorizontal === 'FILL' && layoutSizingVertical === 'FILL') {
                return { horizontal: 'LEFT_RIGHT', vertical: 'TOP_BOTTOM' };
            } else if (layoutSizingHorizontal === 'FILL') {
                return { horizontal: 'LEFT_RIGHT', vertical: 'CENTER' };
            } else if (layoutSizingVertical === 'FILL') {
                return { horizontal: 'CENTER', vertical: 'TOP_BOTTOM' };
            }
            
            // 检查constraints属性（如果存在）
            if (node.constraints) {
                return {
                    horizontal: node.constraints.horizontal || 'CENTER',
                    vertical: node.constraints.vertical || 'CENTER'
                };
            }
            
            // 默认返回居中
            return { horizontal: 'CENTER', vertical: 'CENTER' };
        },
        
        // 显示所有设置
        showAllSettings() {
            this.showCustomSettings = true;
            this.showLayoutSettings = true;
            this.showExportSettings = true;
            
            // 同时展开所有部分
            this.sectionExpanded.controlSettings = true;
            this.sectionExpanded.layoutSettings = true;
            this.sectionExpanded.exportSettings = true;
            
            // 如果节点类型为TEXT，则不设置res_mode
            if (this.currentNode && this.currentNode.type === 'TEXT') {
                this.propertyForm.res_mode = '';
            }
            
            // 根据figma节点布局属性设置默认约束
            if (this.currentNode) {
                const defaultConstraints = this.calculateDefaultConstraints(this.currentNode);
                this.propertyForm.horizontal = defaultConstraints.horizontal;
                this.propertyForm.vertical = defaultConstraints.vertical;
                
                // 调试信息（可选）
                console.log('自动设置锚点默认值:', {
                    nodeId: this.currentNode.id,
                    nodeName: this.currentNode.name,
                    layoutMode: this.currentNode.layoutMode,
                    layoutSizingHorizontal: this.currentNode.layoutSizingHorizontal,
                    layoutSizingVertical: this.currentNode.layoutSizingVertical,
                    layoutWrap: this.currentNode.layoutWrap,
                    constraints: this.currentNode.constraints,
                    defaultAnchor: defaultAnchor
                });
            }
        },
        
        // 切换部分展开/收起状态
        toggleSection(section) {
            this.sectionExpanded[section] = !this.sectionExpanded[section];
        },
        
        // 加载节点设置
        loadNodeSettings(node, previousSectionExpanded = null) {
            // 更新节点JSON显示
            this.updateNodeJsonDisplay();
            
            // 先设置默认值，确保属性面板始终有值
            this.propertyForm = {
                img_ext: 'png',
                components: [],
                rename: '',
                ignore: false,
                res_mode: 'attach',
                img_name: '',
                img_id: '',
                horizontal: 'CENTER',
                vertical: "CENTER",
                parent_id: ''
            };
            
            // 重置自定义设置状态
            this.showCustomSettings = false;
            this.showLayoutSettings = false;
            this.showExportSettings = false;
            
            // 如果节点已经有修改信息，直接使用
            if (node.modifys) {
                // 如果有修改信息，显示自定义设置
                this.showCustomSettings = true;
                this.showLayoutSettings = true;
                this.showExportSettings = true;
                
                this.propertyForm = {
                    img_ext: node.modifys.img_ext || 'png',
                    components: node.modifys.components || [],
                    rename: node.modifys.rename || '',
                    ignore: node.modifys.ignore || false,
                    res_mode: node.modifys.res_mode || 'attach',
                    img_name: node.modifys.img_name || '',
                    img_id: node.modifys.img_id || '',
                    horizontal: node.modifys.horizontal || 'CENTER',
                    vertical: node.modifys.vertical || 'CENTER',
                    parent_id: node.modifys.parent_id || ''
                };
                
                // 恢复展开状态（如果提供了之前的状态）
                if (previousSectionExpanded) {
                    this.sectionExpanded = previousSectionExpanded;
                }
                
                // 强制刷新属性面板视图
                this.$forceUpdate();
                return;
            }
            
            // 否则从API获取
            axios.get(`/figma/project/${this.project.id}/node/${node.id}/settings`)
                .then(response => {
                    const settings = response.data.settings || {};
                    
                    // 如果返回的设置不为空，则更新属性表单
                    if (Object.keys(settings).length > 0) {
                        this.propertyForm = {
                            img_ext: settings.img_ext || 'png',
                            components: settings.components || [],
                            rename: settings.rename || '',
                            ignore: settings.ignore || false,
                            res_mode: settings.res_mode || 'attach',
                            img_name: settings.img_name || '',
                            img_id: settings.img_id || '',
                            horizontal: settings.horizontal || 'CENTER',
                            vertical: settings.vertical || 'CENTER',
                            parent_id: settings.parent_id || ''
                        };
                        
                        // 如果有设置，显示自定义设置
                        this.showCustomSettings = true;
                        this.showLayoutSettings = true;
                        this.showExportSettings = true;
                    }
                    
                    // 强制刷新属性面板视图
                    this.$forceUpdate();
                    
                    // 恢复展开状态（如果提供了之前的状态）
                    if (previousSectionExpanded) {
                        this.sectionExpanded = previousSectionExpanded;
                    }
                })
                .catch(error => {
                    // 如果是404错误，说明节点设置不存在，这是正常的，静默处理
                    if (error.response && error.response.status === 404) {
                        // 节点没有设置，使用默认值，不显示错误
                        // 即使出错也要尝试恢复展开状态
                        if (previousSectionExpanded) {
                            this.sectionExpanded = previousSectionExpanded;
                        }
                        return;
                    }
                    // 其他错误才记录和显示
                    console.error('获取节点设置失败:', error);
                    this.$message.error('获取节点设置失败');
                    
                    // 即使出错也要尝试恢复展开状态
                    if (previousSectionExpanded) {
                        this.sectionExpanded = previousSectionExpanded;
                    }
                });
        },
        
    // 实时保存节点设置的通用方法
    autoSaveNodeSettings() {
        if (!this.currentNode) return;
        
        // 静默保存，不显示成功消息
        axios.post(`/figma/project/${this.project.id}/node/${this.currentNode.id}/settings`, this.propertyForm)
            .then(() => {
                // 更新本地节点的修改信息
                if (!this.currentNode.modifys) {
                    this.currentNode.modifys = {};
                }
                Object.assign(this.currentNode.modifys, this.propertyForm);
                
                // 更新nodeModifys对象
                this.nodeModifys[this.currentNode.id] = {...this.propertyForm};
                
                // 立即更新节点对象的modifys属性
                const targetNode = this.nodes.find(node => node.id === this.currentNode.id);
                if (targetNode) {
                    targetNode.modifys = {...this.propertyForm};
                }
                
                // 只更新当前节点的树形显示，不重新构建整个树
                this.updateTreeNodeLabel(this.currentNode.id);
                
                // 强制Vue更新当前节点对象，触发hasNodeModifications重新计算
                this.currentNode = {...this.currentNode};
            })
            .catch(error => {
                console.error('自动保存节点设置失败:', error);
                this.$message.error('保存失败，请重试');
            });
    },
    
    // 节点启用开关变化处理
    onNodeEnabledChange(value) {
        // value为true表示启用，false表示禁用（忽略）
        this.propertyForm.ignore = !value;
        this.autoSaveNodeSettings();
    },
    
    // 自定义名称变化处理
    onRenameChange(value) {
        // 使用防抖，避免频繁保存
        clearTimeout(this.renameTimer);
        this.renameTimer = setTimeout(() => {
            this.autoSaveNodeSettings();
        }, 500);
    },
    
    // 水平约束变化处理
    onHorizontalChange(value) {
        this.autoSaveNodeSettings();
    },
    
    // 垂直约束变化处理
    onVerticalChange(value) {
        this.autoSaveNodeSettings();
    },
    
    // 父级ID变化处理
    onParentIdChange(value) {
        clearTimeout(this.parentIdTimer);
        this.parentIdTimer = setTimeout(() => {
            this.autoSaveNodeSettings();
        }, 500);
    },
    
    // 资源模式变化处理
    onResModeChange(value) {
        this.autoSaveNodeSettings();
    },
    
    // 图片名称变化处理
    onImgNameChange(value) {
        clearTimeout(this.imgNameTimer);
        this.imgNameTimer = setTimeout(() => {
            this.autoSaveNodeSettings();
        }, 500);
    },
    
    // 图片ID变化处理
    onImgIdChange(value) {
        clearTimeout(this.imgIdTimer);
        this.imgIdTimer = setTimeout(() => {
            this.autoSaveNodeSettings();
        }, 500);
    },
    
    // 图片格式变化处理
    onimg_extChange(value) {
        this.autoSaveNodeSettings();
    },
    
    // 组件列表变化处理
    onComponentsChange(value) {
        this.autoSaveNodeSettings();
    },

    // 保存节点设置
    saveNodeSettings() {
        if (!this.currentNode) return;
        
        // 保存当前属性面板的展开状态
        const currentSectionExpanded = {...this.sectionExpanded};
        
        axios.post(`/figma/project/${this.project.id}/node/${this.currentNode.id}/settings`, this.propertyForm)
            .then(() => {
                // 保存当前展开的节点和选中的节点
                const currentExpandedKeys = [...this.expandedKeys];
                const currentSelectedNodeId = this.currentNode.id;
                
                // 更新本地节点的修改信息
                if (!this.currentNode.modifys) {
                    this.currentNode.modifys = {};
                }
                Object.assign(this.currentNode.modifys, this.propertyForm);
                
                // 强制Vue更新当前节点对象，触发hasNodeModifications重新计算
                this.currentNode = {...this.currentNode};
                
                // 更新树节点的显示名称而不重新初始化整个树
                this.updateTreeNodeLabel(this.currentNode.id);
                
                // 显示自定义设置
                this.showCustomSettings = true;
                this.showLayoutSettings = true;
                this.showExportSettings = true;
                
                // 恢复属性面板的展开状态
                this.$nextTick(() => {
                    this.sectionExpanded = currentSectionExpanded;
                });
                
                // 显示成功消息
                this.$message.success('节点设置保存成功');
            })
            .catch(error => {
                // 即使出错也尝试恢复展开状态
                this.$nextTick(() => {
                    this.sectionExpanded = currentSectionExpanded;
                });
                
                this.$message.error(error.response?.data?.error || '保存节点设置失败');
            });
    },
        
    // 更新树节点的显示名称
    updateTreeNodeLabel(nodeId, parentNodes = this.treeData) {
        for (let i = 0; i < parentNodes.length; i++) {
            if (parentNodes[i].id === nodeId) {
                // 找到节点，更新标签
                const node = this.nodes.find(n => n.id === nodeId);
                if (node) {
                    // 保存原始名称（优先使用originalName，如果没有则使用name）
                    const originalName = node.originalName || node.name || node.id;
                    // 更新树节点标签，根据节点是否有修改信息来决定显示的标签
                    if (node.modifys) {
                        parentNodes[i].label = node.modifys.rename || node.modifys.customName || node.name || node.id;
                        // 更新节点类型标记
                        parentNodes[i].modifys = node.modifys;
                    } else {
                        // 如果没有修改信息，使用原始名称
                        parentNodes[i].label = node.name || node.id;
                        // 移除修改标记
                        parentNodes[i].modifys = null;
                    }
                    // 确保原始名称被保存
                    parentNodes[i].name = originalName;
                    
                    // 尝试直接更新树组件中的节点，避免触发重新渲染
                    if (this.$refs.nodeTree) {
                        try {
                            // 获取树节点实例
                            const treeNode = this.$refs.nodeTree.getNode(nodeId);
                            if (treeNode) {
                                // 直接更新节点的显示文本
                                treeNode.label = parentNodes[i].label;
                                if (treeNode.data) {
                                    treeNode.data.label = parentNodes[i].label;
                                    treeNode.data.modifys = parentNodes[i].modifys;
                                    treeNode.data.name = parentNodes[i].name; // 确保原始名称也被更新
                                }
                                
                                // 更新底层数据
                                parentNodes[i] = {...parentNodes[i]};
                                
                                // 强制重新渲染这个节点
                                this.$forceUpdate();
                                return true;
                            }
                        } catch (error) {
                            console.warn('直接更新树节点失败，使用备用方法:', error);
                        }
                    }
                    
                    // 备用方法：保存展开状态后更新
                    const currentExpandedKeys = [...this.expandedKeys];
                    this.$set(parentNodes, i, parentNodes[i]);
                    
                    this.$nextTick(() => {
                        if (currentExpandedKeys.length > 0) {
                            this.expandedKeys = currentExpandedKeys;
                        }
                    });
                }
                return true;
            }
            
            // 递归检查子节点
            if (parentNodes[i].children && parentNodes[i].children.length) {
                if (this.updateTreeNodeLabel(nodeId, parentNodes[i].children)) {
                    return true;
                }
            }
        }
        
        return false;
    },
        
        // 直接开始导出，不显示对话框
        showExportDialog() {
            this.startExport();
        },
        
        // 开始导出
        startExport() {
            this.exportStatus = 'processing';
            this.exportProgress = 0;
            console.log('开始导出，设置状态:', this.exportStatus, '进度:', this.exportProgress);
            
            // 发送导出请求，传递默认图片格式和缩放比例
            axios.post(`/figma/project/${this.project.id}/export`, {
                format: this.defaultImageFormat,
                scale: this.defaultImageScale
            })
                .then(response => {
                    this.exportJobId = response.data.job_id;
                    console.log('导出任务创建成功，jobId:', this.exportJobId);
                    this.checkExportStatus();
                    this.$message.success('导出任务已开始处理');
                })
                .catch(error => {
                    this.exportStatus = 'failed';
                    console.error('导出任务创建失败:', error);
                    this.$message.error(error.response?.data?.error || '开始导出失败');
                });
        },
        
        // 检查导出状态
        checkExportStatus() {
            if (!this.exportJobId) return;
            
            const checkStatus = () => {
                axios.get(`/figma/export/${this.exportJobId}`)
                    .then(response => {
                        const { status, progress } = response.data;
                        console.log('检查导出状态:', status, '进度:', progress);
                        this.exportStatus = status;
                        this.exportProgress = progress;
                        
                        if (status === 'pending' || status === 'processing') {
                            if (status === 'pending') {
                                console.log('任务排队中，等待处理...');
                            }
                            setTimeout(checkStatus, 1000);
                        } else if (status === 'completed') {
                            this.$message.success('导出完成，正在下载文件...');
                            // 自动下载文件
                            this.downloadExport();
                        } else if (status === 'failed') {
                            this.$message.error('导出失败');
                        } else if (status === 'cancelled') {
                            this.$message.warning('导出任务已被取消');
                        }
                    })
                    .catch(() => {
                        this.exportStatus = 'failed';
                        console.error('检查导出状态失败');
                        this.$message.error('检查导出状态失败');
                    });
            };
            
            checkStatus();
        },
        
        // 下载导出文件
        downloadExport() {
            if (this.exportStatus !== 'completed') return;
            
            window.location.href = `/figma/export/${this.exportJobId}/download`;
        },
        
        // 检查活跃的导出任务
        checkActiveExportJob() {
            if (!this.project || !this.project.id) return;
            
            axios.get(`/figma/project/${this.project.id}/export/active`)
                .then(response => {
                    if (response.data.has_active_job) {
                        // 有活跃任务，恢复导出状态
                        this.exportJobId = response.data.job_id;
                        this.exportStatus = response.data.status;
                        this.exportProgress = response.data.progress;
                        
                        // 如果任务还在处理中，开始轮询
                        if (response.data.status === 'processing') {
                            this.checkExportStatus();
                        }
                    }
                })
                .catch(error => {
                    console.error('检查活跃导出任务失败:', error);
                });
        },
        
    // 获取当前节点的图片URL
    getCurrentNodeImage(scale = 1.0) { // 缩放比例参数，默认为1.0（标准清晰度）
        // 如果没有当前节点，确保缩略图隐藏并返回空
        if (!this.currentNode) {
            this.$nextTick(() => {
                const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
                if (minimapContainer) {
                    minimapContainer.style.display = 'none';
                }
                
                const minimapImage = minimapContainer ? minimapContainer.querySelector('.minimap-image') : null;
                if (minimapImage) {
                    minimapImage.style.display = 'none';
                }
            });
            return '';
        }
        
        // 如果是依赖节点，从 refPreviewImages 中查找
        if (this.currentNode.isRefNodeSelected) {
            // 确保缩略图容器显示
            this.$nextTick(() => {
                const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
                if (minimapContainer && this.refPreviewImages.length > 0) {
                    minimapContainer.style.display = 'block';
                }
            });
            
            // 查找当前依赖节点对应的图片
            const currentRefImage = this.refPreviewImages.find(img => img.nodeId === this.currentNode.id);
            if (currentRefImage && currentRefImage.src) {
                // 在下一个渲染周期，添加图片加载事件监听器
                this.$nextTick(() => {
                    const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
                    const minimapImage = minimapContainer ? minimapContainer.querySelector('.minimap-image') : null;
                    if (minimapImage) {
                        // 先隐藏图片
                        minimapImage.style.display = 'none';
                        
                        // 当图片加载完成后显示
                        minimapImage.onload = function() {
                            minimapImage.style.display = 'block';
                            
                            // 确保图片清晰显示，移除可能导致模糊的CSS效果
                            minimapImage.style.imageRendering = 'crisp-edges'; // 为支持的浏览器添加清晰渲染
                        };
                        
                        // 创建一个新的Image对象来预加载图片
                        const preloadImg = new Image();
                        preloadImg.onload = () => {
                            console.log('[依赖节点] 缩略图加载完成:', currentRefImage.src);
                            // 确保图片已加载完成后再显示
                            if (minimapImage) {
                                minimapImage.src = currentRefImage.src;
                                minimapImage.style.display = 'block';
                            }
                        };
                        // 开始加载图片
                        preloadImg.src = currentRefImage.src;
                    }
                });
                
                // 直接返回图片URL，不添加时间戳
                return currentRefImage.src;
            }
            
            // 如果没有找到当前依赖节点的预览图，构建图片URL
            return `/figma/image/${this.project.id}/${this.currentNode.id}?scale=${scale}&format=${this.defaultImageFormat}`;
        }
        
        // 主节点处理逻辑
        // 如果没有预览图，确保缩略图隐藏并返回空
        if (this.previewImages.length === 0) {
            this.$nextTick(() => {
                const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
                if (minimapContainer) {
                    minimapContainer.style.display = 'none';
                }
                
                const minimapImage = minimapContainer ? minimapContainer.querySelector('.minimap-image') : null;
                if (minimapImage) {
                    minimapImage.style.display = 'none';
                }
            });
            return '';
        }
        
        // 确保缩略图容器显示
        this.$nextTick(() => {
            const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
            if (minimapContainer) {
                minimapContainer.style.display = 'block';
            }
        });
        
        // 查找当前节点对应的图片
        const currentImage = this.previewImages.find(img => img.nodeId === this.currentNode.id);
        if (currentImage && currentImage.src) {
            // 在下一个渲染周期，添加图片加载事件监听器
            this.$nextTick(() => {
                const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
                const minimapImage = minimapContainer ? minimapContainer.querySelector('.minimap-image') : null;
                if (minimapImage) {
                    // 先隐藏图片
                    minimapImage.style.display = 'none';
                    
                    // 当图片加载完成后显示
                    minimapImage.onload = function() {
                        minimapImage.style.display = 'block';
                        
                        // 确保图片清晰显示，移除可能导致模糊的CSS效果
                        minimapImage.style.imageRendering = 'crisp-edges'; // 为支持的浏览器添加清晰渲染
                    };
                    
                    // 创建一个新的Image对象来预加载图片
                    const preloadImg = new Image();
                    preloadImg.onload = () => {
                        console.log('缩略图加载完成:', currentImage.src);
                        // 确保图片已加载完成后再显示
                        if (minimapImage) {
                            minimapImage.src = currentImage.src;
                            minimapImage.style.display = 'block';
                        }
                    };
                    // 开始加载图片
                    preloadImg.src = currentImage.src;
                }
            });
            
            // 直接返回图片URL，不添加时间戳
            return currentImage.src;
        }
        
        // 如果没有找到当前节点的预览图，隐藏缩略图容器
        this.$nextTick(() => {
            const minimapContainer = document.querySelector('.minimap-container:not(.filtered-minimap)');
            if (minimapContainer) {
                minimapContainer.style.display = 'none';
            }
        });
        
        // 如果没有找到，构建图片URL（不添加时间戳），并增加scale参数提高清晰度，使用选择的图片格式
        return `/figma/image/${this.project.id}/${this.currentNode.id}?scale=${scale}&format=${this.defaultImageFormat}`;
    },
    
    // 获取过滤后的节点图片URL（排除子树中有修改且资源模式不是跟随父级的节点）
    getFilteredNodeImage(scale = 1.0) {
        // 检查是否应该获取过滤图片
        // 只有在过滤预览展开时才获取，避免不必要的网络请求
        if (!this.isFilteredPreviewExpanded) {
            console.log('过滤预览未展开，不获取过滤图片');
            return '';
        }
        
        if (!this.currentNode || this.previewImages.length === 0) {
            return '';
        }
        
        console.log('获取过滤图片，过滤预览已展开');
        
        // 构建过滤后的图片URL，添加excludeModified参数
        // 这个参数会告诉后端API排除子树中有修改且资源模式不是跟随父级的节点
        // 添加时间戳参数，防止浏览器缓存，并使用选择的图片格式
        const timestamp = new Date().getTime();
        return `/figma/image/${this.project.id}/${this.currentNode.id}?scale=${scale}&excludeModified=true&format=${this.defaultImageFormat}&t=${timestamp}`;
    },
    
    // 获取批量过滤预览图片（返回节点ID到图片路径的映射）
    async getFilteredNodeImages(scale = 1.0) {
        // 检查是否应该获取过滤图片
        // 只有在过滤预览展开时才获取，避免不必要的网络请求
        if (!this.isFilteredPreviewExpanded) {
            console.log('过滤预览未展开，不获取批量过滤图片');
            return {};
        }
        
        if (!this.currentNode || this.previewImages.length === 0) {
            return {};
        }
        
        console.log('获取批量过滤图片，过滤预览已展开');
        
        // 过滤预览功能必须使用Figma Plugin API
        if (!window.figmaPluginBridge || !window.figmaPluginBridge.isAvailable()) {
            console.error('过滤预览功能需要Figma Plugin支持，但插件不可用');
            this.$message.error('过滤预览功能需要在Figma环境中使用，请确保Figma Plugin已正确安装和配置');
            return {};
        }
        
        console.log('使用Figma Plugin API获取批量预览图片（强制模式）');
        try {
            // 收集所有需要预览的节点ID
            const nodeIds = this.previewImages.map(img => img.nodeId);
            
            // 调用Figma Plugin API
            const result = await window.figmaPluginBridge.previewNodes(
                nodeIds, 
                scale, 
                this.defaultImageFormat.toUpperCase()
            );
            
            if (result && result.images) {
                console.log(`通过Figma Plugin成功获取 ${result.count} 个预览图片`);
                
                // 转换格式以匹配后端API的返回格式
                const images = {};
                Object.entries(result.images).forEach(([nodeId, nodeData]) => {
                    images[nodeId] = nodeData.imageUrl; // 使用base64 data URL
                });
                
                return images;
            } else {
                console.warn('Figma Plugin返回的数据格式不正确');
                throw new Error('Invalid plugin response');
            }
        } catch (error) {
            console.error('Figma Plugin API调用失败:', error);
            this.$message.error('获取过滤预览失败，请确保Figma Plugin正常工作');
            return {};
        }
    },
        
        // 更新节点JSON显示
        updateNodeJsonDisplay() {
            if (!this.currentNode) {
                this.nodeJsonString = '';
                return;
            }
            
            console.log('更新节点JSON显示:', {
                nodeId: this.currentNode.id,
                includeChildren: this.includeChildrenInJson,
                hasChildren: !!this.currentNode.children,
                childrenCount: this.currentNode.children ? this.currentNode.children.length : 0
            });
            
            // 直接使用节点本身，包含所有原始数据
            let nodeData = this.currentNode;
            
            // 如果不包含子节点数据，则删除children字段
            if (!this.includeChildrenInJson) {
                // 创建一个不包含children的深拷贝
                const processedData = this.processNodeDataForDisplay(nodeData);
                this.nodeJsonString = this.formatJson(processedData);
                console.log('不包含子节点，已处理数据');
            } else {
                // 包含完整数据
                this.nodeJsonString = this.formatJson(nodeData);
                console.log('包含完整数据，包括子节点');
            }
        },
        
        // 显示JSON预览弹窗
        showJsonPreviewDialog() {
            this.jsonPreviewDialogVisible = true;
            // 初始化JSON数据显示
            this.updateNodeJsonDisplay();
        },
        
        // 处理节点数据用于显示
        processNodeDataForDisplay(nodeData) {
            // 创建一个深拷贝，避免修改原始数据
            const nodeCopy = JSON.parse(JSON.stringify(nodeData));
            
            // 如果存在children字段，删除它
            if (nodeCopy.children) {
                delete nodeCopy.children;
            }
            return nodeCopy;
        },
        
        // 格式化JSON数据
        formatJson(data) {
            try {
                return JSON.stringify(data, null, 2);
            } catch (error) {
                console.error('格式化JSON出错:', error);
                return '无法格式化JSON数据';
            }
        },
        
        // 拷贝节点JSON数据
        copyNodeJson() {
            if (!this.nodeJsonString) {
                this.$message.warning('没有可拷贝的内容');
                return;
            }
            
            // 使用现代浏览器的 Clipboard API
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(this.nodeJsonString).then(() => {
                    this.$message.success('已拷贝到剪贴板');
                }).catch(err => {
                    console.error('拷贝失败:', err);
                    // 降级方案：使用传统方法
                    this.fallbackCopyText(this.nodeJsonString);
                });
            } else {
                // 降级方案：使用传统方法
                this.fallbackCopyText(this.nodeJsonString);
            }
        },
        
        // 降级拷贝方案（兼容旧浏览器）
        fallbackCopyText(text) {
            // 创建临时textarea元素
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            textarea.style.left = '-999999px';
            textarea.style.top = '-999999px';
            document.body.appendChild(textarea);
            textarea.focus();
            textarea.select();
            
            try {
                const successful = document.execCommand('copy');
                if (successful) {
                    this.$message.success('已拷贝到剪贴板');
                } else {
                    this.$message.error('拷贝失败，请手动选择文本进行拷贝');
                }
            } catch (err) {
                console.error('拷贝失败:', err);
                this.$message.error('拷贝失败，请手动选择文本进行拷贝');
            } finally {
                document.body.removeChild(textarea);
            }
        },

        // ========== MCP相关方法 ==========
        
        // 切换MCP面板显示
        toggleMCPPanel() {
            this.mcpPanelVisible = !this.mcpPanelVisible;
            if (this.mcpPanelVisible) {
                this.refreshMCPLogs();
                // 加载界面描述
                this.loadInterfaceDescription();
            }
        },


        // 刷新MCP调用日志
        async refreshMCPLogs() {
            try {
                const projectId = this.project?.id;
                if (!projectId) {
                    this.mcpLogs = [];
                    return;
                }
                
                const response = await fetch(`/mcp-api/logs/${projectId}?limit=100`, {
                    method: 'GET',
                    credentials: 'include'
                });
                
                if (response.ok) {
                    const data = await response.json();
                    this.mcpLogs = data.logs || [];
                    // 等待DOM更新后滚动到底部
                    this.$nextTick(() => {
                        this.scrollMCPLogsToBottom();
                    });
                } else {
                    console.error('获取MCP调用记录失败');
                    this.mcpLogs = [];
                }
            } catch (error) {
                console.error('刷新MCP调用记录失败:', error);
                this.mcpLogs = [];
            }
        },

        // 滚动MCP日志到底部
        scrollMCPLogsToBottom() {
            try {
                const logsContainer = document.querySelector('.mcp-logs');
                if (logsContainer) {
                    logsContainer.scrollTop = logsContainer.scrollHeight;
                }
            } catch (error) {
                console.error('滚动到底部失败:', error);
            }
        },

        // 格式化时间显示
        formatTime(timeStr) {
            if (!timeStr) return '未知';
            try {
                const date = new Date(timeStr);
                return date.toLocaleString('zh-CN');
            } catch (error) {
                return timeStr;
            }
        },

        // 获取节点优化提示词
        getNodeOptimizationPrompt() {
            if (!this.project || !this.project.id) {
                return '';
            }
            return `基于FigmaDeliver的mcp，获取节点修饰详细提示词，对当前项目:${this.project.id}进行节点修饰`;
        },

        // 拷贝节点优化提示词到剪贴板
        async copyNodeOptimizationPrompt() {
            if (!this.project || !this.project.id) {
                this.$message.warning('当前没有可用的项目');
                return;
            }

            let prompt = this.getNodeOptimizationPrompt();
            
            // 如果有界面描述，则添加到提示词中
            if (this.interfaceDescription && this.interfaceDescription.trim()) {
                prompt += `\n\n ## 本界面注意事项（AI需要遵守）\n${this.interfaceDescription.trim()}`;
            }
            
            try {
                // 使用现代浏览器的Clipboard API
                if (navigator.clipboard && window.isSecureContext) {
                    await navigator.clipboard.writeText(prompt);
                    this.$message.success('提示词已拷贝到剪贴板');
                    // 关闭MCP面板
                    this.mcpPanelVisible = false;
                } else {
                    // 降级方案：使用传统的document.execCommand
                    const textArea = document.createElement('textarea');
                    textArea.value = prompt;
                    textArea.style.position = 'fixed';
                    textArea.style.left = '-999999px';
                    textArea.style.top = '-999999px';
                    document.body.appendChild(textArea);
                    textArea.focus();
                    textArea.select();
                    
                    try {
                        document.execCommand('copy');
                        this.$message.success('提示词已拷贝到剪贴板');
                        // 关闭MCP面板
                        this.mcpPanelVisible = false;
                    } catch (err) {
                        console.error('拷贝失败:', err);
                        this.$message.error('拷贝失败，请手动复制');
                    } finally {
                        document.body.removeChild(textArea);
                    }
                }
            } catch (error) {
                console.error('拷贝提示词失败:', error);
                this.$message.error('拷贝失败: ' + error.message);
            }
        },

        // ========== 界面描述相关方法 ==========
        
        // 加载界面描述
        async loadInterfaceDescription() {
            if (!this.project || !this.project.id) {
                return;
            }

            try {
                const response = await axios.get(`/figma/project/${this.project.id}/interface-description`);
                if (response.data.success) {
                    this.interfaceDescription = response.data.data.interface_description || '';
                    this.interfaceDescriptionChanged = false;
                }
            } catch (error) {
                console.error('加载界面描述失败:', error);
                // 不显示错误消息，因为可能是第一次使用，没有描述
            }
        },

        // 界面描述内容变化时的处理
        onInterfaceDescriptionChange() {
            this.interfaceDescriptionChanged = true;
        },

        // 保存界面描述
        async saveInterfaceDescription() {
            if (!this.project || !this.project.id) {
                this.$message.warning('当前没有可用的项目');
                return;
            }

            if (!this.interfaceDescriptionChanged) {
                this.$message.info('界面描述没有变化');
                return;
            }

            try {
                const response = await axios.put(`/figma/project/${this.project.id}/interface-description`, {
                    interface_description: this.interfaceDescription
                });

                if (response.data.success) {
                    this.$message.success('界面描述保存成功');
                    this.interfaceDescriptionChanged = false;
                } else {
                    this.$message.error('保存失败: ' + (response.data.error || '未知错误'));
                }
            } catch (error) {
                console.error('保存界面描述失败:', error);
                this.$message.error('保存失败: ' + (error.response?.data?.error || error.message));
            }
        },

        // 切换日志项展开状态
        toggleLogExpand(logId) {
            this.$set(this.expandedLogs, logId, !this.expandedLogs[logId]);
        },

        // 清理MCP调用日志
        async clearMCPLogs() {
            this.$confirm('确定要清理所有调用记录吗？', '确认清理', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                type: 'warning'
            }).then(async () => {
                try {
                    const projectId = this.project?.id;
                    if (!projectId) {
                        this.$message.error('项目ID不存在');
                        return;
                    }
                    
                    const response = await fetch(`/mcp-api/logs/${projectId}/clear`, {
                        method: 'DELETE',
                        credentials: 'include',
                        headers: {
                            'Content-Type': 'application/json'
                        }
                    });
                    
                    if (response.ok) {
                        this.mcpLogs = [];
                        this.expandedLogs = {};
                        this.$message.success('调用记录已清理');
                    } else {
                        this.$message.error('清理失败');
                    }
                } catch (error) {
                    console.error('清理MCP调用记录失败:', error);
                    this.$message.error('清理失败: ' + error.message);
                }
            }).catch(() => {
                // 用户取消操作
            });
        },

        // 打开MCP调用记录详情页面
        openMCPLogsPage() {
            if (!this.project || !this.project.id) {
                this.$message.warning('当前没有可用的项目');
                return;
            }
            
            const url = `/mcp-logs?project=${this.project.id}`;
            window.open(url, '_blank');
        },

        // 节点搜索功能
        handleNodeSearch(keyword) {
            if (!keyword || keyword.trim() === '') {
                this.clearNodeSearch();
                return;
            }

            const searchTerm = keyword.trim().toLowerCase();
            this.searchMatchedNodes = [];
            this.currentSearchIndex = 0;

            // 递归搜索节点
            const searchInNodes = (nodes) => {
                for (const node of nodes) {
                    // 搜索节点名称、ID、自定义名称
                    const nodeName = (node.label || '').toLowerCase();
                    const nodeId = (node.id || '').toLowerCase();
                    const customName = (node.modifys?.rename || '').toLowerCase();
                    const originalName = (node.name || '').toLowerCase();

                    if (nodeName.includes(searchTerm) || 
                        nodeId.includes(searchTerm) || 
                        customName.includes(searchTerm) ||
                        originalName.includes(searchTerm)) {
                        this.searchMatchedNodes.push(node.id);
                    }

                    // 递归搜索子节点
                    if (node.children && node.children.length > 0) {
                        searchInNodes(node.children);
                    }
                }
            };

            // 开始搜索
            searchInNodes(this.treeData);

            // 如果找到结果，选中第一个
            if (this.searchMatchedNodes.length > 0) {
                this.selectSearchResult(0);
                this.$message.success(`找到 ${this.searchMatchedNodes.length} 个匹配节点`);
            } else {
                this.$message.warning('未找到匹配的节点');
            }
        },

        // 选中搜索结果
        selectSearchResult(index) {
            if (this.searchMatchedNodes.length === 0) return;

            this.currentSearchIndex = index;
            const nodeId = this.searchMatchedNodes[index];

            // 展开到目标节点
            this.expandToNode(nodeId);

            // 选中节点
            this.handleTreeNodeClick(nodeId, null);

            // 高亮树节点
            this.$nextTick(() => {
                this.$refs.nodeTree.setCurrentKey(nodeId);
            });
        },

        // 展开到指定节点
        expandToNode(nodeId) {
            const pathToNode = [];
            
            // 查找节点路径
            const findPath = (nodes, targetId, path = []) => {
                for (const node of nodes) {
                    const currentPath = [...path, node.id];
                    
                    if (node.id === targetId) {
                        return currentPath;
                    }
                    
                    if (node.children && node.children.length > 0) {
                        const found = findPath(node.children, targetId, currentPath);
                        if (found) return found;
                    }
                }
                return null;
            };

            const path = findPath(this.treeData, nodeId);
            if (path) {
                // 展开路径上的所有节点（除了最后一个目标节点）
                this.expandedKeys = path.slice(0, -1);
            }
        },

        // 清除搜索
        clearNodeSearch() {
            this.nodeSearchKeyword = '';
            this.searchMatchedNodes = [];
            this.currentSearchIndex = 0;
        },

        // 搜索框回车事件 - 循环选择下一个匹配结果
        handleSearchKeyEnter() {
            if (this.searchMatchedNodes.length === 0) return;

            // 移动到下一个匹配结果
            const nextIndex = (this.currentSearchIndex + 1) % this.searchMatchedNodes.length;
            this.selectSearchResult(nextIndex);

            // 提示当前位置
            this.$message.info(`${nextIndex + 1} / ${this.searchMatchedNodes.length}`);
        },

        // ==================== 面板宽度调整功能 ====================

        // 开始调整大小（只调整左侧树形图）
        startResize(event, type) {
            if (type !== 'left') return; // 只支持左侧拖动
            
            this.isResizing = true;
            this.resizeStartX = event.clientX;
            this.resizeStartWidth = this.treePanelWidth;

            // 添加拖动状态类
            document.body.style.cursor = 'col-resize';
            document.body.style.userSelect = 'none';
            
            event.preventDefault();
        },

        // 处理调整移动
        handleResizeMove(event) {
            if (!this.isResizing) return;

            const deltaX = event.clientX - this.resizeStartX;
            const minPanelWidth = 200; // 最小面板宽度
            const maxPanelWidth = 600; // 最大面板宽度

            // 调整左侧树形图面板
            let newTreeWidth = this.resizeStartWidth + deltaX;
            
            // 限制最小和最大宽度
            newTreeWidth = Math.max(minPanelWidth, newTreeWidth);
            newTreeWidth = Math.min(maxPanelWidth, newTreeWidth);
            
            this.treePanelWidth = newTreeWidth;
        },

        // 停止调整大小
        stopResize() {
            if (!this.isResizing) return;

            this.isResizing = false;
            
            // 移除拖动状态类
            document.body.style.cursor = '';
            document.body.style.userSelect = '';

            // 保存面板宽度到localStorage
            this.savePanelWidths();
        },

        // 保存面板宽度到localStorage
        savePanelWidths() {
            const widths = {
                treePanelWidth: this.treePanelWidth,
                propertyPanelWidth: this.propertyPanelWidth
            };
            localStorage.setItem('panelWidths', JSON.stringify(widths));
        },

        // 从localStorage加载面板宽度
        loadPanelWidths() {
            const saved = localStorage.getItem('panelWidths');
            if (saved) {
                try {
                    const widths = JSON.parse(saved);
                    this.treePanelWidth = widths.treePanelWidth || 300;
                    this.propertyPanelWidth = widths.propertyPanelWidth || 350;
                } catch (e) {
                    console.error('加载面板宽度失败:', e);
                }
            }
        },

        // ==================== 模板代码导出功能 ====================

        // 处理模板导出命令
        handleTemplateExport(command) {
            if (!this.project || !this.project.id) {
                this.$message.error('请先选择一个项目');
                return;
            }

            switch (command) {
                case 'unity-appui':
                    this.showUnityExportDialog();
                    break;
                case 'android-kt':
                    this.showAndroidExportDialog();
                    break;
                case 'ios-swift':
                    this.showSwiftExportDialog();
                    break;
                case 'web-vue':
                    this.showVueExportDialog();
                    break;
                default:
                    this.$message.info(`${command} 导出功能即将推出`);
            }
        },

        // 显示Swift导出弹窗
        showSwiftExportDialog() {
            // 重置配置为默认值
            this.swiftExportConfig = {
                imageFormat: 'png',
                imageScale: 2.0,
                codeStyle: 'uikit-autolayout',
                options: ['generateExtensions', 'generateResourceManager']
            };
            this.swiftExportPreview = '';
            this.swiftCodeFiles = {};
            this.activeCodeTab = '';
            this.swiftExportDialogVisible = true;
        },

        // 显示Swift代码预览（在新窗口中打开）
        showSwiftCodePreview() {
            if (!this.project || !this.project.id) {
                this.$message.error('请先选择一个项目');
                return;
            }

            // 构建预览页面URL，包含导出配置参数
            const params = new URLSearchParams({
                imageFormat: this.swiftExportConfig.imageFormat,
                imageScale: this.swiftExportConfig.imageScale.toString(),
                codeStyle: this.swiftExportConfig.codeStyle,
                options: this.swiftExportConfig.options.join(',')
            });

            const previewUrl = `/figma/project/${this.project.id}/swift/code-preview?${params.toString()}`;
            
            // 在新窗口中打开代码预览页面
            const previewWindow = window.open(previewUrl, '_blank', 'width=1400,height=900,scrollbars=yes,resizable=yes');
            
            if (!previewWindow) {
                this.$message.error('无法打开预览窗口，请检查浏览器弹窗设置');
                return;
            }

            // 关闭Swift导出弹窗
            this.swiftExportDialogVisible = false;
            this.$message.success('代码预览页面已在新窗口中打开');
        },

        // 生成Swift代码预览（完整版本，用于代码预览弹窗）
        async generateSwiftCodePreview() {
            if (!this.project || !this.project.id) {
                this.$message.error('请先选择一个项目');
                return;
            }

            try {
                this.generatingPreview = true;
                
                const response = await fetch(`/figma/project/${this.project.id}/swift/preview-full`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': window.csrfToken
                    },
                    body: JSON.stringify(this.swiftExportConfig)
                });

                if (!response.ok) {
                    throw new Error('生成代码预览失败');
                }

                const result = await response.json();
                
                if (result.success) {
                    this.swiftCodeFiles = result.files || {};
                    
                    // 设置默认激活的标签页
                    const fileNames = Object.keys(this.swiftCodeFiles);
                    if (fileNames.length > 0) {
                        this.activeCodeTab = fileNames[0];
                    }
                    
                    this.$message.success('代码预览生成成功');
                } else {
                    throw new Error(result.error || '生成代码预览失败');
                }
                
            } catch (error) {
                console.error('生成Swift代码预览错误:', error);
                this.$message.error('生成代码预览失败: ' + error.message);
                
                // 显示示例代码
                this.swiftCodeFiles = {
                    'ViewController.swift': `//
//  ViewController.swift
//  Generated from Figma Design
//  Created on ${new Date().toISOString().split('T')[0]}
//

import UIKit

class ViewController: UIViewController {
    
    // MARK: - UI Components
    private let containerView = UIView()
    private let titleLabel = UILabel()
    private let iconImageView = UIImageView()
    
    // MARK: - Lifecycle
    override func viewDidLoad() {
        super.viewDidLoad()
        setupUI()
        setupConstraints()
    }
    
    // MARK: - Setup Methods
    private func setupUI() {
        view.backgroundColor = UIColor.systemBackground
        
        // Container View
        containerView.backgroundColor = UIColor(red: 0.247, green: 0.318, blue: 0.710, alpha: 1.000)
        containerView.layer.cornerRadius = 12
        view.addSubview(containerView)
        
        // Title Label
        titleLabel.text = "Welcome"
        titleLabel.font = UIFont.systemFont(ofSize: 24, weight: .bold)
        titleLabel.textColor = UIColor.white
        titleLabel.textAlignment = .center
        containerView.addSubview(titleLabel)
    }
    
    private func setupConstraints() {
        containerView.translatesAutoresizingMaskIntoConstraints = false
        titleLabel.translatesAutoresizingMaskIntoConstraints = false
        
        NSLayoutConstraint.activate([
            // Container View
            containerView.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            containerView.centerYAnchor.constraint(equalTo: view.centerYAnchor),
            containerView.widthAnchor.constraint(equalToConstant: 300),
            containerView.heightAnchor.constraint(equalToConstant: 200),
            
            // Title Label
            titleLabel.topAnchor.constraint(equalTo: containerView.topAnchor, constant: 20),
            titleLabel.leadingAnchor.constraint(equalTo: containerView.leadingAnchor, constant: 20),
            titleLabel.trailingAnchor.constraint(equalTo: containerView.trailingAnchor, constant: -20)
        ])
    }
}`,
                    'README.md': `# Swift UIKit Code

This Swift code was automatically generated from Figma design.

## Error Information
${error.message}

## Files

- ViewController.swift: Main view controller
- *View.swift: Individual UI components  
- UIView+Extensions.swift: Useful UIView extensions (optional)
- ResourceManager.swift: Resource management utilities (optional)

## Usage

1. Add these files to your Xcode project
2. Import the image assets to your project's asset catalog
3. Update the view controller class name if needed
4. Customize the code as needed for your app

Generated on: ${new Date().toLocaleString()}
`
                };
                this.activeCodeTab = 'ViewController.swift';
            } finally {
                this.generatingPreview = false;
            }
        },

        // 导出Swift代码
        async exportSwiftCode() {
            if (!this.project || !this.project.id) {
                this.$message.error('请先选择一个项目');
                return;
            }

            try {
                this.swiftExporting = true;
                
                const response = await fetch(`/figma/project/${this.project.id}/export/swift`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': window.csrfToken
                    },
                    body: JSON.stringify(this.swiftExportConfig)
                });

                if (!response.ok) {
                    throw new Error('导出请求失败');
                }

                const result = await response.json();
                
                if (result.success) {
                    this.$message.success('Swift代码导出任务已创建，请稍候...');
                    this.swiftExportDialogVisible = false;
                    this.checkSwiftExportStatus(result.job_id);
                } else {
                    throw new Error(result.error || '导出失败');
                }
                
            } catch (error) {
                console.error('Swift导出错误:', error);
                this.$message.error('Swift导出失败: ' + error.message);
            } finally {
                this.swiftExporting = false;
            }
        },

        // 检查Swift导出状态
        async checkSwiftExportStatus(jobId) {
            const checkStatus = async () => {
                try {
                    const response = await fetch(`/figma/export/status/${jobId}`);
                    const result = await response.json();
                    
                    if (result.status === 'completed') {
                        this.$message.success('Swift代码导出完成！');
                        // 自动下载
                        window.open(result.download_url, '_blank');
                    } else if (result.status === 'failed') {
                        this.$message.error('Swift代码导出失败: ' + result.error);
                    } else if (result.status === 'processing') {
                        // 显示进度
                        this.$message.info(`Swift代码导出中... ${result.progress}%`);
                        // 继续检查
                        setTimeout(checkStatus, 2000);
                    } else {
                        // 继续检查
                        setTimeout(checkStatus, 2000);
                    }
                } catch (error) {
                    console.error('检查导出状态失败:', error);
                    this.$message.error('检查导出状态失败');
                }
            };
            
            checkStatus();
        },

        // 其他模板导出方法（占位）
        showUnityExportDialog() {
            this.$message.info('Unity-AppUI 导出功能即将推出');
        },

        showAndroidExportDialog() {
            // 重置配置为默认值
            this.androidExportConfig = {
                imageFormat: 'png',
                imageScale: 2.0,
                codeStyle: 'kotlin-constraintlayout',
                options: ['generateExtensions', 'generateResourceManager', 'generateStyles', 'generateColors']
            };
            this.androidExportDialogVisible = true;
        },

        showVueExportDialog() {
            this.$message.info('Web-Vue 导出功能即将推出');
        },

        // 从预览弹窗导出Swift代码
        async exportSwiftCodeFromPreview() {
            await this.exportSwiftCode();
        },

        // 复制代码到剪贴板
        async copyCodeToClipboard(content, filename) {
            try {
                await navigator.clipboard.writeText(content);
                this.$message.success(`${filename} 代码已复制到剪贴板`);
            } catch (error) {
                console.error('复制失败:', error);
                // 降级方案：创建临时文本区域
                const textArea = document.createElement('textarea');
                textArea.value = content;
                document.body.appendChild(textArea);
                textArea.select();
                try {
                    document.execCommand('copy');
                    this.$message.success(`${filename} 代码已复制到剪贴板`);
                } catch (fallbackError) {
                    this.$message.error('复制失败，请手动选择代码');
                }
                document.body.removeChild(textArea);
            }
        },

        // 获取代码文件大小（格式化显示）
        getCodeFileSize(content) {
            const bytes = new Blob([content]).size;
            if (bytes < 1024) {
                return bytes + ' B';
            } else if (bytes < 1024 * 1024) {
                return Math.round(bytes / 1024) + ' KB';
            } else {
                return Math.round(bytes / (1024 * 1024)) + ' MB';
            }
        },

        // ==================== Android代码导出功能 ====================

        // 显示Android代码预览（在新窗口中打开）
        showAndroidCodePreview() {
            if (!this.project || !this.project.id) {
                this.$message.error('请先选择一个项目');
                return;
            }

            // 构建预览页面URL，包含导出配置参数
            const params = new URLSearchParams({
                imageFormat: this.androidExportConfig.imageFormat,
                imageScale: this.androidExportConfig.imageScale.toString(),
                codeStyle: this.androidExportConfig.codeStyle,
                options: this.androidExportConfig.options.join(',')
            });

            const previewUrl = `/figma/project/${this.project.id}/android/code-preview?${params.toString()}`;
            
            // 在新窗口中打开代码预览页面
            const previewWindow = window.open(previewUrl, '_blank', 'width=1400,height=900,scrollbars=yes,resizable=yes');

            if (!previewWindow) {
                this.$message.error('无法打开预览窗口，请检查浏览器弹窗拦截设置');
                return;
            }

            // 关闭Android导出弹窗
            this.androidExportDialogVisible = false;
            this.$message.success('代码预览页面已在新窗口中打开');
        },

        // 导出Android代码
        async exportAndroidCode() {
            if (!this.project || !this.project.id) {
                this.$message.error('请先选择一个项目');
                return;
            }

            this.androidExporting = true;
            
            try {
                const response = await fetch(`/figma/project/${this.project.id}/export/android`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': window.csrfToken
                    },
                    body: JSON.stringify(this.androidExportConfig)
                });

                const result = await response.json();
                
                if (result.success) {
                    this.$message.success('Android代码导出任务已创建，请稍候...');
                    this.androidExportDialogVisible = false;
                    this.checkAndroidExportStatus(result.job_id);
                } else {
                    throw new Error(result.error || '导出失败');
                }
            } catch (error) {
                console.error('导出Android代码失败:', error);
                this.$message.error('导出失败: ' + error.message);
            } finally {
                this.androidExporting = false;
            }
        },

        // 检查Android导出状态
        async checkAndroidExportStatus(jobId) {
            const maxAttempts = 60; // 最多检查60次（5分钟）
            let attempts = 0;

            const checkStatus = async () => {
                try {
                    const response = await fetch(`/figma/export/status/${jobId}`);
                    const result = await response.json();
                    
                    if (result.success) {
                        const job = result.job;
                        
                        if (job.status === 'completed') {
                            this.$message.success('Android代码导出完成！');
                            // 自动下载
                            if (job.file_path) {
                                const downloadUrl = `/figma/export/${jobId}/download`;
                                const link = document.createElement('a');
                                link.href = downloadUrl;
                                link.download = '';
                                document.body.appendChild(link);
                                link.click();
                                document.body.removeChild(link);
                            }
                            return;
                        } else if (job.status === 'failed') {
                            this.$message.error('Android代码导出失败: ' + (job.error_message || '未知错误'));
                            return;
                        } else if (job.status === 'processing') {
                            // 继续检查
                            attempts++;
                            if (attempts < maxAttempts) {
                                setTimeout(checkStatus, 5000); // 5秒后再次检查
                            } else {
                                this.$message.warning('导出任务超时，请稍后手动检查');
                            }
                        }
                    } else {
                        throw new Error(result.error || '检查状态失败');
                    }
                } catch (error) {
                    console.error('检查Android导出状态失败:', error);
                    attempts++;
                    if (attempts < maxAttempts) {
                        setTimeout(checkStatus, 5000); // 5秒后重试
                    } else {
                        this.$message.error('无法检查导出状态，请稍后手动查看');
                    }
                }
            };

            // 开始检查
            setTimeout(checkStatus, 2000); // 2秒后开始检查
        }
    }
};

// 导出Vue应用配置
window.ProjectEditorApp = ProjectEditorApp;

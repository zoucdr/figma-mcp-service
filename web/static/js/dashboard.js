/**
 * Figma Bridge 项目编辑器脚本
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
            selectionOverlayStyle: null, // 选中节点的高层级预览样式
            layerMenuVisible: false, // 层级菜单是否可见（默认隐藏，不是一加载就显示）
            layerMenuStyle: { // 层级菜单样式
                top: '0px',
                left: '0px'
            },
            contextMenuNodeId: null, // 右键点击的节点ID
            contextMenuSource: null, // 右键菜单来源：'preview'（预览图）、'tree'（节点树）或'minimap'（缩略图）
            contextMenuMinimapType: null, // 缩略图类型：'current'（当前预览）或'filtered'（过滤预览）
            filteredPreviewImages: [], // 过滤后的预览图片列表（用于层级菜单）
            exportStatus: '', // pending, processing, completed, failed
            exportJobId: null,
            defaultImageFormat: 'png', // 默认图片格式
            jsonPreviewDialogVisible: false, // JSON预览弹窗是否可见
            renameDialogVisible: false, // 重命名对话框是否可见
            renameForm: {
                nodeId: null,
                newName: ''
            },
            propertyForm: {
                imageDownloadType: 'png',
                components: [],
                rename: '',
                ignore: false,
                res_mode: 'attach',
                img_name: '',
                anchor_pos: 'middle_center',
                anchor_self: false
            },
            loading: false,
            showCustomSettings: false,
            showLayoutSettings: false,
            showExportSettings: false,
            // 各部分展开状态
            sectionExpanded: {
                basicInfo: true,
                controlSettings: false,
                layoutSettings: false,
                exportSettings: false
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
            }
        };
    },
    computed: {
        // 获取当前节点的子节点
        childNodes() {
            if (!this.currentNode) return [];
            return this.nodes.filter(node => node.parentId === this.currentNode.id);
        },
        
        
        // 判断当前节点是否有修改信息
        hasNodeModifications() {
            return this.currentNode && this.currentNode.modifys;
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
        }
    },
    created() {
        this.loading = true;
        this.loadProjectData();
        
        // 添加全局点击事件监听器，用于关闭层级菜单
        document.addEventListener('click', this.handleGlobalClick);
    },
    
    beforeDestroy() {
        // 移除全局点击事件监听器
        document.removeEventListener('click', this.handleGlobalClick);
        
        // 移除鼠标移动和松开事件监听
        document.removeEventListener('mousemove', this.handleMouseMove);
        document.removeEventListener('mouseup', this.handleMouseUp);
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
        
        // 返回项目列表页面
        backToProjectList() {
            window.location.href = '/projects';
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
        
    // 选择根节点
    selectRootNode() {
        // 找到根节点（最顶层的节点）
        const rootNodes = this.nodes.filter(node => !node.parentId);
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
            .then(response => {
                this.nodes = response.data.nodes;
                
                // 2. 获取项目的节点修改信息
                return axios.get(`/figma/project/${this.project.id}/node-modifys`);
            })
            .then(response => {
                this.nodeModifys = response.data.modifys;
                
                // 3. 将修改信息叠加到节点树
                this.applyModifysToNodes();
                
                // 4. 初始化树形数据
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
                    const rootNodes = this.nodes.filter(node => !node.parentId);
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
            })
            .catch(error => {
                console.error('加载项目数据失败:', error);
                this.$message.error(error.response?.data?.error || '加载项目数据失败');
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
                if (this.nodeModifys[nodeId]) {
                    // 将修改信息合并到节点对象
                    Object.assign(node, { modifys: this.nodeModifys[nodeId] });
                }
            }
        },
        
    // 初始化树形数据
    initTreeData() {
        if (!this.nodes.length) return;
        
        // 找到根节点
        const rootNodes = this.nodes.filter(node => !node.parentId);
        this.treeData = this.buildTree(rootNodes);
        
        // 确保根节点始终在展开的节点列表中
        if (rootNodes.length > 0) {
            const rootNodeId = rootNodes[0].id;
            if (!this.expandedKeys.includes(rootNodeId)) {
                this.expandedKeys.push(rootNodeId);
            }
        }
    },
        
        // 构建树形结构
        buildTree(nodes) {
        return nodes
            .filter(node => {
                // 过滤掉visible为false的节点
                return node.visible !== false;
            })
            .map(node => {
                const children = this.nodes.filter(n => n.parentId === node.id);
                
                // 使用自定义名称（如果存在）
                const nodeName = node.modifys?.rename || node.modifys?.customName || node.name || node.id;
                
                // 如果节点没有absoluteBoundingBox，添加一个默认的
                if (!node.absoluteRenderBounds) {
                    console.log(`节点 ${node.id} 没有absoluteBoundingBox，添加默认值`);
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
        
        // 查找新选择的节点
        this.currentNode = this.nodes.find(node => node.id === nodeId);
        console.log('找到节点:', this.currentNode);
        
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
    handleTreeNodeClick(nodeId) {
        console.log('树节点被点击:', nodeId);
        
        // 检查是否是重复点击相同节点（仅对主动点击生效）
        if (this.currentNode && this.currentNode.id === nodeId) {
            console.log('忽略重复点击相同节点');
            return;
        }
        
        // 调用节点选择处理逻辑
        this.handleNodeSelect(nodeId);
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
        
        // 查找当前选中节点
        const node = this.nodes.find(n => n.id === this.currentNode.id);
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
        this.renameForm = {
            nodeId: null,
            newName: ''
        };
    },
    
    // 确认重命名
    confirmRename() {
        const { nodeId, newName } = this.renameForm;
        if (!nodeId || !newName.trim()) {
            this.$message.warning('节点名称不能为空');
            return;
        }
        
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
        
        // 刷新树形结构
        this.buildTreeData();
    },
    
    // 设置节点锚点位置
    setNodeAnchor(nodeId, anchorPos) {
        // 确保nodeModifys中有该节点的记录
        if (!this.nodeModifys[nodeId]) {
            this.nodeModifys[nodeId] = {};
        }
        
        // 设置锚点位置
        this.nodeModifys[nodeId].anchor_pos = anchorPos;
        
        // 保存修改
        this.saveNodeModifys();
        
        // 提示用户
        this.$message({
            message: `已设置锚点位置为: ${anchorPos}`,
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
    
    // 保存节点修改
    saveNodeModifys() {
        // 发送请求保存nodeModifys到服务器
        axios.post(`/api/projects/${this.project.id}/node-modifys`, {
            nodeModifys: this.nodeModifys
        }).catch(error => {
            console.error('保存节点修改失败:', error);
            this.$message.error('保存节点修改失败');
        });
    },
    
    // 另存为节点图片
    saveNodeAsImage() {
        // 获取当前选中的节点，如果没有则使用第一个匹配的节点
        let targetNodeId = this.currentNode ? this.currentNode.id : null;
        if (!targetNodeId && this.filteredPreviewImages.length > 0) {
            targetNodeId = this.filteredPreviewImages[0].nodeId;
        }
        
        if (!targetNodeId) {
            this.$message.warning('请先选择一个节点');
            return;
        }
        
        // 找到对应的预览图片
        const previewImage = this.previewImages.find(img => img.nodeId === targetNodeId);
        if (!previewImage || !previewImage.src) {
            this.$message.error('无法获取节点图片');
            return;
        }
        
        // 获取节点信息
        const node = this.nodes.find(n => n.id === targetNodeId);
        const nodeName = (node && node.name) ? node.name : targetNodeId;
        
        // 获取图片格式（从节点修改中获取，默认为png）
        let imageFormat = 'png';
        if (this.nodeModifys[targetNodeId] && this.nodeModifys[targetNodeId].imageDownloadType) {
            imageFormat = this.nodeModifys[targetNodeId].imageDownloadType;
        }
        
        // 创建下载链接
        const link = document.createElement('a');
        link.href = previewImage.src;
        link.download = `${nodeName}.${imageFormat}`;
        link.style.display = 'none';
        document.body.appendChild(link);
        
        // 触发下载
        link.click();
        
        // 清理
        document.body.removeChild(link);
        
        // 提示用户
        this.$message({
            message: '图片已开始下载',
            type: 'success',
            duration: 1500
        });
        
        // 关闭菜单
        this.layerMenuVisible = false;
        this.contextMenuSource = null;
    },
    
    // 保存缩略图图片
    saveMinimapImage() {
        let imageSrc = null;
        let nodeName = '';
        let imageFormat = 'png';
        
        if (this.contextMenuMinimapType === 'current') {
            // 当前预览
            if (!this.currentNode) {
                this.$message.warning('当前没有选中的节点');
                return;
            }
            
            imageSrc = this.getCurrentNodeImage(1.0);
            nodeName = this.currentNode.name || this.currentNode.id;
            
            // 获取图片格式
            if (this.nodeModifys[this.currentNode.id] && this.nodeModifys[this.currentNode.id].imageDownloadType) {
                imageFormat = this.nodeModifys[this.currentNode.id].imageDownloadType;
            }
        } else if (this.contextMenuMinimapType === 'filtered') {
            // 过滤预览
            if (this.filteredImagesList.length === 0) {
                this.$message.warning('没有可用的过滤预览图片');
                return;
            }
            
            const currentImage = this.filteredImagesList[this.currentFilteredImageIndex];
            imageSrc = currentImage.imagePath;
            nodeName = currentImage.nodeName || currentImage.nodeId;
            
            // 获取图片格式
            if (this.nodeModifys[currentImage.nodeId] && this.nodeModifys[currentImage.nodeId].imageDownloadType) {
                imageFormat = this.nodeModifys[currentImage.nodeId].imageDownloadType;
            }
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
        
        // 设置当前右键的节点ID
        this.contextMenuNodeId = nodeId;
        this.contextMenuSource = 'tree'; // 标记菜单来源为节点树
        
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
            // 找出所有可见的FRAME和COMPONENT节点
            const visibleNodes = this.nodes.filter(node => 
                node.visible !== false &&
                node.absoluteRenderBounds // 必须有边界框信息
            );
            
            console.log('预加载节点数量:', visibleNodes.length);
            
            // 找出根节点（最顶层的节点）
            const rootNodes = visibleNodes.filter(node => !node.parentId);
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
        
        // 添加预览图片（实际加载图片）
        addPreviewImage(node, forceRefresh = false, scale = 1.0) { // 默认不强制刷新，使用1.0标准清晰度
            if (!node) return;
            
            // 构建图片URL（不添加时间戳），增加scale参数提高清晰度
            const imageUrl = `/figma/image/${this.project.id}/${node.id}?scale=${scale}`;
            console.log('图片URL:', imageUrl);
            
            // 检查节点是否有absoluteBoundingBox
            console.log('节点数据:', node);
            console.log('节点是否有absoluteBoundingBox:', !!node.absoluteRenderBounds);
            
            // 如果节点没有absoluteBoundingBox，但是是根节点，则创建一个默认的边界框
            if (!node.absoluteRenderBounds && this.previewImages.length === 0) {
                console.log('根节点没有absoluteBoundingBox，创建默认边界框');
                node.absoluteRenderBounds = {
                    x: 0,
                    y: 0,
                    width: 800,  // 默认宽度
                    height: 600  // 默认高度
                };
            }
            
            // 如果有绝对位置信息，则使用它
            if (node.absoluteRenderBounds) {
                // 如果是第一个节点（根节点）或者还没有设置rootNodeBounds，记录其位置作为参考
                if (this.previewImages.length === 0 || !this.rootNodeBounds) {
                    this.rootNodeBounds = JSON.parse(JSON.stringify(node.absoluteRenderBounds)); // 深拷贝
                    console.log('设置根节点边界:', this.rootNodeBounds);
                    
                    // 适应窗口显示
                    this.$nextTick(() => {
                        this.fitToScreen();
                    });
                }
                
                // 检查是否已存在该节点的预览图，如果存在则更新，否则添加
                const existingIndex = this.previewImages.findIndex(img => img.nodeId === node.id);
                if (existingIndex !== -1) {
                    this.$set(this.previewImages, existingIndex, {
                        nodeId: node.id,
                        src: imageUrl,
                        bounds: node.absoluteRenderBounds, // 保存原始边界框信息，用于点击判断
                        nodeName: node.name || node.id, // 保存节点名称
                        loading: true // 标记为加载中
                    });
                } else {
                    // 添加到预览图片列表
                    this.previewImages.push({
                        nodeId: node.id,
                        src: imageUrl,
                        bounds: node.absoluteRenderBounds, // 保存原始边界框信息，用于点击判断
                        nodeName: node.name || node.id, // 保存节点名称
                        loading: true // 标记为加载中
                    });
                    
                    // 根据节点在树中的顺序对预览图片进行排序
                    this.sortPreviewImagesByTreeOrder();
                }
                
                // 更新对应预加载节点的loaded状态
                const preloadedNodeIndex = this.preloadedNodes.findIndex(pNode => pNode.nodeId === node.id);
                if (preloadedNodeIndex !== -1) {
                    this.$set(this.preloadedNodes[preloadedNodeIndex], 'loaded', true);
                }
                
                // 在下一个渲染周期更新所有图片的样式和缩略图显示
                this.$nextTick(() => {
                    this.updatePreviewImagesScale();
                    
                    // 创建一个新的Image对象来预加载图片，确保图片加载完成后再显示
                    const preloadImg = new Image();
                    preloadImg.onload = () => {
                        console.log('预览图片加载完成:', imageUrl);
                        
                        // 更新预览图的loading状态
                        const imgIndex = this.previewImages.findIndex(img => img.nodeId === node.id);
                        if (imgIndex !== -1) {
                            this.$set(this.previewImages[imgIndex], 'loading', false);
                        }
                        
                        // 更新缩略图显示状态
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
                    };
                    
                    // 开始加载图片
                    preloadImg.src = imageUrl;
                });
                
                return;
            }
            // 如果没有绝对位置信息，使用默认居中
            // 检查是否已存在该节点的预览图，如果存在则更新，否则添加
            const existingIndex = this.previewImages.findIndex(img => img.nodeId === node.id);
            if (existingIndex !== -1) {
                this.$set(this.previewImages, existingIndex, {
                    nodeId: node.id,
                    src: imageUrl,
                    nodeName: node.name || node.id, // 保存节点名称
                    loading: true // 标记为加载中
                });
            } else {
                // 添加到预览图片列表
                this.previewImages.push({
                    nodeId: node.id,
                    src: imageUrl,
                    nodeName: node.name || node.id, // 保存节点名称
                    loading: true // 标记为加载中
                });
                
                // 根据节点在树中的顺序对预览图片进行排序
                this.sortPreviewImagesByTreeOrder();
            }
            
            // 在下一个渲染周期更新所有图片的样式
            this.$nextTick(() => {
                this.updatePreviewImagesScale();
                
                // 创建一个新的Image对象来预加载图片，确保图片加载完成后再显示
                const preloadImg = new Image();
                preloadImg.onload = () => {
                    console.log('预览图片加载完成:', imageUrl);
                    
                    // 更新预览图的loading状态
                    const imgIndex = this.previewImages.findIndex(img => img.nodeId === node.id);
                    if (imgIndex !== -1) {
                        this.$set(this.previewImages[imgIndex], 'loading', false);
                    }
                };
                
                // 开始加载图片
                preloadImg.src = imageUrl;
            });
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
        // 更新预加载节点的样式
        this.updatePreloadedNodesStyle();
        
        if (!this.previewImages.length || !this.rootNodeBounds) {
            console.log('没有预览图或根节点边界，无法更新缩放');
            return;
        }
            
            console.log('更新预览图缩放，当前缩放比例:', this.zoomLevel);
            
            // 根节点的宽高
            const rootWidth = this.rootNodeBounds.width;
            const rootHeight = this.rootNodeBounds.height;
            
            console.log('根节点尺寸:', rootWidth, rootHeight);
            
            // 更新所有预览图片的样式
            this.previewImages.forEach((image, index) => {
                const node = this.nodes.find(n => n.id === image.nodeId);
                if (!node || !node.absoluteRenderBounds) {
                    console.log(`节点 ${image.nodeId} 没有边界框，跳过`);
                    return;
                }
                
                // 计算相对于根节点的位置
                const relX = node.absoluteRenderBounds.x - this.rootNodeBounds.x;
                const relY = node.absoluteRenderBounds.y - this.rootNodeBounds.y;
                
                // 计算相对位置的百分比（相对于根节点尺寸）
                const percentX = relX / rootWidth * 100;
                const percentY = relY / rootHeight * 100;
                
                console.log(`节点 ${node.id} 相对位置:`, relX, relY, '百分比:', percentX.toFixed(2) + '%', percentY.toFixed(2) + '%');
                
                // 使用绝对定位，确保只使用绝对位置，只减去根节点的位置
                // 计算节点在容器中的位置：容器中心 + 相对于根节点中心的偏移量 + 拖拽偏移量
                const style = {
                    position: 'absolute',
                    left: `calc(50% + ${(relX - rootWidth/2) * this.zoomLevel + this.previewOffset.x}px)`,
                    top: `calc(50% + ${(relY - rootHeight/2) * this.zoomLevel + this.previewOffset.y}px)`,
                    width: `${node.absoluteRenderBounds.width}px`,
                    height: `${node.absoluteRenderBounds.height}px`,
                    transform: `scale(${this.zoomLevel})`,
                    transformOrigin: '0 0', // 从左上角开始变换
                    zIndex: index + 1 // 使用索引作为z-index基础值，确保按树节点顺序叠放
                };
                
                // 不修改选中节点的层级，始终按节点树正序遍历的顺序排列
                
                console.log(`节点 ${node.id} 计算样式:`, 
                    `left: calc(50% + ${(relX - rootWidth/2) * this.zoomLevel + this.previewOffset.x}px)`, 
                    `top: calc(50% + ${(relY - rootHeight/2) * this.zoomLevel + this.previewOffset.y}px)`);
                
                // 更新样式
                this.$set(image, 'style', style);
            });
            
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
        findNodePath(nodeId, currentNodes = this.nodes, path = []) {
            // 找到当前节点
            const node = currentNodes.find(n => n.id === nodeId);
            if (!node) return [];
            
            // 将当前节点添加到路径
            path.unshift(nodeId);
            
            // 如果没有父节点，返回路径
            if (!node.parentId) return path;
            
            // 递归查找父节点
            return this.findNodePath(node.parentId, currentNodes, path);
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
                                // 更新节点标签为原始名称
                                treeNode.data.label = this.currentNode.name || this.currentNode.id;
                                // 移除修改标记
                                treeNode.data.modifys = null;
                                // 强制更新节点
                                treeNode.data = {...treeNode.data};
                                
                                // 强制刷新树视图
                                this.$nextTick(() => {
                                    // 触发树的更新
                                    this.treeData = [...this.treeData];
                                });
                            }
                        }
                    }
                    
                    // 重置表单为默认值
                    this.propertyForm = {
                        imageDownloadType: 'png',
                        components: [],
                        rename: '',
                        ignore: false,
                        res_mode: 'attach',
                        img_name: '',
                        anchor_pos: 'middle_center',
                        anchor_self: false
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
        
        // 显示所有设置
        showAllSettings() {
            this.showCustomSettings = true;
            this.showLayoutSettings = true;
            this.showExportSettings = true;
            
            // 同时展开所有部分
            this.sectionExpanded.controlSettings = true;
            this.sectionExpanded.layoutSettings = true;
            this.sectionExpanded.exportSettings = true;
            
            // 如果节点类型为TEXT，则默认选择res_mode为"text"
            if (this.currentNode && this.currentNode.type === 'TEXT') {
                this.propertyForm.res_mode = 'text';
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
                imageDownloadType: 'png',
                components: [],
                rename: '',
                ignore: false,
                res_mode: 'attach',
                img_name: '',
                anchor_pos: 'middle_center',
                anchor_self: false
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
                    imageDownloadType: node.modifys.imageDownloadType || 'png',
                    components: node.modifys.components || node.modifys.controlList || [],
                    rename: node.modifys.rename || node.modifys.customName || '',
                    ignore: node.modifys.ignore || false,
                    res_mode: node.modifys.res_mode || 'attach',
                    img_name: node.modifys.img_name || '',
                    anchor_pos: node.modifys.anchor_pos || node.modifys.anchorPoint || node.modifys.stretchType === 'stretch_horizontal' ? 'stretch_horizontal' : 
                              (node.modifys.stretchType === 'stretch_vertical' ? 'stretch_vertical' : 
                              (node.modifys.stretchType === 'stretch_all' ? 'stretch' : 'middle_center')),
                    anchor_self: node.modifys.anchor_self || false
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
                            imageDownloadType: settings.imageDownloadType || 'png',
                            components: settings.components || settings.controlList || [],
                            rename: settings.rename || settings.customName || '',
                            ignore: settings.ignore || false,
                            res_mode: settings.res_mode || 'attach',
                            img_name: settings.img_name || '',
                            anchor_pos: settings.anchor_pos || settings.anchorPoint || settings.stretchType === 'stretch_horizontal' ? 'stretch_horizontal' : 
                                      (settings.stretchType === 'stretch_vertical' ? 'stretch_vertical' : 
                                      (settings.stretchType === 'stretch_all' ? 'stretch' : 'middle_center')),
                            anchor_self: settings.anchor_self || false
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
                    console.error('获取节点设置失败:', error);
                    this.$message.error('获取节点设置失败');
                    
                    // 即使出错也要尝试恢复展开状态
                    if (previousSectionExpanded) {
                        this.sectionExpanded = previousSectionExpanded;
                    }
                });
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
                    
                    // 强制Vue更新视图
                    this.$set(parentNodes, i, parentNodes[i]);
                    
                    // 如果有树引用，刷新节点
                    if (this.$refs.nodeTree) {
                        // 获取节点实例
                        const treeNode = this.$refs.nodeTree.getNode(nodeId);
                        if (treeNode) {
                            // 强制更新节点数据
                            treeNode.data = {...treeNode.data};
                        }
                    }
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
            
            // 发送导出请求，传递默认图片格式
            axios.post(`/figma/project/${this.project.id}/export`, {
                format: this.defaultImageFormat
            })
                .then(response => {
                    this.exportJobId = response.data.job_id;
                    this.checkExportStatus();
                    this.$message.success('导出任务已开始处理');
                })
                .catch(error => {
                    this.exportStatus = 'failed';
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
                        this.exportStatus = status;
                        this.exportProgress = progress;
                        
                        if (status === 'processing') {
                            setTimeout(checkStatus, 1000);
                        } else if (status === 'completed') {
                            this.$message.success('导出完成，正在下载文件...');
                            // 自动下载文件
                            this.downloadExport();
                        } else if (status === 'failed') {
                            this.$message.error('导出失败');
                        }
                    })
                    .catch(() => {
                        this.exportStatus = 'failed';
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
        
    // 获取当前节点的图片URL
    getCurrentNodeImage(scale = 1.0) { // 缩放比例参数，默认为1.0（标准清晰度）
        // 如果没有当前节点或者没有预览图，确保缩略图隐藏并返回空
        if (!this.currentNode || this.previewImages.length === 0) {
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
        
        try {
            // 调用新的批量获取API，使用选择的图片格式
            const timestamp = new Date().getTime();
            const response = await fetch(`/figma/images/${this.project.id}/${this.currentNode.id}?scale=${scale}&format=${this.defaultImageFormat}&t=${timestamp}`);
            
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            
            const data = await response.json();
            
            if (data.success && data.images) {
                console.log(`成功获取 ${data.count} 个过滤预览图片`);
                return data.images;
            } else {
                console.error('获取批量过滤预览图片失败:', data.error || '未知错误');
                return {};
            }
        } catch (error) {
            console.error('获取批量过滤预览图片时发生错误:', error);
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
        }
    }
};

// 导出Vue应用配置
window.ProjectEditorApp = ProjectEditorApp;

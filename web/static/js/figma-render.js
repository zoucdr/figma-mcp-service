/**
 * Figma 渲染功能模块
 * 提供图片渲染、进度跟踪、队列管理等功能
 */

// 渲染管理器
const FigmaRenderManager = {
    // 轮询间隔（毫秒）
    pollInterval: 2000,
    
    // 当前轮询的定时器
    pollTimer: null,
    
    // 正在轮询的队列ID列表
    pollingQueues: new Set(),
    
    // 项目冷却倒计时信息 { projectId: { cooldownRemaining, lastUpdateTime, countdownTimer } }
    projectCooldowns: {},
    
    /**
     * 创建渲染队列
     * @param {Object} params 渲染参数
     * @param {string} params.fileKey - 文件Key
     * @param {Array<string>} params.nodeIds - 节点ID列表
     * @param {string} params.format - 格式 (png/jpg/svg)
     * @param {number} params.scale - 缩放比例
     * @param {Function} onProgress - 进度回调
     * @param {Function} onComplete - 完成回调
     * @param {Function} onError - 错误回调
     */
    async createRenderQueue(params, onProgress, onComplete, onError) {
        try {
            const requestData = {
                file_key: params.fileKey,
                node_ids: params.nodeIds,
                format: params.format || 'png',
                scale: params.scale || 2.0
            };
            
            // 如果有 projectIds，添加到请求中
            if (params.projectIds && params.projectIds.length > 0) {
                requestData.project_ids = params.projectIds;
                console.log('🔍 [FigmaRenderManager] 添加 project_ids 到请求:', params.projectIds);
            } else {
                console.warn('⚠️ [FigmaRenderManager] params.projectIds 为空或不存在:', params.projectIds);
            }
            
            console.log('📤 [FigmaRenderManager] 发送渲染请求:', requestData);
            
            const response = await axios.post('/figma/manual/render', requestData);
            
            if (response.data.code === 0) {
                const queueId = response.data.data.queue_id;
                
                // 开始轮询队列状态
                this.startPolling(queueId, onProgress, onComplete, onError);
                
                return {
                    success: true,
                    queueId: queueId,
                    message: response.data.message
                };
            } else if (response.data.code === 1001) {
                // Token 冷却中
                if (onError) {
                    onError({
                        type: 'cooldown',
                        message: response.data.message,
                        waitSeconds: response.data.data?.wait_seconds || 30,
                        nextTime: response.data.data?.next_available_time
                    });
                }
                return { success: false, error: 'cooldown' };
            } else {
                throw new Error(response.data.message || '创建渲染队列失败');
            }
        } catch (error) {
            console.error('创建渲染队列失败:', error);
            if (onError) {
                onError({
                    type: 'api_error',
                    message: error.response?.data?.message || error.message
                });
            }
            return { success: false, error: error.message };
        }
    },
    
    /**
     * 开始轮询队列状态
     */
    startPolling(queueId, onProgress, onComplete, onError) {
        // 添加到轮询列表
        this.pollingQueues.add(queueId);
        
        // 如果已经有定时器在运行，不需要创建新的
        if (this.pollTimer) {
            return;
        }
        
        // 创建轮询定时器
        this.pollTimer = setInterval(async () => {
            if (this.pollingQueues.size === 0) {
                // 没有需要轮询的队列，停止定时器
                clearInterval(this.pollTimer);
                this.pollTimer = null;
                return;
            }
            
            // 轮询所有队列
            for (const qId of this.pollingQueues) {
                await this.checkQueueStatus(qId, onProgress, onComplete, onError);
            }
        }, this.pollInterval);
        
        // 立即执行一次
        this.checkQueueStatus(queueId, onProgress, onComplete, onError);
    },
    
    /**
     * 检查队列状态
     */
    async checkQueueStatus(queueId, onProgress, onComplete, onError) {
        try {
            const response = await axios.get('/figma/render/queue');
            
            if (response.data.code === 0) {
                const queues = response.data.data.queues || [];
                const queue = queues.find(q => q.id === queueId);
                
                if (!queue) {
                    // 队列不存在，可能已被删除
                    this.pollingQueues.delete(queueId);
                    return;
                }
                
                // 更新进度
                if (onProgress) {
                    onProgress({
                        queueId: queue.id,
                        status: queue.status,
                        progress: queue.progress || 0,
                        processedNodes: queue.processed_nodes || 0,
                        failedNodes: queue.failed_nodes || 0,
                        totalNodes: queue.total_nodes || 0,
                        cooldownRemaining: queue.cooldown_remaining || 0,
                        errorMessage: queue.error_message || ''
                    });
                }
                
                // 检查是否完成
                if (queue.status === 'completed') {
                    this.pollingQueues.delete(queueId);
                    if (onComplete) {
                        onComplete({
                            queueId: queue.id,
                            totalNodes: queue.total_nodes,
                            processedNodes: queue.processed_nodes,
                            failedNodes: queue.failed_nodes
                        });
                    }
                } else if (queue.status === 'failed' || queue.status === 'error') {
                    this.pollingQueues.delete(queueId);
                    if (onError) {
                        onError({
                            type: 'queue_failed',
                            message: queue.error_message || '渲染失败',
                            queueId: queue.id
                        });
                    }
                }
            }
        } catch (error) {
            console.error('检查队列状态失败:', error);
        }
    },
    
    /**
     * 停止轮询指定队列
     */
    stopPolling(queueId) {
        this.pollingQueues.delete(queueId);
    },
    
    /**
     * 获取队列统计信息
     */
    async getQueueStats() {
        try {
            const response = await axios.get('/figma/render/queue/stats');
            if (response.data.code === 0) {
                return response.data.data;
            }
        } catch (error) {
            console.error('获取队列统计失败:', error);
        }
        return null;
    },
    
    /**
     * 获取项目渲染进度
     * @param {number} projectId - 项目ID
     */
    async getProjectRenderProgress(projectId) {
        try {
            console.log(`🔍 [getProjectRenderProgress] 查询项目 ${projectId} 渲染进度...`);
            const response = await axios.get(`/figma/project/${projectId}/render/progress`);
            console.log(`📊 [getProjectRenderProgress] 项目 ${projectId} HTTP响应:`, response.data);
            if (response.data.code === 0) {
                console.log(`✅ [getProjectRenderProgress] 项目 ${projectId} 进度数据:`, response.data.data);
                return response.data.data;
            } else {
                console.warn(`⚠️ [getProjectRenderProgress] 项目 ${projectId} 返回错误码: ${response.data.code}`, response.data);
                return null;
            }
        } catch (error) {
            console.error(`❌ [getProjectRenderProgress] 获取项目 ${projectId} 渲染进度失败:`, error);
        }
        return null;
    },
    
    /**
     * 开始轮询项目渲染进度
     * @param {number} projectId - 项目ID
     * @param {Function} onProgress - 进度回调
     * @param {Function} onComplete - 完成回调
     * @param {Function} onError - 错误回调
     */
    startProjectProgressPolling(projectId, onProgress, onComplete, onError) {
        const pollKey = `project_${projectId}`;
        
        console.log(`🎬 [startProjectProgressPolling] 准备启动项目 ${projectId} 轮询`);
        
        // 如果已经在轮询，先停止
        if (this.pollingQueues.has(pollKey)) {
            console.log(`⏭️ [startProjectProgressPolling] 项目 ${projectId} 已在轮询中，跳过重复启动`);
            return;
        }
        
        // 添加到轮询列表
        this.pollingQueues.add(pollKey);
        console.log(`✅ [startProjectProgressPolling] 项目 ${projectId} 已加入轮询列表，当前轮询数: ${this.pollingQueues.size}`);
        
        // 保存回调函数，以便在轮询中使用
        if (!this.projectCallbacks) {
            this.projectCallbacks = {};
        }
        this.projectCallbacks[projectId] = { onProgress, onComplete, onError };
        
        // 创建轮询函数
        const poll = async () => {
            try {
                const progress = await this.getProjectRenderProgress(projectId);
                
                if (!progress || !progress.has_render) {
                    // 没有渲染任务，停止轮询
                    console.log(`📭 [poll] 项目 ${projectId} 无渲染任务 (has_render=${progress?.has_render})，停止轮询`);
                    this.stopProjectProgressPolling(projectId);
                    return;
                }
                
                // ============ 关键：只有 processing 状态才轮询 ============
                // waiting: 冷却中，使用本地倒计时
                // completed/failed/error: 已结束，停止轮询
                
                if (progress.status === 'waiting') {
                    console.log(`⏸️ [poll] 项目 ${projectId} 处于 waiting 状态，停止轮询`);
                    if (progress.cooldown_remaining > 0) {
                        console.log(`⏱️ [poll] 启动本地倒计时: ${progress.cooldown_remaining} 秒`);
                        this.startLocalCooldownCountdown(projectId, progress, onProgress);
                    }
                    this.stopProjectProgressPolling(projectId);
                    return;
                }
                
                if (progress.status === 'completed' || progress.status === 'partial') {
                    console.log(`✅ [poll] 项目 ${projectId} 已完成 (status=${progress.status})，停止轮询`);
                    this.stopProjectProgressPolling(projectId);
                    if (onComplete) {
                        onComplete(progress);
                    }
                    return;
                }
                
                if (progress.status === 'error' || progress.status === 'cancelled' || progress.status === 'failed') {
                    console.log(`❌ [poll] 项目 ${projectId} 失败/取消 (status=${progress.status})，停止轮询`);
                    this.stopProjectProgressPolling(projectId);
                    if (onError) {
                        onError({
                            type: 'render_failed',
                            message: progress.error_message || '渲染失败'
                        });
                    }
                    return;
                }
                
                // 只有 processing 状态继续轮询
                if (progress.status === 'processing') {
                    console.log(`🔄 [poll] 项目 ${projectId} 正在处理中 (${progress.processed_nodes}/${progress.total_nodes})，继续轮询`);
                } else {
                    // 未知状态，也停止轮询
                    console.warn(`⚠️ [poll] 项目 ${projectId} 未知状态 (${progress.status})，停止轮询`);
                    this.stopProjectProgressPolling(projectId);
                    return;
                }
                
                // 清除本地倒计时
                this.stopLocalCooldownCountdown(projectId);
                
                // 更新进度
                if (onProgress) {
                    onProgress({
                        has_render: progress.has_render,
                        render_id: progress.render_id,
                        status: progress.status,
                        progress: progress.progress || 0,
                        processed_nodes: progress.processed_nodes || 0,
                        failed_nodes: progress.failed_nodes || 0,
                        total_nodes: progress.total_nodes || 0,
                        cooldown_remaining: 0,
                        error_message: progress.error_message || '',
                        started_at: progress.started_at,
                        completed_at: progress.completed_at,
                        created_at: progress.created_at,
                        updated_at: progress.updated_at
                    });
                }
            } catch (error) {
                console.error('轮询项目渲染进度失败:', error);
            }
        };
        
        // 立即执行一次
        poll();
        
        // 如果没有定时器，创建一个
        if (!this.pollTimer) {
            console.log(`⏰ [startProjectProgressPolling] 创建定时器，间隔: ${this.pollInterval}ms`);
            this.pollTimer = setInterval(async () => {
                console.log(`⏰ [定时器触发] 当前轮询队列数: ${this.pollingQueues.size}`);
                
                if (this.pollingQueues.size === 0) {
                    console.log(`🛑 [定时器] 无轮询任务，清除定时器`);
                    clearInterval(this.pollTimer);
                    this.pollTimer = null;
                    return;
                }
                
                // 轮询所有任务（包括项目和队列）
                for (const key of this.pollingQueues) {
                    console.log(`🔄 [定时器] 处理轮询任务: ${key}`);
                    if (key.startsWith('project_')) {
                        const projId = parseInt(key.replace('project_', ''));
                        
                        // 检查是否正在本地倒计时中
                        if (this.projectCooldowns[projId]) {
                            //console.log(`⏭️ [轮询跳过] 项目 ${projId} 正在本地倒计时，跳过后端查询`);
                            continue; // 跳过，使用本地倒计时
                        }
                        
                        // 从保存的回调中获取
                        const callbacks = this.projectCallbacks[projId];
                        if (callbacks) {
                            // 重新创建poll函数并执行
                            const pollFunc = async () => {
                                try {
                                    const progress = await this.getProjectRenderProgress(projId);
                                    console.log(`📋 [定时器轮询] 项目 ${projId} 查询结果:`, progress);
                                    
                                    if (!progress || !progress.has_render) {
                                        console.log(`📭 [定时器轮询] 项目 ${projId} 无渲染任务 (has_render=${progress?.has_render})，停止轮询`);
                                        this.stopProjectProgressPolling(projId);
                                        return;
                                    }
                                    
                                    // ============ 只有 processing 状态才继续轮询 ============
                                    if (progress.status === 'waiting') {
                                        console.log(`⏸️ [定时器轮询] 项目 ${projId} 处于 waiting 状态，停止轮询`);
                                        if (progress.cooldown_remaining > 0) {
                                            this.startLocalCooldownCountdown(projId, progress, callbacks.onProgress);
                                        }
                                        this.stopProjectProgressPolling(projId);
                                        return;
                                    }
                                    
                                    if (progress.status === 'completed' || progress.status === 'partial') {
                                        console.log(`✅ [定时器轮询] 项目 ${projId} 已完成 (status=${progress.status})，停止轮询`);
                                        this.stopProjectProgressPolling(projId);
                                        if (callbacks.onComplete) {
                                            callbacks.onComplete(progress);
                                        }
                                        return;
                                    }
                                    
                                    if (progress.status === 'error' || progress.status === 'cancelled' || progress.status === 'failed') {
                                        console.log(`❌ [定时器轮询] 项目 ${projId} 失败/取消 (status=${progress.status})，停止轮询`);
                                        this.stopProjectProgressPolling(projId);
                                        if (callbacks.onError) {
                                            callbacks.onError({
                                                type: 'render_failed',
                                                message: progress.error_message || '渲染失败'
                                            });
                                        }
                                        return;
                                    }
                                    
                                    // 只有 processing 状态继续轮询
                                    if (progress.status === 'processing') {
                                        console.log(`🔄 [定时器轮询] 项目 ${projId} 正在处理中，继续轮询`);
                                    } else {
                                        // 未知状态，也停止轮询
                                        console.warn(`⚠️ [定时器轮询] 项目 ${projId} 未知状态 (${progress.status})，停止轮询`);
                                        this.stopProjectProgressPolling(projId);
                                        return;
                                    }
                                    
                                    if (callbacks.onProgress) {
                                        callbacks.onProgress({
                                            has_render: progress.has_render,
                                            render_id: progress.render_id,
                                            status: progress.status,
                                            progress: progress.progress || 0,
                                            processed_nodes: progress.processed_nodes || 0,
                                            failed_nodes: progress.failed_nodes || 0,
                                            total_nodes: progress.total_nodes || 0,
                                            cooldown_remaining: 0,
                                            error_message: progress.error_message || ''
                                        });
                                    }
                                } catch (error) {
                                    console.error('轮询项目渲染进度失败:', error);
                                }
                            };
                            await pollFunc();
                        }
                    }
                }
            }, this.pollInterval);
        }
    },
    
    /**
     * 停止轮询项目渲染进度
     * @param {number} projectId - 项目ID
     */
    stopProjectProgressPolling(projectId) {
        const pollKey = `project_${projectId}`;
        console.log(`🛑 [stopProjectProgressPolling] 停止项目 ${projectId} 轮询`);
        this.pollingQueues.delete(pollKey);
        console.log(`📊 [stopProjectProgressPolling] 当前轮询数: ${this.pollingQueues.size}`);
        
        // 清除本地倒计时
        this.stopLocalCooldownCountdown(projectId);
        
        // 清理回调函数
        if (this.projectCallbacks && this.projectCallbacks[projectId]) {
            delete this.projectCallbacks[projectId];
        }
    },
    
    /**
     * 启动本地冷却倒计时（避免频繁请求后端）
     */
    startLocalCooldownCountdown(projectId, initialProgress, onProgress) {
        // 清除已存在的倒计时
        this.stopLocalCooldownCountdown(projectId);
        
        // 记录初始冷却时间和当前时间
        this.projectCooldowns[projectId] = {
            initialCooldown: initialProgress.cooldown_remaining,
            startTime: Date.now(),
            progress: initialProgress
        };
        
        //console.log(`⏱️ [本地倒计时] 项目 ${projectId} 开始倒计时: ${initialProgress.cooldown_remaining} 秒`);
        
        // 立即更新一次
        if (onProgress) {
            onProgress({
                ...initialProgress,
                cooldown_remaining: initialProgress.cooldown_remaining
            });
        }
        
        // 每秒更新一次倒计时
        const countdownTimer = setInterval(() => {
            const cooldownInfo = this.projectCooldowns[projectId];
            if (!cooldownInfo) {
                clearInterval(countdownTimer);
                return;
            }
            
            // 计算剩余时间
            const elapsed = Math.floor((Date.now() - cooldownInfo.startTime) / 1000);
            const remaining = Math.max(0, cooldownInfo.initialCooldown - elapsed);
            
            //console.log(`⏱️ [本地倒计时] 项目 ${projectId} 剩余: ${remaining} 秒`);
            
            // 更新前端显示
            if (onProgress) {
                onProgress({
                    ...cooldownInfo.progress,
                    cooldown_remaining: remaining
                });
            }
            
            // 倒计时结束，重新查询后端
            if (remaining <= 0) {
                console.log(`✅ [本地倒计时] 项目 ${projectId} 倒计时结束，重新查询后端`);
                clearInterval(countdownTimer);
                delete this.projectCooldowns[projectId];
                
                // 重新查询后端状态
                this.getProjectRenderProgress(projectId).then(progress => {
                    if (progress && onProgress) {
                        onProgress({
                            ...progress,
                            cooldown_remaining: progress.cooldown_remaining || 0
                        });
                        
                        // 如果还是 waiting 状态且有冷却时间，继续倒计时
                        if (progress.status === 'waiting' && progress.cooldown_remaining > 0) {
                            this.startLocalCooldownCountdown(projectId, progress, onProgress);
                        }
                    }
                }).catch(error => {
                    console.error('重新查询进度失败:', error);
                });
            }
        }, 1000); // 每秒更新
        
        // 保存定时器引用
        this.projectCooldowns[projectId].countdownTimer = countdownTimer;
    },
    
    /**
     * 停止本地冷却倒计时
     */
    stopLocalCooldownCountdown(projectId) {
        const cooldownInfo = this.projectCooldowns[projectId];
        if (cooldownInfo && cooldownInfo.countdownTimer) {
            clearInterval(cooldownInfo.countdownTimer);
            delete this.projectCooldowns[projectId];
            console.log(`🛑 [本地倒计时] 项目 ${projectId} 倒计时已停止`);
        }
    }
};

// Vue Mixin - 可以在任何组件中混入使用
const FigmaRenderMixin = {
    data() {
        return {
            // 渲染对话框可见性
            renderDialogVisible: false,
            
            // 渲染表单数据
            renderForm: {
                fileKey: '',
                nodeIds: [],
                projectIds: [], // 关联的项目ID列表
                format: 'png',
                scale: 2.0
            },
            
            // 渲染进度
            renderProgress: {
                queueId: null,
                status: 'waiting',
                progress: 0,
                processedNodes: 0,
                failedNodes: 0,
                totalNodes: 0,
                message: ''
            },
            
            // 项目渲染进度
            projectRenderProgress: {
                hasRender: false,
                renderId: null,
                status: 'idle',
                progress: 0,
                processedNodes: 0,
                failedNodes: 0,
                totalNodes: 0,
                message: ''
            },
            
            // 是否正在渲染
            rendering: false
        };
    },
    
    mounted() {
        // 如果组件有 project.id，自动开始轮询渲染进度
        if (this.project && this.project.id) {
            this.startWatchingProjectRender();
        }
    },
    
    beforeDestroy() {
        // 组件销毁时停止轮询
        if (this.project && this.project.id) {
            FigmaRenderManager.stopProjectProgressPolling(this.project.id);
        }
    },
    
    methods: {
        /**
         * 显示渲染对话框
         */
        showRenderDialog(fileKey, nodeIds, projectIds) {
            console.log('🎯 [showRenderDialog] 接收参数:', { fileKey, nodeIds: nodeIds?.length, projectIds });
            this.renderForm.fileKey = fileKey;
            this.renderForm.nodeIds = nodeIds || [];
            this.renderForm.projectIds = projectIds || []; // 设置项目ID列表
            console.log('📝 [showRenderDialog] renderForm 已更新:', this.renderForm);
            this.renderDialogVisible = true;
        },
        
        /**
         * 开始渲染
         */
        async startRender() {
            if (!this.renderForm.fileKey || this.renderForm.nodeIds.length === 0) {
                this.$message.warning('请选择要渲染的节点');
                return;
            }
            
            this.rendering = true;
            this.renderProgress.status = 'preparing';
            this.renderProgress.message = '正在创建渲染队列...';
            
            const result = await FigmaRenderManager.createRenderQueue(
                this.renderForm,
                // 进度回调
                (progress) => {
                    this.renderProgress = {
                        ...this.renderProgress,
                        ...progress,
                        nextAvailableTime: progress.nextAvailableTime || progress.next_available_time || 0
                    };
                    
                    // 更新消息
                    if (progress.status === 'waiting') {
                        if (progress.nextAvailableTime || progress.next_available_time) {
                            const wait = this.calculateWaitSeconds(progress.nextAvailableTime || progress.next_available_time);
                            this.renderProgress.message = wait > 0 ? `等待中... 预计等待 ${this.formatWaitTime(wait)}` : '等待处理...';
                        } else {
                            this.renderProgress.message = '等待处理...';
                        }
                    } else if (progress.status === 'processing') {
                        this.renderProgress.message = `处理中... ${progress.processedNodes}/${progress.totalNodes}`;
                    }
                },
                // 完成回调
                (result) => {
                    this.rendering = false;
                    this.renderProgress.status = 'completed';
                    this.renderProgress.message = '渲染完成！';
                    
                    this.$message.success({
                        message: `渲染完成！成功: ${result.processedNodes}, 失败: ${result.failedNodes}`,
                        duration: 3000
                    });
                    
                    // 3秒后关闭对话框
                    setTimeout(() => {
                        this.renderDialogVisible = false;
                        this.resetRenderForm();
                    }, 3000);
                },
                // 错误回调
                (error) => {
                    this.rendering = false;
                    
                    if (error.type === 'cooldown') {
                        this.$alert(
                            `Token 冷却中，请等待 ${error.waitSeconds} 秒后重试`,
                            'Token 冷却',
                            {
                                confirmButtonText: '确定',
                                type: 'warning'
                            }
                        );
                    } else {
                        this.$alert(
                            error.message || '渲染失败，请稍后重试',
                            '渲染错误',
                            {
                                confirmButtonText: '确定',
                                type: 'error'
                            }
                        );
                    }
                }
            );
            
            if (!result.success) {
                return;
            }
            
            this.renderProgress.queueId = result.queueId;
            this.renderProgress.status = 'waiting';
            this.renderProgress.message = '队列已创建，等待处理...';
        },
        
        /**
         * 取消渲染
         */
        cancelRender() {
            if (this.renderProgress.queueId) {
                FigmaRenderManager.stopPolling(this.renderProgress.queueId);
            }
            this.renderDialogVisible = false;
            this.rendering = false;
            this.resetRenderForm();
        },
        
        /**
         * 重置渲染表单
         */
        resetRenderForm() {
            this.renderForm = {
                fileKey: '',
                nodeIds: [],
                projectIds: [], // 重置项目ID列表
                format: 'png',
                scale: 2.0
            };
            this.renderProgress = {
                queueId: null,
                status: 'waiting',
                progress: 0,
                processedNodes: 0,
                failedNodes: 0,
                totalNodes: 0,
                nextAvailableTime: 0,
                message: ''
            };
        },
        
        /**
         * 格式化状态文本
         */
        formatStatus(status) {
            const statusMap = {
                'waiting': '等待中',
                'preparing': '准备中',
                'processing': '处理中',
                'completed': '已完成',
                'failed': '失败',
                'error': '错误'
            };
            return statusMap[status] || status;
        },
        
        /**
         * 获取状态颜色
         */
        getStatusColor(status) {
            const colorMap = {
                'waiting': '#909399',
                'preparing': '#409EFF',
                'processing': '#409EFF',
                'completed': '#67C23A',
                'failed': '#F56C6C',
                'error': '#F56C6C'
            };
            return colorMap[status] || '#909399';
        },
        
        /**
         * 开始监控项目渲染进度
         */
        startWatchingProjectRender() {
            if (!this.project || !this.project.id) {
                return;
            }
            
            FigmaRenderManager.startProjectProgressPolling(
                this.project.id,
                // 进度回调
                (progress) => {
                    this.projectRenderProgress = {
                        hasRender: progress.has_render,
                        renderId: progress.render_id,
                        status: progress.status,
                        progress: progress.progress || 0,
                        processedNodes: progress.processed_nodes || 0,
                        failedNodes: progress.failed_nodes || 0,
                        totalNodes: progress.total_nodes || 0,
                        message: this.formatRenderMessage(progress)
                    };
                },
                // 完成回调
                (progress) => {
                    this.$message.success('渲染完成！');
                    this.projectRenderProgress.status = 'completed';
                    this.projectRenderProgress.message = '渲染完成！';
                    
                    // 刷新页面数据（如果有的话）
                    if (typeof this.loadProjectData === 'function') {
                        this.loadProjectData(true);
                    }
                },
                // 错误回调
                (error) => {
                    this.$message.error(error.message || '渲染失败');
                    this.projectRenderProgress.status = 'error';
                    this.projectRenderProgress.message = error.message || '渲染失败';
                }
            );
        },
        
        /**
         * 停止监控项目渲染进度
         */
        stopWatchingProjectRender() {
            if (!this.project || !this.project.id) {
                return;
            }
            FigmaRenderManager.stopProjectProgressPolling(this.project.id);
        },
        
        /**
         * 格式化渲染消息
         */
        formatRenderMessage(progress) {
            if (!progress.has_render) {
                return '';
            }
            
            switch (progress.status) {
                case 'waiting':
                    return '等待处理...';
                case 'processing':
                    return `渲染中... ${progress.processed_nodes}/${progress.total_nodes}`;
                case 'completed':
                    return '渲染完成！';
                case 'error':
                case 'failed':
                    return progress.error_message || '渲染失败';
                default:
                    return '';
            }
        },
        
        /**
         * 计算等待时间（秒）
         * @param {number} nextAvailableTime - Unix时间戳（秒）
         */
        calculateWaitSeconds(nextAvailableTime) {
            if (!nextAvailableTime) return 0;
            const now = Math.floor(Date.now() / 1000);
            const wait = nextAvailableTime - now;
            return Math.max(0, wait);
        },
        
        /**
         * 格式化等待时间显示
         * @param {number} seconds - 等待秒数
         */
        formatWaitTime(seconds) {
            if (seconds < 60) {
                return `${seconds}秒`;
            } else if (seconds < 3600) {
                const minutes = Math.floor(seconds / 60);
                const secs = seconds % 60;
                return secs > 0 ? `${minutes}分${secs}秒` : `${minutes}分钟`;
            } else {
                const hours = Math.floor(seconds / 3600);
                const minutes = Math.floor((seconds % 3600) / 60);
                return minutes > 0 ? `${hours}小时${minutes}分钟` : `${hours}小时`;
            }
        },
        
        /**
         * 获取状态对应的Tag类型
         */
        getStatusType(status) {
            const typeMap = {
                'waiting': 'info',
                'processing': 'primary',
                'completed': 'success',
                'failed': 'danger',
                'error': 'danger',
                'cancelled': 'warning'
            };
            return typeMap[status] || 'info';
        },
        
        /**
         * 获取状态对应的进度条状态
         */
        getProgressStatus(status) {
            if (status === 'completed') return 'success';
            if (status === 'failed' || status === 'error') return 'exception';
            return null;
        },
        
        /**
         * 获取状态文本
         */
        getStatusText(status) {
            const textMap = {
                'waiting': '等待中',
                'processing': '渲染中',
                'completed': '已完成',
                'failed': '失败',
                'error': '错误',
                'cancelled': '已取消'
            };
            return textMap[status] || status;
        }
    }
};

// 导出
window.FigmaRenderManager = FigmaRenderManager;
window.FigmaRenderMixin = FigmaRenderMixin;


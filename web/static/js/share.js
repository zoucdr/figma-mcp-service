/**
 * Figma Deliver 提示词分享页面脚本
 */

const ShareApp = {
    data() {
        return {
            userId: window.initialData?.userId || 0,
            username: window.initialData?.username || '',
            csrfToken: window.initialData?.csrfToken || '',
            shares: [],
            myShares: [],
            total: 0,
            limit: 20,
            currentPage: 1,
            loading: false,
            loadingMyShares: false,
            createDialogVisible: false,
            mySharesDialogVisible: false,
            submitting: false,
            createForm: {
                title: '',
                description: '',
                prompt: ''
            },
            createRules: {
                title: [
                    { required: true, message: '请输入标题', trigger: 'blur' },
                    { min: 2, max: 100, message: '长度在 2 到 100 个字符', trigger: 'blur' }
                ],
                prompt: [
                    { required: true, message: '请输入提示词内容', trigger: 'blur' },
                    { min: 10, message: '提示词内容至少 10 个字符', trigger: 'blur' }
                ]
            }
        };
    },
    
    mounted() {
        this.loadShares();
    },
    
    methods: {
        // 加载分享列表
        async loadShares() {
            this.loading = true;
            try {
                const offset = (this.currentPage - 1) * this.limit;
                const response = await axios.get(`/share/api/shares?limit=${this.limit}&offset=${offset}`);
                
                if (response.data.success) {
                    this.shares = response.data.data.shares.map(share => ({
                        ...share,
                        liked: false,
                        liking: false,
                        deleting: false,
                        expanded: false
                    }));
                    this.total = response.data.data.total;
                    
                    // 如果用户已登录，检查点赞状态
                    if (this.userId) {
                        this.checkLikeStatuses();
                    }
                }
            } catch (error) {
                console.error('加载分享列表失败:', error);
                this.$message.error('加载分享列表失败');
            } finally {
                this.loading = false;
            }
        },
        
        // 检查点赞状态
        async checkLikeStatuses() {
            for (const share of this.shares) {
                if (share.user_id !== this.userId) {
                    try {
                        const response = await axios.get(`/share/api/shares/${share.id}/like-status`);
                        if (response.data.success) {
                            share.liked = response.data.liked;
                        }
                    } catch (error) {
                        console.error('检查点赞状态失败:', error);
                    }
                }
            }
        },
        
        // 切换点赞
        async toggleLike(share) {
            if (!this.userId) {
                this.$message.warning('请先登录');
                return;
            }
            
            if (share.user_id === this.userId) {
                this.$message.warning('不能为自己的分享点赞');
                return;
            }
            
            share.liking = true;
            try {
                const url = share.liked 
                    ? `/share/api/shares/${share.id}/unlike`
                    : `/share/api/shares/${share.id}/like`;
                    
                const response = await axios.post(url, {}, {
                    headers: {
                        'X-CSRF-Token': this.csrfToken
                    }
                });
                
                if (response.data.success) {
                    share.liked = !share.liked;
                    share.like_count = response.data.like_count;
                    this.$message.success(response.data.message);
                }
            } catch (error) {
                console.error('操作失败:', error);
                this.$message.error(error.response?.data?.error || '操作失败');
            } finally {
                share.liking = false;
            }
        },
        
        // 显示创建对话框
        showCreateDialog() {
            if (!this.userId) {
                this.$message.warning('请先登录');
                this.goToLogin();
                return;
            }
            this.createDialogVisible = true;
        },
        
        // 提交分享
        submitShare() {
            this.$refs.createForm.validate(async (valid) => {
                if (!valid) {
                    return false;
                }
                
                this.submitting = true;
                try {
                    const response = await axios.post('/share/api/shares', this.createForm, {
                        headers: {
                            'X-CSRF-Token': this.csrfToken
                        }
                    });
                    
                    if (response.data.success) {
                        this.$message.success('分享成功');
                        this.createDialogVisible = false;
                        this.createForm = {
                            title: '',
                            description: '',
                            prompt: ''
                        };
                        this.$refs.createForm.resetFields();
                        this.currentPage = 1;
                        this.loadShares();
                    }
                } catch (error) {
                    console.error('分享失败:', error);
                    this.$message.error(error.response?.data?.error || '分享失败');
                } finally {
                    this.submitting = false;
                }
            });
        },
        
        // 删除分享
        deleteShare(share, isFromMyShares = false) {
            this.$confirm('确定要删除这个分享吗？', '提示', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                type: 'warning'
            }).then(async () => {
                share.deleting = true;
                try {
                    const response = await axios.delete(`/share/api/shares/${share.id}`, {
                        headers: {
                            'X-CSRF-Token': this.csrfToken
                        }
                    });
                    
                    if (response.data.success) {
                        this.$message.success('删除成功');
                        if (isFromMyShares) {
                            this.loadMyShares();
                        }
                        this.loadShares();
                    }
                } catch (error) {
                    console.error('删除失败:', error);
                    this.$message.error(error.response?.data?.error || '删除失败');
                } finally {
                    share.deleting = false;
                }
            }).catch(() => {
                // 取消删除
            });
        },
        
        // 显示我的分享
        async showMyShares() {
            this.mySharesDialogVisible = true;
            this.loadMyShares();
        },
        
        // 加载我的分享
        async loadMyShares() {
            this.loadingMyShares = true;
            try {
                const response = await axios.get('/share/api/my-shares');
                
                if (response.data.success) {
                    this.myShares = response.data.data;
                }
            } catch (error) {
                console.error('加载我的分享失败:', error);
                this.$message.error('加载我的分享失败');
            } finally {
                this.loadingMyShares = false;
            }
        },
        
        // 切换提示词展开状态
        togglePromptExpand(share) {
            share.expanded = !share.expanded;
        },
        
        // 获取显示的提示词内容
        getDisplayPrompt(share) {
            if (share.expanded || share.prompt.length <= 300) {
                return share.prompt;
            }
            return share.prompt.substring(0, 300) + '...';
        },
        
        // 复制提示词
        copyPrompt(prompt) {
            const textArea = document.createElement('textarea');
            textArea.value = prompt;
            textArea.style.position = 'fixed';
            textArea.style.left = '-999999px';
            document.body.appendChild(textArea);
            textArea.select();
            
            try {
                document.execCommand('copy');
                this.$message.success('复制成功');
            } catch (err) {
                console.error('复制失败:', err);
                this.$message.error('复制失败，请手动复制');
            }
            
            document.body.removeChild(textArea);
        },
        
        // 分页切换
        handlePageChange(page) {
            this.currentPage = page;
            this.loadShares();
            window.scrollTo(0, 0);
        },
        
        // 格式化日期
        formatDate(dateString) {
            const date = new Date(dateString);
            const now = new Date();
            const diff = now - date;
            
            // 小于1分钟
            if (diff < 60000) {
                return '刚刚';
            }
            // 小于1小时
            if (diff < 3600000) {
                return Math.floor(diff / 60000) + '分钟前';
            }
            // 小于1天
            if (diff < 86400000) {
                return Math.floor(diff / 3600000) + '小时前';
            }
            // 小于7天
            if (diff < 604800000) {
                return Math.floor(diff / 86400000) + '天前';
            }
            
            // 超过7天，显示具体日期
            const year = date.getFullYear();
            const month = String(date.getMonth() + 1).padStart(2, '0');
            const day = String(date.getDate()).padStart(2, '0');
            
            if (year === now.getFullYear()) {
                return `${month}-${day}`;
            }
            
            return `${year}-${month}-${day}`;
        },
        
        // 导航方法
        goToHome() {
            window.location.href = '/';
        },
        
        goToLogin() {
            window.location.href = '/login';
        },
        
        goToProfile() {
            window.location.href = '/profile';
        },
        
        // 处理用户菜单命令
        handleUserCommand(command) {
            switch (command) {
                case 'profile':
                    window.location.href = '/profile';
                    break;
                case 'projects':
                    window.location.href = '/projects';
                    break;
                case 'about':
                    window.location.href = '/about';
                    break;
                case 'logout':
                    window.location.href = '/logout';
                    break;
            }
        }
    }
};

// 初始化Vue应用
new Vue(ShareApp).$mount('#share-app');


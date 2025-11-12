// 个人资料页面Vue实例
new Vue({
    el: '#app',
    data() {
        return {
            loading: false,
            mcpTokenLoading: false,
            unityDialogVisible: false,
            androidDialogVisible: false,
            iosDialogVisible: false,
            shareDialogVisible: false,
            shareCodeDialogVisible: false,
            sharing: false,
            sharingCode: false,
            mcpToken: window.profileData?.mcpToken || '',
            profileForm: {
                username: window.profileData?.username || '',
                figmaToken: window.profileData?.figmaToken || '',
                compTypes: window.profileData?.compTypes || '',
                prompts: window.profileData?.prompts || '',
                codePrompts: window.profileData?.codePrompts || ''
            },
            shareForm: {
                title: '',
                description: '',
                prompt: ''
            },
            shareCodeForm: {
                title: '',
                description: '',
                prompt: ''
            },
            shareRules: {
                title: [
                    { required: true, message: '请输入标题', trigger: 'blur' },
                    { min: 2, max: 100, message: '长度在 2 到 100 个字符', trigger: 'blur' }
                ],
                prompt: [
                    { required: true, message: '提示词内容不能为空', trigger: 'blur' }
                ]
            },
            shareCodeRules: {
                title: [
                    { required: true, message: '请输入标题', trigger: 'blur' },
                    { min: 2, max: 100, message: '长度在 2 到 100 个字符', trigger: 'blur' }
                ],
                prompt: [
                    { required: true, message: '提示词内容不能为空', trigger: 'blur' }
                ]
            },
            profileRules: {
                username: [
                    // 用户名可以为空
                ],
                figmaToken: [
                    // 移除必填验证，允许为空
                    // { required: true, message: '请输入Figma Private Token', trigger: 'blur' }
                ]
            },
            unityComponents: "Button, Toggle, Slider, Dropdown, InputField, ScrollView, Scrollbar, ToggleGroup, Mask, GridLayoutGroup, VerticalLayoutGroup, HorizontalLayoutGroup, ContentSizeFitter",
            androidComponents: "Button, CheckBox, RadioButton, Switch, ToggleButton, SeekBar, ProgressBar, Spinner, ListView, GridView, RecyclerView, ScrollView, LinearLayout, RelativeLayout, FrameLayout, ConstraintLayout, ViewPager, Toolbar, CardView, RatingBar, SearchView",
            iosComponents: "UIButton, UISwitch, UISlider, UIProgressView, UISegmentedControl, UITableView, UICollectionView, UIScrollView, UIStackView, UINavigationBar, UITabBar, UIPageControl, UIPickerView, UIDatePicker, UISearchBar, UIStepper",
            // 模块收起/展开状态
            sectionStates: {
                basicInfo: true,
                mcpConnection: true,
                controlConfig: true,
                aiPrompt: true,
                codePrompt: true
            }
        };
    },
    methods: {
        handleCommand(command) {
            if (command === 'dashboard') {
                window.location.href = '/projects';
            } else if (command === 'about') {
                window.location.href = '/about';
            } else if (command === 'logout') {
                window.location.href = '/logout';
            }
        },
        
        // 返回上一个页面
        goBack() {
            // 检查是否有历史记录可以返回
            if (window.history.length > 1) {
                window.history.back();
            } else {
                // 如果没有历史记录，默认返回项目列表
                window.location.href = '/projects';
            }
        },

        // 切换模块收起/展开状态
        toggleSection(sectionName) {
            this.sectionStates[sectionName] = !this.sectionStates[sectionName];
            // 保存状态到localStorage
            localStorage.setItem('profileSectionStates', JSON.stringify(this.sectionStates));
            
            // 在下一个tick中更新DOM类
            this.$nextTick(() => {
                this.updateCardStates();
                this.updateContainerGaps();
            });
        },

        // 更新卡片状态
        updateCardStates() {
            const cards = document.querySelectorAll('.collapsible-card');
            cards.forEach(card => {
                const isExpanded = this.getSectionStateByCard(card);
                if (isExpanded) {
                    card.classList.remove('collapsed');
                } else {
                    card.classList.add('collapsed');
                }
            });
        },

        // 更新容器间距
        updateContainerGaps() {
            const leftContainer = document.querySelector('.profile-left');
            const rightContainer = document.querySelector('.profile-right');
            
            // 检查左侧容器是否有收起的卡片
            const leftHasCollapsed = leftContainer && leftContainer.querySelector('.collapsible-card.collapsed');
            if (leftContainer) {
                if (leftHasCollapsed) {
                    leftContainer.classList.add('has-collapsed');
                } else {
                    leftContainer.classList.remove('has-collapsed');
                }
            }
            
            // 检查右侧容器是否有收起的卡片
            const rightHasCollapsed = rightContainer && rightContainer.querySelector('.collapsible-card.collapsed');
            if (rightContainer) {
                if (rightHasCollapsed) {
                    rightContainer.classList.add('has-collapsed');
                } else {
                    rightContainer.classList.remove('has-collapsed');
                }
            }
        },

        // 根据卡片元素获取对应的状态
        getSectionStateByCard(cardElement) {
            if (cardElement.querySelector('[class*="el-icon-user"]')) {
                return this.sectionStates.basicInfo;
            } else if (cardElement.querySelector('[class*="el-icon-connection"]')) {
                return this.sectionStates.mcpConnection;
            } else if (cardElement.querySelector('[class*="el-icon-setting"]')) {
                return this.sectionStates.controlConfig;
            } else if (cardElement.querySelector('[class*="el-icon-edit-outline"]')) {
                return this.sectionStates.aiPrompt;
            }
            return true;
        },
        // 显示Unity控件示例
        showUnityExamples() {
            this.unityDialogVisible = true;
        },
        // 显示Android控件示例
        showAndroidExamples() {
            this.androidDialogVisible = true;
        },
        // 显示iOS控件示例
        showIOSExamples() {
            this.iosDialogVisible = true;
        },
        // 使用Unity控件示例
        useUnityExamples() {
            this.profileForm.compTypes = this.unityComponents;
            this.unityDialogVisible = false;
            this.$message.success('已应用Unity控件列表');
        },
        // 使用Android控件示例
        useAndroidExamples() {
            this.profileForm.compTypes = this.androidComponents;
            this.androidDialogVisible = false;
            this.$message.success('已应用Android控件列表');
        },
        // 使用iOS控件示例
        useIOSExamples() {
            this.profileForm.compTypes = this.iosComponents;
            this.iosDialogVisible = false;
            this.$message.success('已应用iOS控件列表');
        },
        updateProfile() {
            this.$refs.profileForm.validate(valid => {
                if (valid) {
                    this.loading = true;
                    const formData = new FormData();
                    formData.append('username', this.profileForm.username);
                    formData.append('figma_token', this.profileForm.figmaToken);
                    formData.append('comp_types', this.profileForm.compTypes);
                    formData.append('prompts', this.profileForm.prompts);
                    formData.append('code_prompts', this.profileForm.codePrompts);
                    // CSRF令牌已由main.js中的axios拦截器自动添加
                    // 无需在此手动添加
                    
                    axios.post('/profile/update', formData)
                        .then(response => {
                            this.$message.success('个人资料更新成功');
                            // 保存成功后不跳转页面，保持在当前页面
                        })
                        .catch(error => {
                            this.$message.error(error.response?.data?.error || '更新失败');
                        })
                        .finally(() => {
                            this.loading = false;
                        });
                }
            });
        },
        
        // 生成MCP Token
        async generateMCPToken() {
            try {
                this.mcpTokenLoading = true;
                const response = await fetch('/profile/generate-mcp-token', {
                    method: 'POST',
                    credentials: 'include',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': window.profileData.csrfToken
                    }
                });

                if (response.ok) {
                    const data = await response.json();
                    if (data.success) {
                        this.mcpToken = data.token;
                        this.$message.success(data.message);
                    } else {
                        this.$message.error(data.error || '生成Token失败');
                    }
                } else {
                    const errorData = await response.json();
                    this.$message.error(errorData.error || '生成Token失败');
                }
            } catch (error) {
                console.error('生成MCP Token失败:', error);
                this.$message.error('生成Token失败: ' + error.message);
            } finally {
                this.mcpTokenLoading = false;
            }
        },

        // 撤销MCP Token
        async revokeMCPToken() {
            this.$confirm('确定要撤销MCP Token吗？撤销后Cursor将无法连接到MCP服务。', '确认撤销', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                type: 'warning'
            }).then(async () => {
                try {
                    this.mcpTokenLoading = true;
                    const response = await fetch('/profile/revoke-mcp-token', {
                        method: 'POST',
                        credentials: 'include',
                        headers: {
                            'Content-Type': 'application/json',
                            'X-CSRF-Token': window.profileData.csrfToken
                        }
                    });

                    if (response.ok) {
                        const data = await response.json();
                        if (data.success) {
                            this.mcpToken = '';
                            this.$message.success(data.message);
                        } else {
                            this.$message.error(data.error || '撤销Token失败');
                        }
                    } else {
                        const errorData = await response.json();
                        this.$message.error(errorData.error || '撤销Token失败');
                    }
                } catch (error) {
                    console.error('撤销MCP Token失败:', error);
                    this.$message.error('撤销Token失败: ' + error.message);
                } finally {
                    this.mcpTokenLoading = false;
                }
            }).catch(() => {
                // 用户取消操作
            });
        },

        // 复制Token到剪贴板
        copyToken() {
            if (navigator.clipboard) {
                navigator.clipboard.writeText(this.mcpToken).then(() => {
                    this.$message.success('Token已复制到剪贴板');
                }).catch(() => {
                    this.fallbackCopyToken();
                });
            } else {
                this.fallbackCopyToken();
            }
        },

        // 备用复制方法
        fallbackCopyToken() {
            const textArea = document.createElement('textarea');
            textArea.value = this.mcpToken;
            document.body.appendChild(textArea);
            textArea.select();
            try {
                document.execCommand('copy');
                this.$message.success('Token已复制到剪贴板');
            } catch (err) {
                this.$message.error('复制失败，请手动复制');
            }
            document.body.removeChild(textArea);
        },

        // 获取Cursor配置示例
        getCursorConfig() {
            if (!this.mcpToken) return '';
            return JSON.stringify({
                "mcpServers": {
                    "FigmaDeliver": {
                        "url": `http://127.0.0.1:8080/mcp/${this.mcpToken}`
                    }
                }
            }, null, 2);
        },
        
        // 显示分享提示词对话框
        showSharePromptDialog() {
            if (!this.profileForm.prompts || this.profileForm.prompts.trim() === '') {
                this.$message.warning('请先设置修饰提示词内容');
                return;
            }
            
            this.shareForm = {
                title: '',
                description: '',
                prompt: this.profileForm.prompts
            };
            this.shareDialogVisible = true;
        },
        
        // 提交分享提示词
        async submitSharePrompt() {
            this.$refs.shareForm.validate(async (valid) => {
                if (!valid) {
                    return false;
                }
                
                this.sharing = true;
                try {
                    const response = await axios.post('/share/api/shares', this.shareForm);
                    
                    if (response.data.success) {
                        this.$message.success('分享成功');
                        this.shareDialogVisible = false;
                        this.shareForm = {
                            title: '',
                            description: '',
                            prompt: ''
                        };
                        this.$refs.shareForm.resetFields();
                    }
                } catch (error) {
                    console.error('分享失败:', error);
                    this.$message.error(error.response?.data?.error || '分享失败');
                } finally {
                    this.sharing = false;
                }
            });
        },
        
        // 跳转到分享广场
        goToSharePage() {
            window.location.href = '/share';
        },
        
        // 显示分享代码生成提示词对话框
        showShareCodePromptDialog() {
            if (!this.profileForm.codePrompts || this.profileForm.codePrompts.trim() === '') {
                this.$message.warning('请先设置代码生成提示词内容');
                return;
            }
            
            this.shareCodeForm = {
                title: '',
                description: '',
                prompt: this.profileForm.codePrompts
            };
            this.shareCodeDialogVisible = true;
        },
        
        // 提交分享代码生成提示词
        async submitShareCodePrompt() {
            this.$refs.shareCodeForm.validate(async (valid) => {
                if (!valid) {
                    return false;
                }
                
                this.sharingCode = true;
                try {
                    const response = await axios.post('/share/api/shares', this.shareCodeForm);
                    
                    if (response.data.success) {
                        this.$message.success('分享成功');
                        this.shareCodeDialogVisible = false;
                        this.shareCodeForm = {
                            title: '',
                            description: '',
                            prompt: ''
                        };
                        this.$refs.shareCodeForm.resetFields();
                    }
                } catch (error) {
                    console.error('分享失败:', error);
                    this.$message.error(error.response?.data?.error || '分享失败');
                } finally {
                    this.sharingCode = false;
                }
            });
        }
    },
    mounted() {
        // 恢复保存的模块状态
        const savedStates = localStorage.getItem('profileSectionStates');
        if (savedStates) {
            try {
                const states = JSON.parse(savedStates);
                this.sectionStates = { ...this.sectionStates, ...states };
            } catch (error) {
                console.error('恢复模块状态失败:', error);
            }
        }

        // 初始化卡片状态
        this.$nextTick(() => {
            this.updateCardStates();
            this.updateContainerGaps();
        });
    }
});

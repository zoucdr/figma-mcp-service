// About页面Vue应用
new Vue({
    el: '#about-app',
    data() {
        return {
            // 当前激活的标签页
            activeTab: 'plugin',
            
            // API代码示例标签页
            activeApiTab: 'python',
            
            // 当前展开的FAQ项
            activeFaq: ['faq1'],
            
            // 功能特点数据
            features: [
                {
                    id: 'export',
                    icon: 'el-icon-download',
                    title: '直接导出设计资源',
                    description: '从 Figma 直接导出设计资源，支持多种格式和缩放比例，无需手动操作'
                },
                {
                    id: 'plugin',
                    icon: 'el-icon-connection',
                    title: 'Figma 插件支持',
                    description: '提供原生 Figma 插件，在设计环境中直接操作，提升工作效率'
                },
                {
                    id: 'auto',
                    icon: 'el-icon-magic-stick',
                    title: '自动处理转换',
                    description: '自动处理设计元素的导出和转换，智能优化输出格式和质量'
                },
                {
                    id: 'api',
                    icon: 'el-icon-s-platform',
                    title: 'API 接口集成',
                    description: '提供完整的 API 接口，方便开发团队集成到现有工作流中'
                },
                {
                    id: 'preview',
                    icon: 'el-icon-view',
                    title: '实时预览功能',
                    description: '支持实时预览设计效果，包括过滤预览和批量预览功能'
                },
                {
                    id: 'mcp',
                    icon: 'el-icon-cpu',
                    title: 'MCP 协议支持',
                    description: '集成 Model Context Protocol，支持 AI 辅助的设计处理和优化'
                },
                {
                    id: 'batch',
                    icon: 'el-icon-s-grid',
                    title: '批量操作',
                    description: '支持批量导出、批量预览和批量配置，提高大型项目处理效率'
                },
                {
                    id: 'team',
                    icon: 'el-icon-user-solid',
                    title: '团队协作',
                    description: '支持多用户管理，项目权限控制，便于团队协作和资源共享'
                }
            ],
            
            // API端点数据 - 专注于API服务模块
            apiEndpoints: [
                {
                    method: 'GET',
                    path: '/api/:mcptoken',
                    description: '通过 MCP Token 获取用户信息'
                },
                {
                    method: 'GET',
                    path: '/api/:mcptoken/optimized_nodes',
                    description: '获取优化的 JSON 数据，包含节点修改信息'
                },
                {
                    method: 'GET',
                    path: '/api/:mcptoken/nodes',
                    description: '获取项目节点列表和修改状态'
                },
                {
                    method: 'GET',
                    path: '/api/:mcptoken/download_image',
                    description: '下载指定节点的图片文件'
                },
                {
                    method: 'POST',
                    path: '/api/:mcptoken/clear_cache',
                    description: '清除指定项目的缓存文件'
                },
                {
                    method: 'POST',
                    path: '/api/:mcptoken/clear_temp_files',
                    description: '清除用户的所有临时文件'
                }
            ],
            
            // 常见问题数据
            faqs: [
                {
                    id: 'faq1',
                    question: '如何安装和配置 Figma 插件？',
                    answer: `
                        <p><strong>开发模式安装：</strong></p>
                        <ol>
                            <li>打开 Figma 桌面应用</li>
                            <li>进入 <code>Plugins</code> > <code>Development</code> > <code>Import plugin from manifest...</code></li>
                            <li>选择项目中的 <code>figma-plugin/manifest.json</code> 文件</li>
                            <li>插件将出现在 Plugins 菜单中</li>
                        </ol>
                        <p><strong>配置连接：</strong></p>
                        <ol>
                            <li>在插件设置中配置服务器地址：<code>http://localhost:8080</code></li>
                            <li>输入用户名和密码进行认证</li>
                            <li>认证成功后即可使用</li>
                        </ol>
                    `
                },
                {
                    id: 'faq2',
                    question: '服务无法启动怎么办？',
                    answer: `
                        <p>请检查以下几个方面：</p>
                        <ul>
                            <li><strong>Go 版本：</strong>确保已安装 Go 1.16 或更高版本</li>
                            <li><strong>配置文件：</strong>检查 <code>configs/config.env</code> 文件是否正确配置</li>
                            <li><strong>端口占用：</strong>确认 8080 端口没有被其他程序占用</li>
                            <li><strong>数据库连接：</strong>检查数据库连接配置是否正确</li>
                        </ul>
                        <p>如果问题仍然存在，请查看控制台输出的错误信息。</p>
                    `
                },
                {
                    id: 'faq3',
                    question: '插件认证失败怎么解决？',
                    answer: `
                        <p>认证失败通常由以下原因引起：</p>
                        <ul>
                            <li><strong>服务器地址错误：</strong>确认服务器地址是否正确，默认为 <code>http://localhost:8080</code></li>
                            <li><strong>用户凭据错误：</strong>检查用户名和密码是否正确</li>
                            <li><strong>网络连接问题：</strong>确保插件能够访问服务器</li>
                            <li><strong>CORS 配置：</strong>确认服务器已正确配置 CORS 头</li>
                        </ul>
                        <p>可以先在浏览器中访问服务器地址，确认服务正常运行。</p>
                    `
                },
                {
                    id: 'faq4',
                    question: '导出失败的常见原因？',
                    answer: `
                        <p>导出失败可能的原因：</p>
                        <ul>
                            <li><strong>Figma API 令牌无效：</strong>检查 Figma API 令牌是否有效且有足够权限</li>
                            <li><strong>节点选择问题：</strong>确认在 Figma 中选择了有效的设计元素</li>
                            <li><strong>网络连接：</strong>确保网络连接稳定</li>
                            <li><strong>文件权限：</strong>检查 Figma 文件是否有访问权限</li>
                            <li><strong>服务器资源：</strong>确认服务器有足够的存储空间和处理能力</li>
                        </ul>
                        <p>建议先尝试导出简单的单个元素进行测试。</p>
                    `
                },
                {
                    id: 'faq5',
                    question: '如何使用 MCP 功能？',
                    answer: `
                        <p>MCP (Model Context Protocol) 功能使用方法：</p>
                        <ol>
                            <li><strong>连接 MCP 服务：</strong>在项目页面点击 MCP 连接按钮</li>
                            <li><strong>获取预览：</strong>使用 MCP 工具获取节点预览图</li>
                            <li><strong>配置节点：</strong>通过 MCP 接口批量配置节点属性</li>
                            <li><strong>AI 辅助：</strong>获取 AI 生成的配置提示词</li>
                        </ol>
                        <p>MCP 功能特别适合与支持 MCP 协议的 AI 编辑器（如 Cursor）配合使用。</p>
                    `
                },
                {
                    id: 'faq6',
                    question: '支持哪些导出格式？',
                    answer: `
                        <p>Figma Deliver 支持以下导出格式：</p>
                        <ul>
                            <li><strong>PNG：</strong>适用于大多数图片资源，支持透明背景</li>
                            <li><strong>JPG：</strong>适用于照片类图片，文件体积较小</li>
                            <li><strong>SVG：</strong>矢量格式，适用于图标和简单图形</li>
                            <li><strong>PDF：</strong>适用于文档和打印用途</li>
                        </ul>
                        <p>同时支持多种缩放比例：0.5x、1x、2x、3x、4x，满足不同分辨率需求。</p>
                    `
                }
            ]
        };
    },
    
    methods: {
        // 处理用户下拉菜单命令
        handleCommand(command) {
            switch (command) {
                case 'profile':
                    window.location.href = '/profile';
                    break;
                case 'settings':
                    // 跳转到设置页面或显示设置对话框
                    this.$message.info('设置功能即将推出');
                    break;
                case 'about':
                    // 已经在关于页面，显示提示或滚动到顶部
                    this.$message.info('您已经在关于页面了');
                    window.scrollTo({ top: 0, behavior: 'smooth' });
                    break;
                case 'logout':
                    this.logout();
                    break;
            }
        },
        
        // 退出登录
        logout() {
            this.$confirm('确定要退出登录吗？', '提示', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                type: 'warning'
            }).then(() => {
                // 发送退出登录请求
                axios.post('/logout')
                    .then(() => {
                        this.$message.success('已退出登录');
                        window.location.href = '/login';
                    })
                    .catch(error => {
                        console.error('退出登录失败:', error);
                        // 即使请求失败也跳转到登录页
                        window.location.href = '/login';
                    });
            }).catch(() => {
                // 用户取消
            });
        },
        
        // 平滑滚动到指定区域
        scrollToSection(sectionId) {
            const element = document.getElementById(sectionId);
            if (element) {
                element.scrollIntoView({ 
                    behavior: 'smooth',
                    block: 'start'
                });
            }
        },
        
        // 复制文本到剪贴板
        copyToClipboard(text) {
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(text).then(() => {
                    this.$message.success('已复制到剪贴板');
                }).catch(err => {
                    console.error('复制失败:', err);
                    this.fallbackCopyTextToClipboard(text);
                });
            } else {
                this.fallbackCopyTextToClipboard(text);
            }
        },
        
        // 备用复制方法
        fallbackCopyTextToClipboard(text) {
            const textArea = document.createElement("textarea");
            textArea.value = text;
            textArea.style.top = "0";
            textArea.style.left = "0";
            textArea.style.position = "fixed";
            
            document.body.appendChild(textArea);
            textArea.focus();
            textArea.select();
            
            try {
                const successful = document.execCommand('copy');
                if (successful) {
                    this.$message.success('已复制到剪贴板');
                } else {
                    this.$message.error('复制失败');
                }
            } catch (err) {
                console.error('复制失败:', err);
                this.$message.error('复制失败');
            }
            
            document.body.removeChild(textArea);
        },
        
        // 打开外部链接
        openExternalLink(url) {
            window.open(url, '_blank', 'noopener,noreferrer');
        },
        
        // 格式化代码显示
        formatCode(code) {
            return code.replace(/</g, '&lt;').replace(/>/g, '&gt;');
        },
        
        // 处理功能卡片点击
        onFeatureCardClick(feature) {
            // 可以添加功能卡片点击的交互效果
            console.log('点击功能卡片:', feature.title);
        },
        
        // 处理API项点击
        onApiItemClick(api) {
            // 复制API路径
            this.copyToClipboard(api.path);
        },
        
        // 初始化页面动画
        initAnimations() {
            // 可以添加页面加载动画
            this.$nextTick(() => {
                // 添加渐入动画类
                const elements = document.querySelectorAll('.feature-card, .installation-card, .contribute-item');
                elements.forEach((el, index) => {
                    setTimeout(() => {
                        el.style.opacity = '0';
                        el.style.transform = 'translateY(20px)';
                        el.style.transition = 'all 0.6s ease';
                        
                        setTimeout(() => {
                            el.style.opacity = '1';
                            el.style.transform = 'translateY(0)';
                        }, 100);
                    }, index * 100);
                });
            });
        }
    },
    
    mounted() {
        // 页面加载完成后的初始化
        this.initAnimations();
        
        // 监听滚动事件，添加导航栏背景效果
        window.addEventListener('scroll', () => {
            const header = document.querySelector('.about-header');
            if (window.scrollY > 50) {
                header.style.background = 'rgba(255, 255, 255, 0.95)';
                header.style.backdropFilter = 'blur(20px)';
            } else {
                header.style.background = 'rgba(255, 255, 255, 0.1)';
                header.style.backdropFilter = 'blur(10px)';
            }
        });
        
        // 添加平滑滚动支持
        document.querySelectorAll('a[href^="#"]').forEach(anchor => {
            anchor.addEventListener('click', function (e) {
                e.preventDefault();
                const target = document.querySelector(this.getAttribute('href'));
                if (target) {
                    target.scrollIntoView({
                        behavior: 'smooth',
                        block: 'start'
                    });
                }
            });
        });
    },
    
    beforeDestroy() {
        // 清理事件监听器
        window.removeEventListener('scroll', this.handleScroll);
    }
});

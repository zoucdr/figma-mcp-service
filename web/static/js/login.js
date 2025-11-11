/**
 * Figma Bridge 登录页面脚本
 */

// 创建Vue实例
new Vue({
    el: '#app',
    data() {
        // 确认密码验证规则
        const validateConfirmPassword = (rule, value, callback) => {
            if (value !== this.registerForm.password) {
                callback(new Error('两次输入的密码不一致'));
            } else {
                callback();
            }
        };
        
        return {
            activeTab: 'login',
            loading: false,
            loginForm: {
                username: '',
                password: ''
            },
            registerForm: {
                username: '',
                password: '',
                confirmPassword: '',
                figmaToken: ''
            },
            loginRules: {
                username: [
                    { required: true, message: '请输入用户名', trigger: 'blur' }
                ],
                password: [
                    { required: true, message: '请输入密码', trigger: 'blur' },
                    { min: 6, message: '密码长度至少为6位', trigger: 'blur' }
                ]
            },
            registerRules: {
                username: [
                    { required: true, message: '请输入用户名', trigger: 'blur' }
                ],
                password: [
                    { required: true, message: '请输入密码', trigger: 'blur' },
                    { min: 6, message: '密码长度至少为6位', trigger: 'blur' }
                ],
                confirmPassword: [
                    { required: true, message: '请确认密码', trigger: 'blur' },
                    { validator: validateConfirmPassword, trigger: 'blur' }
                ],
                figmaToken: [
                    { required: true, message: '请输入Figma Private Token', trigger: 'blur' }
                ]
            }
        };
    },
    methods: {
        handleLogin() {
            this.$refs.loginForm.validate(valid => {
                if (valid) {
                    this.loading = true;
                    const formData = new FormData();
                    formData.append('username', this.loginForm.username);
                    formData.append('password', this.loginForm.password);
                    
                    // CSRF令牌已由main.js中的axios拦截器自动添加
                    // 无需在此手动添加
                    
                    axios.post('/login', formData)
                        .then(response => {
                            this.$message.success('登录成功');
                            // 登录成功后直接跳转到仪表盘
                            window.location.href = '/dashboard';
                        })
                        .catch(error => {
                            this.$message.error(error.response?.data?.error || '登录失败');
                        })
                        .finally(() => {
                            this.loading = false;
                        });
                }
            });
        },
        handleRegister() {
            this.$refs.registerForm.validate(valid => {
                if (valid) {
                    this.loading = true;
                    const formData = new FormData();
                    formData.append('username', this.registerForm.username);
                    formData.append('password', this.registerForm.password);
                    formData.append('figma_token', this.registerForm.figmaToken);
                    
                    // CSRF令牌已由main.js中的axios拦截器自动添加
                    // 无需在此手动添加
                    
                    axios.post('/register', formData)
                        .then(response => {
                            this.$message.success('注册成功');
                            window.location.href = '/dashboard';
                        })
                        .catch(error => {
                            this.$message.error(error.response?.data?.error || '注册失败');
                        })
                        .finally(() => {
                            this.loading = false;
                        });
                }
            });
        }
    }
});

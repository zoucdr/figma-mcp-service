// 个人资料页面Vue实例
new Vue({
    el: '#app',
    data() {
        return {
            loading: false,
            unityDialogVisible: false,
            androidDialogVisible: false,
            iosDialogVisible: false,
            profileForm: {
                username: window.profileData?.username || '',
                figmaToken: window.profileData?.figmaToken || '',
                compTypes: window.profileData?.compTypes || ''
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
            unityComponents: "Button, Toggle, Slider, Dropdown, InputField, Text, Image, RawImage, ScrollView, Scrollbar, ToggleGroup, Canvas, Panel, Mask, EventSystem, GridLayoutGroup, VerticalLayoutGroup, HorizontalLayoutGroup, ContentSizeFitter, RectTransform, Animator, AudioSource, Camera, Light",
            androidComponents: "TextView, EditText, Button, ImageView, CheckBox, RadioButton, Switch, ToggleButton, SeekBar, ProgressBar, Spinner, ListView, GridView, RecyclerView, ScrollView, WebView, LinearLayout, RelativeLayout, FrameLayout, ConstraintLayout, ViewPager, Toolbar, CardView, ImageButton, AutoCompleteTextView, RatingBar, SearchView, SurfaceView, VideoView",
            iosComponents: "UILabel, UITextField, UIButton, UIImageView, UISwitch, UISlider, UIProgressView, UIActivityIndicatorView, UISegmentedControl, UITableView, UICollectionView, UITextView, UIScrollView, UIStackView, UINavigationBar, UITabBar, UIPageControl, UIPickerView, UIDatePicker, UIWebView (旧), WKWebView, UIVisualEffectView, UISearchBar, UIAlertController, UIStepper"
        };
    },
    methods: {
        handleCommand(command) {
            if (command === 'dashboard') {
                window.location.href = '/dashboard';
            } else if (command === 'logout') {
                window.location.href = '/logout';
            }
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
                    // CSRF令牌已由main.js中的axios拦截器自动添加
                    // 无需在此手动添加
                    
                    axios.post('/profile/update', formData)
                        .then(response => {
                            this.$message.success('个人资料更新成功');
                            // 更新成功后跳转到仪表盘页面
                            window.location.href = '/dashboard';
                        })
                        .catch(error => {
                            this.$message.error(error.response?.data?.error || '更新失败');
                        })
                        .finally(() => {
                            this.loading = false;
                        });
                }
            });
        }
    }
});

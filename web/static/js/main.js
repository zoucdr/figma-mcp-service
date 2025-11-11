/**
 * Figma Deliver 主脚本文件
 */

// 全局配置
const config = {
    apiBaseUrl: '',
    debug: false
};

// 配置Axios全局CSRF令牌
document.addEventListener('DOMContentLoaded', function() {
    // 从meta标签获取CSRF令牌
    const metaTag = document.querySelector('meta[name="csrf-token"]');
    if (metaTag) {
        const csrfToken = metaTag.getAttribute('content');
        console.log('全局CSRF令牌:', csrfToken);
        
        // 为所有请求添加CSRF令牌和AJAX标识
        axios.defaults.headers.common['X-CSRF-Token'] = csrfToken;
        axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest';
        
        // 为表单提交添加CSRF令牌
        axios.interceptors.request.use(function(config) {
            if (config.data instanceof FormData) {
                config.data.append('_csrf', csrfToken);
            }
            return config;
        });
    } else {
        console.error('找不到CSRF令牌meta标签');
    }
});

// 全局工具函数
const utils = {
    /**
     * 显示加载中
     * @param {string} message - 加载提示消息
     * @returns {object} - 加载实例
     */
    showLoading(message = '加载中...') {
        return Vue.prototype.$loading({
            lock: true,
            text: message,
            spinner: 'el-icon-loading',
            background: 'rgba(0, 0, 0, 0.7)'
        });
    },
    
    /**
     * 格式化日期
     * @param {Date|string} date - 日期对象或日期字符串
     * @param {string} format - 格式化模板
     * @returns {string} - 格式化后的日期字符串
     */
    formatDate(date, format = 'YYYY-MM-DD HH:mm:ss') {
        if (!date) return '';
        
        date = typeof date === 'string' ? new Date(date) : date;
        
        const year = date.getFullYear();
        const month = date.getMonth() + 1;
        const day = date.getDate();
        const hours = date.getHours();
        const minutes = date.getMinutes();
        const seconds = date.getSeconds();
        
        const pad = (num) => (num < 10 ? '0' + num : num);
        
        return format
            .replace('YYYY', year)
            .replace('MM', pad(month))
            .replace('DD', pad(day))
            .replace('HH', pad(hours))
            .replace('mm', pad(minutes))
            .replace('ss', pad(seconds));
    },
    
    /**
     * 获取URL参数
     * @param {string} name - 参数名
     * @returns {string|null} - 参数值
     */
    getUrlParam(name) {
        const urlParams = new URLSearchParams(window.location.search);
        return urlParams.get(name);
    },
    
    /**
     * 深拷贝对象
     * @param {object} obj - 要拷贝的对象
     * @returns {object} - 拷贝后的对象
     */
    deepClone(obj) {
        return JSON.parse(JSON.stringify(obj));
    },
    
    /**
     * 防抖函数
     * @param {Function} func - 要执行的函数
     * @param {number} wait - 等待时间
     * @returns {Function} - 防抖后的函数
     */
    debounce(func, wait = 300) {
        let timeout;
        return function(...args) {
            clearTimeout(timeout);
            timeout = setTimeout(() => {
                func.apply(this, args);
            }, wait);
        };
    }
};

// 全局HTTP请求配置
axios.interceptors.request.use(config => {
    // 添加CSRF令牌
    const csrfToken = document.querySelector('meta[name="csrf-token"]')?.getAttribute('content');
    if (csrfToken) {
        config.headers['X-CSRF-Token'] = csrfToken;
    }
    return config;
}, error => {
    return Promise.reject(error);
});

// 全局HTTP响应拦截器
axios.interceptors.response.use(response => {
    return response;
}, error => {
    if (error.response) {
        // 处理401未授权错误
        if (error.response.status === 401) {
            window.location.href = '/login';
            return;
        }
        
        // 处理500服务器错误
        if (error.response.status === 500) {
            Vue.prototype.$message.error('服务器错误，请稍后重试');
        }
    }
    
    return Promise.reject(error);
});

// 全局Vue配置
Vue.config.productionTip = false;

// 全局Vue过滤器
Vue.filter('formatDate', utils.formatDate);

// 全局Vue指令
Vue.directive('focus', {
    inserted: function(el) {
        el.focus();
    }
});

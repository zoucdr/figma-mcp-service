/**
 * 代码预览页面脚本
 */

const CodePreviewApp = {
    data() {
        return {
            // 基础数据
            projectId: window.codePreviewData?.projectId || null,
            projectName: window.codePreviewData?.projectName || '',
            codeFiles: window.codePreviewData?.codeFiles || {},
            exportConfig: window.codePreviewData?.exportConfig || {
                imageFormat: 'png',
                imageScale: 2.0,
                codeStyle: 'uikit-autolayout',
                options: ['generateExtensions', 'generateResourceManager']
            },
            
            // 界面状态
            activeFile: '',
            settingsDialogVisible: false,
            
            // 加载状态
            generating: false,
            regenerating: false,
            exporting: false,
            applying: false
        };
    },
    
    mounted() {
        // 设置默认激活文件
        const fileNames = Object.keys(this.codeFiles);
        if (fileNames.length > 0) {
            this.activeFile = fileNames[0];
        }
        
        // 如果没有代码文件，自动生成
        if (fileNames.length === 0) {
            this.generateCode();
        }
    },
    
    methods: {
        // 选择文件
        selectFile(filename) {
            this.activeFile = filename;
        },
        
        // 获取文件图标
        getFileIcon(filename) {
            const ext = filename.split('.').pop().toLowerCase();
            switch (ext) {
                case 'swift':
                    return 'el-icon-document file-icon-swift';
                case 'md':
                    return 'el-icon-document file-icon-md';
                case 'txt':
                    return 'el-icon-document file-icon-txt';
                default:
                    return 'el-icon-document file-icon-default';
            }
        },
        
        // 获取语言类名
        getLanguageClass(filename) {
            const ext = filename.split('.').pop().toLowerCase();
            return `lang-${ext}`;
        },
        
        // 获取文件大小
        getFileSize(content) {
            const bytes = new Blob([content]).size;
            if (bytes < 1024) {
                return bytes + ' B';
            } else if (bytes < 1024 * 1024) {
                return Math.round(bytes / 1024) + ' KB';
            } else {
                return Math.round(bytes / (1024 * 1024)) + ' MB';
            }
        },
        
        // 复制文件内容
        async copyFile(filename, content) {
            try {
                await navigator.clipboard.writeText(content);
                this.$message.success(`${filename} 已复制到剪贴板`);
            } catch (error) {
                console.error('复制失败:', error);
                // 降级方案
                const textArea = document.createElement('textarea');
                textArea.value = content;
                document.body.appendChild(textArea);
                textArea.select();
                try {
                    document.execCommand('copy');
                    this.$message.success(`${filename} 已复制到剪贴板`);
                } catch (fallbackError) {
                    this.$message.error('复制失败，请手动选择代码');
                }
                document.body.removeChild(textArea);
            }
        },
        
        // 下载单个文件
        downloadFile(filename, content) {
            const blob = new Blob([content], { type: 'text/plain' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            this.$message.success(`${filename} 下载成功`);
        },
        
        // 生成代码
        async generateCode() {
            if (!this.projectId) {
                this.$message.error('项目ID无效');
                return;
            }
            
            try {
                this.generating = true;
                
                const response = await fetch(`/figma/project/${this.projectId}/swift/preview-full`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': window.codePreviewData.csrfToken
                    },
                    body: JSON.stringify(this.exportConfig)
                });
                
                if (!response.ok) {
                    throw new Error('生成代码失败');
                }
                
                const result = await response.json();
                
                if (result.success) {
                    this.codeFiles = result.files || {};
                    
                    // 设置默认激活文件
                    const fileNames = Object.keys(this.codeFiles);
                    if (fileNames.length > 0) {
                        this.activeFile = fileNames[0];
                    }
                    
                    this.$message.success('代码生成成功');
                } else {
                    throw new Error(result.error || '生成代码失败');
                }
                
            } catch (error) {
                console.error('生成代码错误:', error);
                this.$message.error('生成代码失败: ' + error.message);
                
                // 显示示例代码
                this.showExampleCode(error.message);
                
            } finally {
                this.generating = false;
            }
        },
        
        // 重新生成代码
        async regenerateCode() {
            this.regenerating = true;
            await this.generateCode();
            this.regenerating = false;
        },
        
        // 导出全部代码
        async exportAllCode() {
            if (!this.projectId) {
                this.$message.error('项目ID无效');
                return;
            }
            
            try {
                this.exporting = true;
                
                const response = await fetch(`/figma/project/${this.projectId}/export/swift`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': window.codePreviewData.csrfToken
                    },
                    body: JSON.stringify(this.exportConfig)
                });
                
                if (!response.ok) {
                    throw new Error('导出请求失败');
                }
                
                const result = await response.json();
                
                if (result.success) {
                    this.$message.success('Swift代码导出任务已创建，请稍候...');
                    this.checkExportStatus(result.job_id);
                } else {
                    throw new Error(result.error || '导出失败');
                }
                
            } catch (error) {
                console.error('导出错误:', error);
                this.$message.error('导出失败: ' + error.message);
            } finally {
                this.exporting = false;
            }
        },
        
        // 检查导出状态
        async checkExportStatus(jobId) {
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
        
        // 打开设置
        openSettings() {
            this.settingsDialogVisible = true;
        },
        
        // 应用设置
        async applySettings() {
            this.applying = true;
            this.settingsDialogVisible = false;
            
            // 重新生成代码
            await this.generateCode();
            
            this.applying = false;
            this.$message.success('设置已应用，代码已重新生成');
        },
        
        // 关闭预览
        closePreview() {
            // 如果是在新窗口中打开的，关闭窗口
            if (window.opener) {
                window.close();
            } else {
                // 否则返回上一页
                window.history.back();
            }
        },
        
        // 显示示例代码
        showExampleCode(errorMessage) {
            this.codeFiles = {
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
        
        // Icon Image View
        iconImageView.image = UIImage(named: "icon_welcome")
        iconImageView.contentMode = .scaleAspectFit
        containerView.addSubview(iconImageView)
    }
    
    private func setupConstraints() {
        containerView.translatesAutoresizingMaskIntoConstraints = false
        titleLabel.translatesAutoresizingMaskIntoConstraints = false
        iconImageView.translatesAutoresizingMaskIntoConstraints = false
        
        NSLayoutConstraint.activate([
            // Container View
            containerView.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            containerView.centerYAnchor.constraint(equalTo: view.centerYAnchor),
            containerView.widthAnchor.constraint(equalToConstant: 300),
            containerView.heightAnchor.constraint(equalToConstant: 200),
            
            // Title Label
            titleLabel.topAnchor.constraint(equalTo: containerView.topAnchor, constant: 20),
            titleLabel.leadingAnchor.constraint(equalTo: containerView.leadingAnchor, constant: 20),
            titleLabel.trailingAnchor.constraint(equalTo: containerView.trailingAnchor, constant: -20),
            
            // Icon Image View
            iconImageView.topAnchor.constraint(equalTo: titleLabel.bottomAnchor, constant: 20),
            iconImageView.centerXAnchor.constraint(equalTo: containerView.centerXAnchor),
            iconImageView.widthAnchor.constraint(equalToConstant: 60),
            iconImageView.heightAnchor.constraint(equalToConstant: 60)
        ])
    }
}`,
                'README.md': `# ${this.projectName} - Swift UIKit Code

This Swift code was automatically generated from Figma design.

## Error Information
${errorMessage}

## Files

- ViewController.swift: Main view controller
- *View.swift: Individual UI components  
- UIView+Extensions.swift: Useful UIView extensions (optional)
- ResourceManager.swift: Resource management utilities (optional)
- ConstraintHelpers.swift: Auto Layout helper methods (optional)

## Usage

1. Add these files to your Xcode project
2. Import the image assets to your project's asset catalog
3. Update the view controller class name if needed
4. Customize the code as needed for your app

## Generated Information

- Project: ${this.projectName}
- Generated on: ${new Date().toLocaleString()}
- Export Config: ${JSON.stringify(this.exportConfig, null, 2)}

## Troubleshooting

If you encounter issues:
1. Check your Figma project permissions
2. Ensure the project has valid nodes
3. Try regenerating the code with different settings
4. Contact support if the problem persists
`
            };
            this.activeFile = 'ViewController.swift';
        }
    }
};

// 初始化Vue应用
document.addEventListener('DOMContentLoaded', function() {
    new Vue(CodePreviewApp).$mount('#code-preview-app');
});

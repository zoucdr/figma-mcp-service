# 代码生成提示词功能测试

## 功能概述
在个人资料页面添加了代码生成提示词配置功能，参考AI提示词（修饰提示词）的实现方式。

## 测试步骤

### 1. 数据库迁移
首先需要执行SQL脚本添加新字段：
```sql
-- 执行 add_code_prompts_field.sql
ALTER TABLE users ADD COLUMN code_prompts TEXT COMMENT '代码生成提示词，用于Swift等代码生成';
```

### 2. 前端界面测试

#### 2.1 访问个人资料页面
1. 启动服务：`./figma-deliver-service` 或 `go run main.go`
2. 访问：`http://localhost:8080/profile`
3. 登录后应该能看到个人资料页面

#### 2.2 验证代码生成提示词区域
1. 在右侧栏应该能看到新增的"代码生成提示词"卡片
2. 卡片图标应该是文档图标（el-icon-document）
3. 卡片应该可以折叠/展开
4. 应该有"分享提示词"按钮

#### 2.3 测试提示词输入
1. 在代码生成提示词文本框中输入测试内容
2. 验证字符计数功能（最多16k字符）
3. 验证提示信息显示正确

#### 2.4 测试保存功能
1. 填写代码生成提示词内容
2. 点击"保存所有设置"按钮
3. 验证保存成功提示
4. 刷新页面，验证内容是否保存

#### 2.5 测试分享功能
1. 填写代码生成提示词内容
2. 点击"分享提示词"按钮
3. 验证分享对话框打开
4. 填写标题和描述
5. 验证提示词内容自动填充
6. 测试提交分享功能

### 3. 后端API测试

#### 3.1 测试个人资料更新API
```bash
# 使用curl测试更新接口
curl -X POST http://localhost:8080/profile/update \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=testuser&figma_token=test_token&comp_types=Button,Input&prompts=修饰提示词测试&code_prompts=代码生成提示词测试"
```

#### 3.2 验证数据库存储
```sql
-- 查询用户表验证字段是否正确保存
SELECT id, username, prompts, code_prompts FROM users WHERE username = 'testuser';
```

### 4. 功能验证清单

- [ ] 数据库字段添加成功
- [ ] 前端界面显示正常
- [ ] 代码生成提示词卡片可以折叠/展开
- [ ] 文本框输入功能正常
- [ ] 字符计数功能正常
- [ ] 保存功能正常
- [ ] 分享对话框功能正常
- [ ] 分享提交功能正常
- [ ] 后端API接收参数正常
- [ ] 数据库存储功能正常
- [ ] 页面刷新后数据保持

## 预期结果

1. **界面展示**：在个人资料页面右侧应该看到新的"代码生成提示词"配置区域
2. **数据保存**：填写的代码生成提示词应该能够正确保存到数据库
3. **数据回显**：刷新页面后，之前保存的内容应该正确显示
4. **分享功能**：应该能够分享代码生成提示词到提示词广场

## 注意事项

1. 确保数据库已经添加了 `code_prompts` 字段
2. 确保前端JavaScript正确处理新的字段
3. 确保后端控制器正确接收和处理新的参数
4. 测试时注意检查浏览器控制台是否有JavaScript错误

## 相关文件

- 数据库模型：`internal/models/user.go`
- 后端控制器：`internal/controllers/auth.go`
- 前端模板：`web/templates/profile.html`
- 前端脚本：`web/static/js/profile.js`
- 数据库迁移：`add_code_prompts_field.sql`

# 禁止 Token 切换优化

## 问题描述

之前的系统存在一个严重问题：**文件缓存刷新时，会查找项目获取用户的 token，而不是使用缓存记录中指定的 token**。这导致系统可能使用其他已冷却的 token 去请求 Figma API，违反了 token 冷却管理的设计原则。

## 根本原因

1. **`figma_file_caches` 表缺少 `figma_token` 字段**
   - `figma_render_queues` 表有 `figma_token` 字段 ✅
   - `figma_file_caches` 表**没有** `figma_token` 字段 ❌

2. **调度器逻辑错误**
   - 文件缓存处理器通过查找项目和用户来获取 token
   - 这可能导致使用与缓存记录创建时不同的 token

## 解决方案

### 1. 数据库架构调整

为 `figma_file_caches` 表添加 `figma_token` 字段：

```sql
-- migrations/add_figma_token_to_file_caches.sql
ALTER TABLE `figma_file_caches` 
ADD COLUMN `figma_token` VARCHAR(255) NOT NULL 
COMMENT '必须使用此token请求Figma API' 
AFTER `id`;

ADD INDEX `idx_figma_token` (`figma_token`);
```

### 2. 模型层修改

```go
// internal/models/cache.go
type FigmaFileCache struct {
    ID           uint   `gorm:"primaryKey" json:"id"`
    FigmaToken   string `gorm:"size:255;not null;index:idx_figma_token" json:"figma_token"` // 新增
    FileKey      string `gorm:"size:100;not null;uniqueIndex:idx_file_root" json:"file_key"`
    RootNodeID   string `gorm:"size:100;not null;uniqueIndex:idx_file_root" json:"root_node_id"`
    // ... 其他字段
}
```

### 3. 服务层修改

#### CreateFileCache 方法签名调整

```go
// 修改前
func (cs *CacheService) CreateFileCache(fileKey, rootNodeID string, nodeIDs []string) (*models.FigmaFileCache, error)

// 修改后
func (cs *CacheService) CreateFileCache(fileKey, rootNodeID, token string, nodeIDs []string) (*models.FigmaFileCache, error)
```

#### 调度器逻辑优化

```go
// internal/services/scheduler.go - processFileCaches()

// 修改前：从项目查找 token
var project models.FigmaProject
err := models.DB.Where("file_key = ? AND root_node_id = ?", cache.FileKey, cache.RootNodeID).
    First(&project).Error
// ...
var user models.User
err = models.DB.First(&user, project.UserID).Error
token := user.FigmaToken

// 修改后：直接使用缓存记录中的 token
if cache.FigmaToken == "" {
    log.Printf("❌ [文件缓存处理器] 缓存记录缺少 figma_token 字段")
    continue
}
token := cache.FigmaToken
```

### 4. 控制器层修改

所有创建缓存记录的地方都需要传入 token：

```go
// internal/controllers/cache.go

// 获取用户 token
var project models.FigmaProject
models.DB.Where("file_key = ? AND root_node_id = ?", req.FileKey, req.RootNodeID).
    First(&project)
var user models.User
models.DB.First(&user, project.UserID)

// 创建缓存时传入 token
cache, err = cc.cacheService.CreateFileCache(
    req.FileKey, 
    req.RootNodeID, 
    user.FigmaToken,  // 新增参数
    []string{},
)
```

## 核心原则

### ✅ 正确做法

1. **记录创建时**：将 token 存储到 `figma_token` 字段
2. **记录处理时**：必须使用记录中的 `figma_token`，禁止查找其他 token
3. **冷却检查**：使用记录中的 token 检查冷却状态

```go
// ✅ 正确示例
token := cache.FigmaToken  // 或 queue.FigmaToken
canRequest, _, err := CheckFileAPICooldown(token)
if canRequest {
    CallFigmaAPI(token, ...)
}
```

### ❌ 错误做法

1. **禁止**通过项目查找用户 token
2. **禁止**使用其他已冷却的 token
3. **禁止**在处理时切换 token

```go
// ❌ 错误示例
var project models.FigmaProject
DB.Where("file_key = ?", cache.FileKey).First(&project)
var user models.User
DB.First(&user, project.UserID)
token := user.FigmaToken  // 错误！可能与创建时的 token 不同
```

## 影响范围

### 修改的文件

1. **模型层**
   - `internal/models/cache.go` - 添加 `FigmaToken` 字段

2. **服务层**
   - `internal/services/cache.go` - 修改 `CreateFileCache` 签名
   - `internal/services/scheduler.go` - 优化文件缓存处理器逻辑
   - `internal/services/figma.go` - 创建缓存时传入 token

3. **控制器层**
   - `internal/controllers/cache.go` - 调用创建缓存时传入 token

4. **数据库迁移**
   - `migrations/add_figma_token_to_file_caches.sql`

### 数据迁移策略

```sql
-- 1. 添加字段（先允许 NULL）
ALTER TABLE `figma_file_caches` ADD COLUMN `figma_token` VARCHAR(255) DEFAULT NULL;

-- 2. 从关联项目填充数据
UPDATE `figma_file_caches` fc
INNER JOIN `figma_projects` fp ON fc.file_key = fp.file_key AND fc.root_node_id = fp.root_node_id
INNER JOIN `users` u ON fp.user_id = u.id
SET fc.figma_token = u.figma_token
WHERE fc.figma_token IS NULL;

-- 3. 删除无法填充的孤立记录
DELETE FROM `figma_file_caches` WHERE `figma_token` IS NULL OR `figma_token` = '';

-- 4. 设置为 NOT NULL
ALTER TABLE `figma_file_caches` MODIFY COLUMN `figma_token` VARCHAR(255) NOT NULL;
```

## 测试要点

1. **创建缓存记录**
   - 检查 `figma_token` 字段是否正确填充
   - 日志应显示：`token=figd_xxx...`

2. **处理缓存队列**
   - 确认使用缓存记录中的 token
   - 禁止查找项目或用户的 token

3. **冷却管理**
   - Token 冷却状态正确检查
   - 冷却记录使用正确的 token

4. **边界情况**
   - 缓存记录缺少 token 时正确报错
   - 不会因为其他 token 已冷却而切换

## 日志示例

### 创建缓存

```log
✅ [FileCache] 已创建缓存记录 id=123, fileKey=abc, rootNodeID=1:2, token=figd_BTd-C..., status=waiting
```

### 处理缓存

```log
🔍 [文件缓存处理器] 处理缓存记录 (CacheID=123, Token=figd_BTd-C..., FileKey=abc)
⏰ [文件缓存处理器] Token 冷却中，延迟处理 (CacheID=123, Token=figd_BTd-C..., WaitSeconds=3600)
🔄 [文件缓存处理器] 开始刷新节点树 (CacheID=123, Token=figd_BTd-C..., FileKey=abc, RootNodeID=1:2)
✅ [文件缓存处理器] 刷新成功 (CacheID=123, Token=figd_BTd-C..., NodeCount=150)
```

## 升级步骤

1. **停止服务**
   ```bash
   systemctl stop figma-service
   ```

2. **备份数据库**
   ```bash
   mysqldump -u root -p figma_deliver > backup_$(date +%Y%m%d).sql
   ```

3. **执行迁移**
   ```bash
   mysql -u root -p figma_deliver < migrations/add_figma_token_to_file_caches.sql
   ```

4. **部署新版本**
   ```bash
   cp figma-service.exe /path/to/production/
   ```

5. **启动服务**
   ```bash
   systemctl start figma-service
   ```

6. **检查日志**
   ```bash
   tail -f /var/log/figma-service/app.log | grep "文件缓存处理器"
   ```

## 总结

这次优化确保了：
✅ 每个缓存/队列记录都有明确的 token 绑定
✅ 处理时必须使用记录中的 token，不允许切换
✅ Token 冷却管理更加精确和可靠
✅ 符合 Figma API 速率限制的设计原则

**关键原则：一个缓存/队列记录，一个 token，绝不切换！**


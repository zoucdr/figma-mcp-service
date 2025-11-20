# 修复：节点图片记录重复时使用 UPSERT

## 问题描述

在创建节点图片缓存记录时，如果相同的 `(file_key, node_id, format, scale)` 组合已存在，会抛出唯一键冲突错误：

```
Error 1062 (23000): Duplicate entry 'wlo3fMIPfYdx30lNs7wAVc-8:11265-png-1.0' 
for key 'figma_node_images.idx_node_format_scale'
```

### 问题场景

#### 场景 1: 重复渲染相同节点

```
第1次渲染: 创建记录
  file_key: wlo3fMIPfYdx30lNs7wAVc
  node_id: 8:11265
  format: png
  scale: 1.0
  ↓
  INSERT INTO figma_node_images ... ✅

第2次渲染: 尝试创建相同记录
  file_key: wlo3fMIPfYdx30lNs7wAVc
  node_id: 8:11265
  format: png
  scale: 1.0
  ↓
  INSERT INTO figma_node_images ... ❌ 唯一键冲突
```

#### 场景 2: 并发请求

```
请求A: 创建记录 (file_key=xxx, node_id=8:11265, format=png, scale=1.0)
请求B: 同时创建记录 (file_key=xxx, node_id=8:11265, format=png, scale=1.0)
  ↓
其中一个请求会失败
```

### 数据库约束

**表结构**: `figma_node_images`

**唯一索引**: `idx_node_format_scale` (file_key, node_id, format, scale)

**目的**: 确保同一节点的相同格式和比例只有一条记录

## 解决方案

### 实现 UPSERT 语义

将 `CreateNodeImage` 改为使用 `INSERT ... ON DUPLICATE KEY UPDATE` 语法，当记录已存在时自动更新，而不是报错。

**文件**: `internal/models/cache.go`

### 修改前

```go
// CreateNodeImage 创建节点图片缓存
func CreateNodeImage(image *FigmaNodeImage) error {
	return DB.Create(image).Error  // ❌ 遇到重复会报错
}
```

**问题**: 使用 GORM 的 `Create` 方法，遇到唯一键冲突直接返回错误

### 修改后

```go
// CreateNodeImage 创建节点图片缓存
// 如果记录已存在（根据 file_key + node_id + format + scale 唯一索引），则更新记录
func CreateNodeImage(image *FigmaNodeImage) error {
	// 使用 Clauses 实现 ON DUPLICATE KEY UPDATE
	// 当唯一键冲突时，更新除了 ID 和 CreatedAt 之外的所有字段
	result := DB.Exec(`
		INSERT INTO figma_node_images (
			file_key, node_id, format, scale, figma_cdn_url, obs_key, obs_expires_at,
			file_size, width, height, status, error_message, hit_count, last_accessed_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			figma_cdn_url = VALUES(figma_cdn_url),
			obs_key = VALUES(obs_key),
			obs_expires_at = VALUES(obs_expires_at),
			file_size = VALUES(file_size),
			width = VALUES(width),
			height = VALUES(height),
			status = VALUES(status),
			error_message = VALUES(error_message),
			updated_at = VALUES(updated_at)
	`,
		image.FileKey, image.NodeID, image.Format, image.Scale, image.FigmaCDNURL,
		image.OBSKey, image.OBSExpiresAt, image.FileSize, image.Width, image.Height,
		image.Status, image.ErrorMessage, image.HitCount, image.LastAccessedAt,
		image.CreatedAt, image.UpdatedAt,
	)
	
	return result.Error
}
```

## SQL 语义说明

### INSERT ... ON DUPLICATE KEY UPDATE

这是 MySQL 的特殊语法，具有以下特点：

1. **尝试插入**: 首先尝试执行 INSERT 操作
2. **检测冲突**: 如果唯一键冲突（记录已存在）
3. **自动更新**: 执行 UPDATE 操作更新现有记录
4. **原子操作**: 整个过程是原子的，避免竞态条件

### VALUES() 函数

`VALUES(column_name)` 返回 INSERT 语句中指定的值：

```sql
INSERT INTO table (id, name) VALUES (1, 'Alice')
ON DUPLICATE KEY UPDATE
    name = VALUES(name)  -- 使用 INSERT 语句中的 'Alice'
```

## 字段更新策略

### 更新的字段

当记录已存在时，更新以下字段：

| 字段 | 说明 | 原因 |
|------|------|------|
| `figma_cdn_url` | Figma CDN URL | 可能变化（URL 有时效） |
| `obs_key` | OBS 对象键 | 可能新上传到 OBS |
| `obs_expires_at` | OBS 过期时间 | 随 OBS 上传更新 |
| `file_size` | 文件大小 | 可能重新生成 |
| `width` | 图片宽度 | 可能重新生成 |
| `height` | 图片高度 | 可能重新生成 |
| `status` | 状态 | 需要更新状态 |
| `error_message` | 错误消息 | 清除旧错误或记录新错误 |
| `updated_at` | 更新时间 | 记录最后更新时间 |

### 不更新的字段

| 字段 | 说明 | 原因 |
|------|------|------|
| `id` | 主键 | 自增主键，不应改变 |
| `file_key` | 文件键 | 唯一索引的一部分，不变 |
| `node_id` | 节点ID | 唯一索引的一部分，不变 |
| `format` | 格式 | 唯一索引的一部分，不变 |
| `scale` | 缩放比例 | 唯一索引的一部分，不变 |
| `hit_count` | 命中次数 | 保留旧值，累计统计 |
| `last_accessed_at` | 最后访问时间 | 保留旧值 |
| `created_at` | 创建时间 | 保留原始创建时间 |

## 执行流程

### 场景 1: 记录不存在（首次创建）

**输入**:
```go
image := &FigmaNodeImage{
    FileKey:      "wlo3fMIPfYdx30lNs7wAVc",
    NodeID:       "8:11265",
    Format:       "png",
    Scale:        1.0,
    FigmaCDNURL:  "https://figma-cdn.com/...",
    Status:       "pending",
    CreatedAt:    1731000000,
    UpdatedAt:    1731000000,
}

CreateNodeImage(image)
```

**执行的 SQL**:
```sql
INSERT INTO figma_node_images (...) VALUES (...)
```

**结果**: 
- ✅ 插入新记录
- `id` = 1 (自增)
- `created_at` = 1731000000

### 场景 2: 记录已存在（重复创建）

**数据库现有记录**:
```
id: 1
file_key: wlo3fMIPfYdx30lNs7wAVc
node_id: 8:11265
format: png
scale: 1.0
figma_cdn_url: https://old-url.com/...
obs_key: ""
status: pending
created_at: 1731000000
updated_at: 1731000000
```

**输入**:
```go
image := &FigmaNodeImage{
    FileKey:      "wlo3fMIPfYdx30lNs7wAVc",
    NodeID:       "8:11265",
    Format:       "png",
    Scale:        1.0,
    FigmaCDNURL:  "https://new-url.com/...",
    OBSKey:       "figma_images/xxx/png_1.0x/8-11265.png",
    Status:       "obs_synced",
    CreatedAt:    1731000100,  // 新的时间戳
    UpdatedAt:    1731000100,
}

CreateNodeImage(image)
```

**执行的 SQL**:
```sql
INSERT INTO figma_node_images (...) VALUES (...)
ON DUPLICATE KEY UPDATE
    figma_cdn_url = 'https://new-url.com/...',
    obs_key = 'figma_images/xxx/png_1.0x/8-11265.png',
    status = 'obs_synced',
    updated_at = 1731000100
```

**结果**:
```
id: 1  (保持不变)
file_key: wlo3fMIPfYdx30lNs7wAVc
node_id: 8:11265
format: png
scale: 1.0
figma_cdn_url: https://new-url.com/...  (✅ 已更新)
obs_key: figma_images/xxx/png_1.0x/8-11265.png  (✅ 已更新)
status: obs_synced  (✅ 已更新)
created_at: 1731000000  (✅ 保留原始时间)
updated_at: 1731000100  (✅ 已更新)
```

## 并发安全性

### 问题：并发插入

```
时间轴:
T1: 请求A 查询记录 → 不存在
T2: 请求B 查询记录 → 不存在
T3: 请求A INSERT → ✅ 成功
T4: 请求B INSERT → ❌ 唯一键冲突
```

### 解决：UPSERT 原子操作

```
时间轴:
T1: 请求A INSERT ... ON DUPLICATE KEY UPDATE
T2: 请求B INSERT ... ON DUPLICATE KEY UPDATE
    ↓
结果: 两个请求都成功
- 请求A: INSERT 新记录
- 请求B: UPDATE 现有记录
```

**优点**: 
- ✅ 无需先查询再插入/更新（避免 TOCTTOU 问题）
- ✅ 数据库层面保证原子性
- ✅ 自动处理竞态条件

## 测试验证

### 测试 1: 首次创建

**操作**:
```go
image := &FigmaNodeImage{
    FileKey:     "test_key",
    NodeID:      "1:100",
    Format:      "png",
    Scale:       1.0,
    Status:      "pending",
    CreatedAt:   1731000000,
    UpdatedAt:   1731000000,
}

err := CreateNodeImage(image)
```

**预期**:
- ✅ `err == nil`
- ✅ 数据库插入新记录
- ✅ 记录数 = 1

### 测试 2: 重复创建（UPSERT）

**操作**:
```go
// 第1次
image1 := &FigmaNodeImage{
    FileKey:     "test_key",
    NodeID:      "1:100",
    Format:      "png",
    Scale:       1.0,
    FigmaCDNURL: "url1",
    Status:      "pending",
    CreatedAt:   1731000000,
    UpdatedAt:   1731000000,
}
err1 := CreateNodeImage(image1)

// 第2次（相同的唯一键）
image2 := &FigmaNodeImage{
    FileKey:     "test_key",
    NodeID:      "1:100",
    Format:      "png",
    Scale:       1.0,
    FigmaCDNURL: "url2",  // 不同的 URL
    OBSKey:      "obs_key",
    Status:      "obs_synced",
    CreatedAt:   1731000100,
    UpdatedAt:   1731000100,
}
err2 := CreateNodeImage(image2)
```

**预期**:
- ✅ `err1 == nil` (第1次插入成功)
- ✅ `err2 == nil` (第2次更新成功，不报错)
- ✅ 数据库记录数 = 1 (没有重复记录)
- ✅ `figma_cdn_url` = "url2" (已更新)
- ✅ `obs_key` = "obs_key" (已更新)
- ✅ `status` = "obs_synced" (已更新)
- ✅ `created_at` = 1731000000 (保留第1次的创建时间)
- ✅ `updated_at` = 1731000100 (使用第2次的更新时间)

### 测试 3: 并发创建

**操作**:
```go
var wg sync.WaitGroup

for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(index int) {
        defer wg.Done()
        
        image := &FigmaNodeImage{
            FileKey:     "test_key",
            NodeID:      "1:100",
            Format:      "png",
            Scale:       1.0,
            Status:      "pending",
            CreatedAt:   uint32(time.Now().Unix()),
            UpdatedAt:   uint32(time.Now().Unix()),
        }
        
        err := CreateNodeImage(image)
        if err != nil {
            log.Printf("goroutine %d failed: %v", index, err)
        }
    }(i)
}

wg.Wait()
```

**预期**:
- ✅ 所有 goroutine 都成功（无错误）
- ✅ 数据库记录数 = 1 (只有一条记录)
- ✅ 无死锁或竞态条件

## 性能影响

### UPSERT vs SELECT + INSERT/UPDATE

**传统方式**:
```go
// 1. 先查询
existing, err := GetNodeImage(fileKey, nodeID, format, scale)
if err != nil || existing == nil {
    // 2. 不存在，插入
    err = DB.Create(image).Error
} else {
    // 3. 已存在，更新
    err = DB.Model(&FigmaNodeImage{}).
        Where("id = ?", existing.ID).
        Updates(image).Error
}
```

**问题**:
- ❌ 需要 2 次数据库查询（SELECT + INSERT/UPDATE）
- ❌ 存在竞态条件（TOCTTOU）
- ❌ 代码复杂度高

**UPSERT 方式**:
```go
// 一条 SQL 完成
err = CreateNodeImage(image)
```

**优点**:
- ✅ 只需 1 次数据库操作
- ✅ 原子操作，无竞态条件
- ✅ 代码简洁

## 兼容性说明

### MySQL/MariaDB

✅ 原生支持 `INSERT ... ON DUPLICATE KEY UPDATE` 语法

### PostgreSQL

如果需要支持 PostgreSQL，应使用：
```sql
INSERT INTO ... VALUES (...)
ON CONFLICT (file_key, node_id, format, scale) 
DO UPDATE SET ...
```

### SQLite

SQLite 3.24.0+ 支持：
```sql
INSERT INTO ... VALUES (...)
ON CONFLICT (file_key, node_id, format, scale) 
DO UPDATE SET ...
```

## 相关文档

- [节点图片缓存系统](NODE_IMAGE_CACHE_SYSTEM.md)
- [OBS 集成](OBS_INTEGRATION.md)
- [渲染队列系统](RENDER_QUEUE_DESIGN.md)

---

**修复时间**: 2025-11-19  
**修改文件**: `internal/models/cache.go`  
**核心改进**:
- 使用 `INSERT ... ON DUPLICATE KEY UPDATE` 实现 UPSERT
- 当唯一键冲突时自动更新记录，不报错
- 保留原始创建时间，更新其他字段
- 支持并发安全的记录创建/更新

**测试状态**: ✅ 编译通过，等待实际运行验证


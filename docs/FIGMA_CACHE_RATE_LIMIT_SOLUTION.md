
# Figma API 缓存与速率限制优化方案

## 文档版本
- 创建日期：2025-11-19
- 版本：v1.1
- 最后更新：2025-11-19

## 关键调整要点

1. ✅ **速率限制**: Figma API 冷却时间统一为 **30秒**
2. ✅ **Token 查询**: 直接使用 `figma_token`，不使用 `token_hash`
3. ✅ **队列管理**: `figma_render_queues` 不使用 `user_id`，只用 `figma_token`
4. ✅ **强制刷新**: 节点树刷新无 `force` 参数，必然排队刷新
5. ✅ **时间格式**: 所有时间字段使用 Unix 时间戳（秒，int32）
6. ✅ **图片接口**: 获取节点图片接口直接返回图片，不返回 JSON
7. ✅ **无需清理**: 不实现过期清理功能（本地缓存有其他机制）
8. ✅ **OBS 过期**: OBS 文件过期时间为 **1个月**，不关心 Figma CDN 过期

## 一、背景与目标

### 1.1 问题背景
Figma API（`https://api.figma.com/v1/*`）存在严格的速率限制：
- **Get File API**: 每30秒只能请求一次
- **Get Images API**: 每30秒只能请求一次
- **超限后果**: 返回 429 (Too Many Requests)，需要等待冷却时间

### 1.2 解决目标
1. **缓存优先**: 减少对 Figma API 的直接请求
2. **队列管理**: 实现请求队列，防止并发冲突
3. **速率控制**: 确保请求间隔符合 Figma API 限制
4. **云端备份**: 将图片缓存到华为云 OBS，提高可用性
5. **用户体验**: 前端实时显示队列状态和渲染进度

---

## 二、数据库设计

### 2.1 Token 冷却管理表 (figma_token_cooldowns)

**用途**: 记录每个 Token 的最后请求时间，确保速率限制（30秒间隔）

```sql
CREATE TABLE `figma_token_cooldowns` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `figma_token` VARCHAR(255) NOT NULL COMMENT 'Figma Token',
  `last_file_request_time` INT UNSIGNED DEFAULT 0 COMMENT '最后一次文件树请求时间（Unix时间戳，秒）',
  `last_image_request_time` INT UNSIGNED DEFAULT 0 COMMENT '最后一次渲染图请求时间（Unix时间戳，秒）',
  `created_at` INT UNSIGNED NOT NULL COMMENT '创建时间（Unix时间戳，秒）',
  `updated_at` INT UNSIGNED NOT NULL COMMENT '更新时间（Unix时间戳，秒）',
  UNIQUE INDEX `idx_figma_token` (`figma_token`),
  INDEX `idx_last_file_request` (`last_file_request_time`),
  INDEX `idx_last_image_request` (`last_image_request_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Figma Token 冷却管理表';
```

**字段说明**:
- `figma_token`: Figma Token，直接存储用于查询
- `last_file_request_time`: 最后一次调用 Get File API 的时间（Unix时间戳）
- `last_image_request_time`: 最后一次调用 Get Images API 的时间（Unix时间戳）
- **冷却时间**: 30秒，即两次请求间隔必须 >= 30秒

### 2.2 节点树缓存表 (figma_file_caches)

**用途**: 缓存 Figma 文件节点树数据，避免重复请求。支持完整文件树和子树缓存。

```sql
CREATE TABLE `figma_file_caches` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `file_key` VARCHAR(100) NOT NULL COMMENT 'Figma 文件 Key',
  `root_node_id` VARCHAR(100) NOT NULL COMMENT '根节点 ID（空字符串表示整个文件）',
  `node_ids` TEXT COMMENT '该树包含的所有节点 ID 列表（逗号分隔，用于子树查找）',
  `file_data` MEDIUMTEXT NOT NULL COMMENT '节点树 JSON 数据（最大 16MB）',
  `file_version` VARCHAR(100) COMMENT 'Figma 文件版本号',
  `status` ENUM('waiting', 'loading', 'loaded', 'error') NOT NULL DEFAULT 'waiting' COMMENT '状态',
  `error_message` TEXT COMMENT '错误信息',
  `next_available_time` INT UNSIGNED DEFAULT 0 COMMENT '下次可请求时间（Unix时间戳，秒）',
  `hit_count` INT UNSIGNED DEFAULT 0 COMMENT '缓存命中次数',
  `created_at` INT UNSIGNED NOT NULL COMMENT '创建时间（Unix时间戳，秒）',
  `updated_at` INT UNSIGNED NOT NULL COMMENT '更新时间（Unix时间戳，秒）',
  UNIQUE INDEX `idx_file_root` (`file_key`, `root_node_id`),
  INDEX `idx_status` (`status`),
  INDEX `idx_next_available_time` (`next_available_time`),
  INDEX `idx_file_key` (`file_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Figma 文件节点树缓存表';
```

**字段说明**:
- `root_node_id`: 该缓存记录的根节点ID
  - 空字符串 `""` 表示整个文件
  - 具体节点ID（如 `"1:123"`）表示某个子树
- `node_ids`: **⚠️ 重要** - 该树包含的所有节点ID列表（逗号分隔）
  - **用途1 - 子树查找**: 当查询某个节点时，通过此字段快速判断是否在已缓存的树中
  - **用途2 - 子树提取**: 如果节点在 `node_ids` 中，从 `file_data` 提取对应子树返回
  - **示例**: `"1:123,1:124,2:456,2:457"` 表示该缓存包含这4个节点
- `status`: 
  - `waiting`: 等待下载
  - `loading`: 正在下载
  - `loaded`: 已加载成功
  - `error`: 下载失败
- `next_available_time`: 下次可请求时间（Unix时间戳），用于冷却控制
- `hit_count`: 统计缓存使用频率，便于优化

**子树查找优化示例**:
```
场景：客户端请求节点 "2:456" 的子树

步骤1：查询数据库
SELECT * FROM figma_file_caches 
WHERE file_key = 'abc123' 
  AND status = 'loaded'
  AND FIND_IN_SET('2:456', REPLACE(node_ids, ',', ','));

步骤2：如果找到包含该节点的缓存记录
  - 从 file_data (完整树) 中提取节点 "2:456" 及其子节点
  - 返回提取的子树
  - hit_count++

步骤3：如果没找到
  - 请求 Figma API 获取该子树
  - 保存为新的缓存记录（root_node_id = "2:456"）
  - 提取并保存该子树的所有节点ID到 node_ids
```

### 2.3 渲染队列表 (figma_render_queues)

**用途**: 管理图片渲染请求队列，按 Token 排队处理

```sql
CREATE TABLE `figma_render_queues` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `figma_token` VARCHAR(255) NOT NULL COMMENT 'Figma Token',
  `file_key` VARCHAR(100) NOT NULL COMMENT 'Figma 文件 Key',
  `node_ids` TEXT NOT NULL COMMENT '节点 ID 列表，逗号分隔（最多1000个）',
  `format` VARCHAR(10) NOT NULL DEFAULT 'png' COMMENT '图片格式: png, jpg, svg',
  `scale` DECIMAL(3,1) NOT NULL DEFAULT 1.0 COMMENT '图片缩放比例',
  `status` ENUM('waiting', 'processing', 'completed', 'error', 'cancelled') NOT NULL DEFAULT 'waiting' COMMENT '队列状态',
  `progress` INT UNSIGNED DEFAULT 0 COMMENT '完成进度 0-100',
  `total_nodes` INT UNSIGNED DEFAULT 0 COMMENT '总节点数',
  `processed_nodes` INT UNSIGNED DEFAULT 0 COMMENT '已处理节点数',
  `failed_nodes` INT UNSIGNED DEFAULT 0 COMMENT '失败节点数',
  `error_message` TEXT COMMENT '错误信息',
  `next_available_time` INT UNSIGNED DEFAULT 0 COMMENT '下次可请求时间（Unix时间戳，秒）',
  `started_at` INT UNSIGNED DEFAULT 0 COMMENT '开始处理时间（Unix时间戳，秒）',
  `completed_at` INT UNSIGNED DEFAULT 0 COMMENT '完成时间（Unix时间戳，秒）',
  `created_at` INT UNSIGNED NOT NULL COMMENT '创建时间（Unix时间戳，秒）',
  `updated_at` INT UNSIGNED NOT NULL COMMENT '更新时间（Unix时间戳，秒）',
  INDEX `idx_figma_token` (`figma_token`),
  INDEX `idx_status` (`status`),
  INDEX `idx_file_key` (`file_key`),
  INDEX `idx_next_available_time` (`next_available_time`),
  INDEX `idx_token_status` (`figma_token`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Figma 渲染队列表';
```

**字段说明**:
- `figma_token`: 直接使用 Figma Token 查询，不使用 user_id（因为不同用户可能访问相同文件）
- `node_ids`: 单条记录最多 1000 个节点，超过则拆分为多条记录
- `status`:
  - `waiting`: 等待处理
  - `processing`: 正在处理
  - `completed`: 已完成
  - `error`: 处理失败
  - `cancelled`: 已取消
- `progress`: 用于前端进度条显示
- `next_available_time`: 冷却时间（Unix时间戳）

### 2.4 节点图片缓存表 (figma_node_images)

**用途**: 缓存单个节点的渲染图片信息

```sql
CREATE TABLE `figma_node_images` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `file_key` VARCHAR(100) NOT NULL COMMENT 'Figma 文件 Key',
  `node_id` VARCHAR(100) NOT NULL COMMENT '节点 ID',
  `format` VARCHAR(10) NOT NULL COMMENT '图片格式',
  `scale` DECIMAL(3,1) NOT NULL COMMENT '缩放比例',
  `figma_cdn_url` VARCHAR(1000) COMMENT 'Figma CDN URL（临时，用于下载）',
  `obs_url` VARCHAR(1000) COMMENT '华为云 OBS URL',
  `obs_key` VARCHAR(500) COMMENT '华为云 OBS 存储 Key',
  `obs_expires_at` INT UNSIGNED DEFAULT 0 COMMENT 'OBS 文件过期时间（Unix时间戳，秒，1个月）',
  `file_size` BIGINT UNSIGNED COMMENT '文件大小（字节）',
  `width` INT UNSIGNED COMMENT '图片宽度',
  `height` INT UNSIGNED COMMENT '图片高度',
  `status` ENUM('pending', 'figma_cdn', 'obs_synced', 'error') NOT NULL DEFAULT 'pending' COMMENT '状态',
  `error_message` TEXT COMMENT '错误信息',
  `hit_count` INT UNSIGNED DEFAULT 0 COMMENT '访问次数',
  `last_accessed_at` INT UNSIGNED DEFAULT 0 COMMENT '最后访问时间（Unix时间戳，秒）',
  `created_at` INT UNSIGNED NOT NULL COMMENT '创建时间（Unix时间戳，秒）',
  `updated_at` INT UNSIGNED NOT NULL COMMENT '更新时间（Unix时间戳，秒）',
  UNIQUE INDEX `idx_node_format_scale` (`file_key`, `node_id`, `format`, `scale`),
  INDEX `idx_status` (`status`),
  INDEX `idx_obs_key` (`obs_key`),
  INDEX `idx_obs_expires` (`obs_expires_at`),
  INDEX `idx_last_accessed` (`last_accessed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Figma 节点图片缓存表';
```

**字段说明**:
- `status`:
  - `pending`: 待下载
  - `figma_cdn`: 已从 Figma 下载，但未同步到 OBS
  - `obs_synced`: 已同步到 OBS
  - `error`: 下载失败
- `figma_cdn_url`: 临时存储，仅用于从 Figma 下载图片
- `obs_expires_at`: OBS 文件过期时间（1个月），不关心 Figma CDN 过期
- `hit_count`: 统计热门图片，优化缓存策略

---

## 三、核心逻辑流程

### 3.1 节点树请求流程（支持子树优化）

```
客户端请求节点树（file_key + node_id）
    ↓
【步骤1：精确匹配查找】
检查 figma_file_caches 表
WHERE file_key = ? AND root_node_id = ? AND status = 'loaded'
    ↓
    ├─→ 找到精确匹配 → 直接返回缓存数据（最优情况）
    ↓
    └─→ 未找到精确匹配
        ↓
        【步骤2：子树查找优化】⚠️ 重要
        检查是否有包含该节点的父树缓存
        WHERE file_key = ? 
          AND status = 'loaded' 
          AND FIND_IN_SET(?, node_ids)  -- 在 node_ids 中查找目标节点
        ↓
        ├─→ 找到包含该节点的缓存
        │   ↓
        │   从 file_data 中提取子树
        │   ↓
        │   返回提取的子树数据
        │   ↓
        │   更新 hit_count（缓存命中）
        │   ↓
        │   【可选】保存提取的子树为新缓存记录（加速后续查询）
        ↓
        └─→ 未找到任何包含该节点的缓存
            ↓
            【步骤3：请求 Figma API】
            检查 status
            ↓
            ├─→ status = 'loading' → 等待加载完成（轮询或 WebSocket）
            ↓
            └─→ status = 'waiting' 或不存在
                ↓
                检查 Token 冷却时间 (figma_token_cooldowns.last_file_request_time)
                ↓
                ├─→ 冷却时间未到 → 创建/更新记录，设置 next_available_time，status='waiting'
                ↓
                └─→ 冷却时间已到
                    ↓
                    更新 status='loading'
                    ↓
                    调用 Figma Get File API（请求指定节点的子树）
                    ↓
                    ├─→ 成功
                    │   ↓
                    │   提取树中所有节点ID → 保存到 node_ids
                    │   ↓
                    │   保存到 figma_file_caches
                    │   ↓
                    │   status='loaded'
                    ├─→ 429 错误 → 记录错误，延长 next_available_time
                    └─→ 其他错误 → 记录错误信息
                    ↓
                    更新 figma_token_cooldowns.last_file_request_time
                    ↓
                    触发下一个等待队列
```

**关键优化点**:
1. ✅ **三级查找策略**: 精确匹配 → 子树查找 → API请求
2. ✅ **减少API调用**: 已缓存的父树可以提取出任意子树
3. ✅ **快速响应**: 子树提取比API请求快得多
4. ✅ **灵活缓存**: 可以缓存完整文件树，也可以缓存特定子树

### 3.2 渲染队列处理流程

```
客户端请求渲染
    ↓
提取节点树中的所有节点 ID
    ↓
检查 figma_node_images 表，筛选出未缓存的节点
    ↓
将节点 ID 按 1000 个一组拆分
    ↓
检查 figma_render_queues 是否有相同 file_key + figma_token + status='waiting' 的记录
    ↓
    ├─→ 存在 → 合并 node_ids（去重），更新 total_nodes
    ↓
    └─→ 不存在 → 创建新队列记录
    ↓
返回队列 ID 给客户端
    ↓
【后台定时任务 - 每 5 秒执行一次】
    ↓
查询 status='waiting' 且 next_available_time <= NOW() 的队列
    ↓
按 figma_token 分组，每个 Token 只处理一个队列
    ↓
检查 Token 冷却时间 (figma_token_cooldowns.last_image_request_time)
    ↓
    ├─→ 冷却时间未到 → 更新 next_available_time，跳过
    ↓
    └─→ 冷却时间已到
        ↓
        更新 status='processing'
        ↓
        调用 Figma Get Images API（批量请求，最多 1000 个节点）
        ↓
        ├─→ 成功
        │   ↓
        │   保存 figma_cdn_url 到 figma_node_images 表
        │   ↓
        │   异步下载图片并上传到华为云 OBS
        │   ↓
        │   更新 obs_url 和 status='obs_synced'
        │   ↓
        │   更新队列进度 (progress, processed_nodes)
        │   ↓
        │   如果全部完成 → status='completed'
        ↓
        ├─→ 429 错误
        │   ↓
        │   解析 Retry-After 响应头
        │   ↓
        │   更新 next_available_time 和 figma_token_cooldowns
        │   ↓
        │   status='waiting'，等待重试
        ↓
        └─→ 其他错误
            ↓
            记录 error_message
            ↓
            status='error'
        ↓
        更新 figma_token_cooldowns.last_image_request_time
        ↓
        触发下一个等待队列
```

### 3.3 图片加载优先级

```
客户端请求图片
    ↓
检查 figma_node_images 表
    ↓
    ├─→ obs_url 存在且未过期 → 返回 OBS URL（最高优先级）
    ↓
    ├─→ figma_cdn_url 存在 → 返回 Figma CDN URL
    ↓
    └─→ 无缓存 → 加入渲染队列，返回占位图
```

---

## 四、API 接口设计

### 4.1 节点树刷新请求

**接口**: `POST /api/figma/file/refresh`

**说明**: 强制将节点树加入刷新队列，无需 force 参数

**请求参数**:
```json
{
  "file_key": "abc123",
  "root_node_id": "1:123"  // 可选，空则刷新整个文件
}
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "cache_id": 12345,
    "status": "waiting",  // waiting | loading | loaded
    "next_available_time": 1732003800,  // Unix时间戳（秒）
    "estimated_wait_seconds": 45
  }
}
```

### 4.2 项目渲染请求

**接口**: `POST /api/figma/project/render`

**请求参数**:
```json
{
  "project_id": 1,
  "format": "png",  // png | jpg | svg
  "scale": 2.0,
  "include_ref_nodes": true  // 是否包含依赖节点
}
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "queue_ids": [123, 124, 125],
    "total_nodes": 2500,
    "estimated_duration_seconds": 300
  }
}
```

### 4.3 获取节点树请求状态

**接口**: `GET /api/figma/file/cache/status/:cache_id`

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "cache_id": 12345,
    "status": "loaded",
    "progress": 100,
    "file_data": "{ ... }",  // status='loaded' 时返回
    "error_message": null,
    "created_at": 1732003200,  // Unix时间戳（秒）
    "updated_at": 1732003290   // Unix时间戳（秒）
  }
}
```

### 4.4 获取渲染队列信息

**接口**: `GET /api/figma/render/queue`

**请求参数**:
```
?figma_token=xxx&status=processing,waiting
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "queues": [
      {
        "id": 123,
        "file_key": "abc123",
        "format": "png",
        "scale": 2.0,
        "status": "processing",
        "progress": 65,
        "total_nodes": 1000,
        "processed_nodes": 650,
        "failed_nodes": 5,
        "next_available_time": 0,
        "created_at": 1732003200,  // Unix时间戳（秒）
        "updated_at": 1732003530   // Unix时间戳（秒）
      }
    ],
    "total": 1
  }
}
```

### 4.5 获取节点图片（直接返回图片）

**接口**: `GET /api/figma/node/image`

**说明**: 直接返回图片文件，不返回 JSON

**请求参数**:
```
?file_key=abc123&node_id=1:123&format=png&scale=2.0
```

**响应**:
- **Content-Type**: `image/png` | `image/jpeg` | `image/svg+xml`
- **响应体**: 图片二进制数据
- **如果图片未就绪**: 返回占位图或 HTTP 202 Accepted，并在响应头中包含 `X-Queue-Id`

---

## 五、华为云 OBS 集成

### 5.1 配置参数

在 `configs/config.yaml` 中添加：

```yaml
huawei_cloud:
  obs:
    enabled: true
    endpoint: "obs.cn-north-4.myhuaweicloud.com"
    access_key_id: "YOUR_ACCESS_KEY"
    secret_access_key: "YOUR_SECRET_KEY"
    bucket_name: "figma-deliver-cache"
    region: "cn-north-4"
    cdn_domain: ""  # 可选，如果启用了 CDN 加速
    path_prefix: "figma_images/"  # 存储路径前缀
```

### 5.2 存储路径规则

```
{path_prefix}/{file_key}/{format}_{scale}x/{node_id}.{ext}

示例：
figma_images/abc123/png_2.0x/1-123.png
figma_images/abc123/jpg_1.0x/1-456.jpg
```

### 5.3 上传策略

1. **优先级**: Figma CDN 下载成功后，后台异步上传到 OBS
2. **并发控制**: 限制同时上传任务数（建议 5-10 个）
3. **失败重试**: 上传失败后最多重试 3 次
4. **过期时间**: OBS 文件过期时间设为 **1个月**（30天）

### 5.4 下载策略

```go
// 优先级顺序
1. 检查 OBS URL 是否存在且未过期 → 返回 OBS URL
2. 检查 Figma CDN URL 是否存在 → 从 Figma CDN 下载并上传到 OBS
3. 加入渲染队列 → 返回占位图或队列信息
```

**说明**: 不关心 Figma CDN 过期时间，只关心 OBS 过期时间（1个月）

---

## 六、定时任务设计

### 6.1 渲染队列处理器

**执行频率**: 每 5 秒

**处理逻辑**:
```go
func ProcessRenderQueue() {
    // 1. 查询待处理队列（status='waiting'，next_available_time <= NOW()）
    queues := GetWaitingQueues()
    
    // 2. 按 figma_token 分组，每个 Token 只处理一个
    for token, queue := range GroupByToken(queues) {
        // 3. 检查冷却时间
        if !CheckTokenCooldown(token, "image") {
            UpdateNextAvailableTime(queue.ID, CalculateNextTime())
            continue
        }
        
        // 4. 调用 Figma Get Images API
        result, err := CallFigmaImagesAPI(queue)
        
        // 5. 处理结果
        if err != nil {
            HandleAPIError(queue, err)
        } else {
            SaveImageCaches(result)
            AsyncUploadToOBS(result)
            UpdateQueueProgress(queue.ID)
        }
        
        // 6. 更新 Token 冷却时间
        UpdateTokenCooldown(token, "image")
    }
}
```

### 6.2 队列状态清理器（可选）

**执行频率**: 每 1 小时

**清理规则**:
1. 清理 `status='completed'` 且超过 7 天的 `figma_render_queues` 记录
2. 清理超时的 `status='loading'` 记录（超过 10 分钟未更新）

**说明**: 不需要清理缓存文件，本地缓存有其他机制处理

---

## 七、配置参数

### 7.1 configs/config.yaml

```yaml
# Figma API 配置
figma_api:
  # Get File API 冷却时间（秒）
  file_api_cooldown: 30
  
  # Get Images API 冷却时间（秒）
  images_api_cooldown: 30
  
  # 批量请求最大节点数
  max_nodes_per_request: 1000

# 缓存配置
cache:
  # OBS 文件过期时间（天）
  obs_expires_days: 30  # 1个月

# 华为云 OBS 配置
huawei_cloud:
  obs:
    enabled: true
    endpoint: "obs.cn-north-4.myhuaweicloud.com"
    access_key_id: "${HUAWEI_OBS_AK}"
    secret_access_key: "${HUAWEI_OBS_SK}"
    bucket_name: "figma-deliver-cache"
    region: "cn-north-4"
    path_prefix: "figma_images/"
    
    # 上传配置
    upload_concurrency: 5  # 并发上传数
    upload_timeout_seconds: 300

# 任务调度配置
scheduler:
  # 渲染队列处理间隔（秒）
  render_queue_interval: 5
  
  # 队列状态清理间隔（小时）
  queue_cleanup_interval: 1
```

---

## 八、监控与运维

### 8.1 关键指标监控

1. **API 调用频率**: 监控对 Figma API 的实际调用频率（应≥30秒间隔）
2. **缓存命中率**: 监控缓存命中率，优化缓存策略
3. **队列积压情况**: 监控队列长度和处理速度
4. **OBS 上传成功率**: 监控 OBS 上传失败次数
5. **错误率**: 监控 400、429 等错误的发生频率
6. **冷却时间遵守率**: 监控 Token 冷却时间是否严格遵守 30秒间隔

### 8.2 日志记录

1. **API 调用日志**: 记录每次 Figma API 调用的时间戳和 Token
2. **队列处理日志**: 记录队列创建、开始、完成的时间
3. **OBS 上传日志**: 记录上传成功/失败及文件信息
4. **错误日志**: 详细记录 429、400 等错误及响应头信息

---

## 九、总结

本方案通过四张数据库表实现了完整的缓存和队列管理机制：

1. **Token 冷却表**: 确保每个 Token 严格遵守 30秒 API 调用间隔
2. **节点树缓存表**: 缓存 Figma 文件结构，减少重复请求
3. **渲染队列表**: 按 Token 排队处理图片渲染请求
4. **图片缓存表**: 记录图片在 Figma CDN 和华为云 OBS 的 URL

核心优势：
- ✅ 有效避免 Figma API 429 错误
- ✅ 提高响应速度（缓存优先）
- ✅ 降低 API 调用成本
- ✅ 提升用户体验（队列可视化）
- ✅ 数据持久化（OBS 云端备份）


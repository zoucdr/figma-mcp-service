# Figma API 速率限制配置功能

> **新功能**：Figma Images API 的调用频率限制现已支持通过配置文件进行调整！

## 🎯 功能概述

为了防止频繁调用 Figma Images API 导致触发速率限制，我们实现了一个全局的速率限制器。**现在您可以通过配置文件灵活调整限制间隔！**

## ⚡ 快速配置

### 方法1：配置文件（推荐）

编辑 `configs/config.yaml`：

```yaml
figma_api:
  images_api_rate_limit: 10  # 单位：秒
```

### 方法2：环境变量

```bash
export FIGMA_API_RATE_LIMIT=15
```

## 📋 配置说明

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `figma_api.images_api_rate_limit` | int | 10 | API 调用间隔（秒） |

### 推荐配置值

- **开发环境**: 5-10 秒
- **生产环境**: 10-15 秒
- **高负载环境**: 15-20 秒

### 配置优先级

1. 环境变量 `FIGMA_API_RATE_LIMIT` ⭐
2. 配置文件 `config.yaml`
3. 默认值：10 秒

## 🚀 工作原理

```
请求1 → 立即执行 → 记录时间 T1
            ↓
请求2 (3秒后) → 等待7秒 → 执行 (T1+10秒)
                    ↓
                记录时间 T2
                    ↓
请求3 (15秒后) → 立即执行 (距T2已超10秒)
```

## 📊 日志示例

### 需要等待时

```
🔒 等待 Figma Images API 速率限制检查...
⏱️ Figma Images API 速率限制: 距离上次调用仅 3.5 秒，需要等待 6.5 秒
✅ Figma Images API 调用已允许，下次可调用时间: 14:30:45
```

### 无需等待时

```
🔒 等待 Figma Images API 速率限制检查...
✅ Figma Images API 调用已允许，下次可调用时间: 14:30:45
```

## 📁 相关文件

### 修改的文件

1. **configs/config.yaml** - 添加配置项
2. **main.go** - 配置加载和初始化
3. **internal/services/figma.go** - 速率限制器实现

### 新增的文档

1. **docs/figma_api_rate_limit.md** - 详细功能说明
2. **docs/CHANGELOG_FIGMA_API_RATE_LIMIT.md** - 修改记录
3. **docs/CONFIG_GUIDE.md** - 配置指南
4. **docs/README_RATE_LIMIT_CONFIG.md** - 本文档
5. **configs/config.example.yaml** - 配置示例

## 💡 使用场景

### 场景1：开发调试

```yaml
figma_api:
  images_api_rate_limit: 5  # 快速迭代
```

### 场景2：正常运行

```yaml
figma_api:
  images_api_rate_limit: 10  # 默认配置
```

### 场景3：高峰期

```yaml
figma_api:
  images_api_rate_limit: 15  # 增加间隔，避免限流
```

### 场景4：Docker 部署

```yaml
services:
  figma-deliver:
    environment:
      - FIGMA_API_RATE_LIMIT=12
```

## ✅ 特性

- ✨ **灵活配置** - 支持配置文件和环境变量
- 🔒 **全局控制** - 统一管理所有 API 调用
- 🚦 **自动排队** - 超过频率自动等待
- 🔐 **线程安全** - 支持高并发场景
- 📝 **详细日志** - 清晰的日志输出
- 🎯 **零侵入** - 对业务代码透明

## ⚠️ 注意事项

1. ⚠️ **修改配置后需要重启服务**
2. 🌐 **速率限制是全局的**，影响所有用户
3. 💾 **建议配合缓存使用**，减少 API 调用
4. ⏱️ **最小值建议不小于 5 秒**
5. 📊 **并发请求会自动排队等待**

## 🧪 测试验证

### 1. 验证配置是否生效

启动服务，查看日志输出：

```bash
./figma-deliver

# 预期输出：
# 2025/11/18 14:30:00 已加载配置: debug 模式
# 2025/11/18 14:30:00 Figma Images API 速率限制: 10 秒
# 2025/11/18 14:30:00 Services 配置已初始化: Figma API 速率限制=10秒
```

### 2. 测试速率限制

快速连续请求多个预览图，观察日志：

```bash
# 第1个请求 - 立即执行
curl http://localhost:8080/api/figma/preview/...

# 第2个请求（3秒后）- 需要等待7秒
curl http://localhost:8080/api/figma/preview/...

# 预期日志：
# ⏱️ Figma Images API 速率限制: 距离上次调用仅 3.0 秒，需要等待 7.0 秒
```

## 📚 更多文档

- [配置指南](./CONFIG_GUIDE.md) - 完整的配置说明
- [功能详解](./figma_api_rate_limit.md) - 详细的技术实现
- [修改记录](./CHANGELOG_FIGMA_API_RATE_LIMIT.md) - 完整的修改历史

## 🔧 故障排查

### 配置不生效？

1. ✅ 确认配置文件格式正确
2. ✅ 确认已重启服务
3. ✅ 检查启动日志中的配置值
4. ✅ 检查是否有环境变量覆盖

### 等待时间过长？

1. 🔍 检查配置值是否过大
2. 🔍 确认是否有大量并发请求
3. 💡 考虑启用缓存减少 API 调用

### 如何禁用速率限制？

不建议禁用，但如果必须：

```yaml
figma_api:
  images_api_rate_limit: 1  # 设置为最小值1秒
```

## 📞 技术支持

遇到问题？

1. 查看[配置指南](./CONFIG_GUIDE.md)
2. 查看服务启动日志
3. 参考[示例配置](../configs/config.example.yaml)

## 📅 更新历史

- **2025-11-18**: 初始版本
  - 实现基础速率限制功能
  - 添加配置文件支持
  - 支持环境变量覆盖
  - 完善文档

---

**版本**: v1.0.0  
**更新日期**: 2025-11-18  
**状态**: ✅ 已发布


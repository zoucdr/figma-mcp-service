# 渲染进度监控功能测试指南

## 测试环境准备

### 前置条件
1. 已启动 figma-deliver 服务
2. 已登录系统并配置了 Figma Token
3. 已创建至少一个项目并导入了节点数据

## 测试用例

### 测试用例 1: 自动启动监控

**目的：** 验证打开Dashboard页面时，系统能自动检测并显示渲染状态

**步骤：**
1. 打开任意项目的Dashboard页面
2. 在另一个标签页中，对同一项目发起渲染请求
3. 返回Dashboard页面，观察是否自动显示渲染状态

**预期结果：**
- 页面顶部导航栏显示渲染进度指示器
- 页面底部显示渲染状态栏
- 状态信息每2秒自动更新

**验证方法：**
```javascript
// 在浏览器控制台执行，检查监控是否启动
console.log(FigmaRenderManager.pollingQueues);
// 应该看到类似 Set { 'project_123' } 的输出
```

---

### 测试用例 2: 等待状态和时间预估

**目的：** 验证在Token冷却期间，系统能正确显示等待时间

**步骤：**
1. 确保Figma Token处于冷却期（可以通过连续发起多次请求触发）
2. 点击"渲染"按钮
3. 观察状态显示

**预期结果：**
- 顶部显示："等待中 (预计等待 X秒)"
- 底部显示："渲染等待中 (预计等待 X秒)"
- 等待时间每2秒递减
- 时间格式正确（秒/分钟/小时）

**测试数据：**
| 剩余秒数 | 预期显示格式 |
|---------|------------|
| 30 | 30秒 |
| 90 | 1分30秒 |
| 120 | 2分钟 |
| 3700 | 1小时1分钟 |
| 7200 | 2小时 |

---

### 测试用例 3: 渲染进行中状态

**目的：** 验证渲染过程中进度显示的准确性

**步骤：**
1. 选择一个包含较多节点的项目（建议>50个节点）
2. 点击"渲染"按钮
3. 观察进度更新

**预期结果：**
- 顶部显示："渲染中 X/Y"和进度条
- 底部显示节点处理数量和百分比
- 进度条平滑增长
- 处理数量实时更新
- 百分比计算正确

**验证方法：**
```javascript
// 在控制台查看当前进度
const app = document.getElementById('project-editor-app').__vue__;
console.log(app.projectRenderProgress);
// 应该看到类似以下输出：
// {
//   hasRender: true,
//   status: 'processing',
//   progress: 45,
//   processedNodes: 45,
//   totalNodes: 100,
//   ...
// }
```

---

### 测试用例 4: 页面刷新后恢复监控

**目的：** 验证页面刷新后，系统能继续监控渲染进度

**步骤：**
1. 启动一个渲染任务
2. 等待进度到达约50%
3. 刷新页面（F5或Ctrl+R）
4. 观察渲染状态是否恢复

**预期结果：**
- 页面加载完成后，自动显示渲染状态
- 进度继续从当前值更新（不会从0开始）
- 轮询继续进行

**验证方法：**
1. 刷新前记录当前进度值
2. 刷新后检查进度值应该大于等于刷新前的值
3. 检查控制台无错误信息

---

### 测试用例 5: 渲染完成后的处理

**目的：** 验证渲染完成后的状态更新和UI变化

**步骤：**
1. 启动一个小规模渲染任务（10-20个节点）
2. 等待渲染完成

**预期结果：**
- 显示成功提示消息："渲染完成！"
- 渲染状态栏自动隐藏
- 节点树数据自动刷新
- 预览图片更新

**验证方法：**
```javascript
// 检查渲染状态
const app = document.getElementById('project-editor-app').__vue__;
console.log(app.projectRenderProgress.status);
// 应该输出: 'completed'
```

---

### 测试用例 6: 多标签页同步

**目的：** 验证在多个标签页中打开同一项目时的行为

**步骤：**
1. 在标签页A打开项目Dashboard
2. 在标签页B打开同一项目Dashboard
3. 在标签页A启动渲染
4. 观察标签页B的状态

**预期结果：**
- 标签页B应该在2秒内自动显示渲染状态
- 两个标签页的进度应该保持同步（误差不超过一个轮询周期）

**注意：** 由于是独立轮询，两个标签页可能有最多2秒的显示差异

---

### 测试用例 7: 错误状态处理

**目的：** 验证渲染失败时的错误提示

**步骤：**
1. 配置一个无效的Figma Token
2. 尝试启动渲染
3. 观察错误提示

**预期结果：**
- 显示错误消息
- 渲染状态栏显示失败状态
- 轮询自动停止
- 用户可以重新尝试

---

### 测试用例 8: 性能测试

**目的：** 验证长时间轮询不会影响页面性能

**步骤：**
1. 启动一个大规模渲染任务（>200个节点）
2. 让页面保持打开状态30分钟以上
3. 观察浏览器性能

**预期结果：**
- CPU使用率稳定，无异常峰值
- 内存使用稳定，无内存泄漏
- 网络请求频率稳定（每2秒一次）
- 页面响应流畅，无卡顿

**验证方法：**
1. 打开Chrome DevTools -> Performance
2. 开始录制性能数据
3. 观察5-10分钟
4. 检查是否有内存持续增长或CPU异常

---

## 边界条件测试

### 边界条件 1: 无渲染任务
- **场景：** 项目没有进行中的渲染任务
- **预期：** 不显示渲染状态栏

### 边界条件 2: 极短等待时间
- **场景：** 等待时间小于1秒
- **预期：** 显示"0秒"或直接开始渲染

### 边界条件 3: 极长等待时间
- **场景：** 等待时间超过24小时
- **预期：** 正确显示小时数（如"25小时"）

### 边界条件 4: 网络断开
- **场景：** 在轮询过程中断开网络
- **预期：** 控制台记录错误，但不影响页面稳定性

### 边界条件 5: 服务器返回异常数据
- **场景：** 后端返回不完整的进度数据
- **预期：** 使用默认值填充，不导致页面崩溃

---

## 自动化测试脚本

以下是一些可以在浏览器控制台运行的测试脚本：

### 脚本 1: 检查监控状态
```javascript
function checkMonitoringStatus() {
    const app = document.getElementById('project-editor-app').__vue__;
    const progress = app.projectRenderProgress;
    
    console.log('=== 渲染进度监控状态 ===');
    console.log('是否有渲染任务:', progress.hasRender);
    console.log('渲染状态:', progress.status);
    console.log('当前进度:', progress.progress + '%');
    console.log('已处理节点:', progress.processedNodes);
    console.log('总节点数:', progress.totalNodes);
    console.log('下次可用时间:', progress.nextAvailableTime);
    
    if (progress.nextAvailableTime > 0) {
        const wait = app.calculateWaitSeconds(progress.nextAvailableTime);
        console.log('预计等待:', app.formatWaitTime(wait));
    }
    
    console.log('轮询队列:', FigmaRenderManager.pollingQueues);
}

checkMonitoringStatus();
```

### 脚本 2: 模拟进度更新
```javascript
function simulateProgress() {
    const app = document.getElementById('project-editor-app').__vue__;
    let processed = 0;
    const total = 100;
    
    const interval = setInterval(() => {
        processed += Math.floor(Math.random() * 10) + 1;
        if (processed > total) processed = total;
        
        app.projectRenderProgress = {
            hasRender: true,
            status: processed >= total ? 'completed' : 'processing',
            progress: Math.floor((processed / total) * 100),
            processedNodes: processed,
            totalNodes: total,
            message: `渲染中... ${processed}/${total}`
        };
        
        console.log(`进度: ${processed}/${total}`);
        
        if (processed >= total) {
            clearInterval(interval);
            console.log('模拟渲染完成');
        }
    }, 1000);
}

simulateProgress();
```

### 脚本 3: 测试等待时间格式化
```javascript
function testWaitTimeFormat() {
    const app = document.getElementById('project-editor-app').__vue__;
    
    const testCases = [30, 90, 120, 180, 3600, 3700, 7200, 86400];
    
    console.log('=== 等待时间格式化测试 ===');
    testCases.forEach(seconds => {
        const formatted = app.formatWaitTime(seconds);
        console.log(`${seconds}秒 -> ${formatted}`);
    });
}

testWaitTimeFormat();
```

---

## 问题排查

### 问题 1: 页面不显示渲染状态

**排查步骤：**
1. 检查 `figma-render.js` 是否加载
   ```javascript
   console.log(typeof FigmaRenderManager);
   // 应该输出: 'object'
   ```

2. 检查项目是否有 render_id
   ```javascript
   const app = document.getElementById('project-editor-app').__vue__;
   console.log(app.project.render_id);
   // 应该输出: 非0的数字
   ```

3. 检查API是否返回正确数据
   ```javascript
   fetch(`/figma/project/${projectId}/render/progress`)
       .then(r => r.json())
       .then(d => console.log(d));
   ```

### 问题 2: 等待时间不显示

**排查步骤：**
1. 检查 next_available_time 字段
   ```javascript
   const app = document.getElementById('project-editor-app').__vue__;
   console.log(app.projectRenderProgress.nextAvailableTime);
   // 应该输出: Unix时间戳（秒）
   ```

2. 检查计算函数
   ```javascript
   const app = document.getElementById('project-editor-app').__vue__;
   const wait = app.calculateWaitSeconds(app.projectRenderProgress.nextAvailableTime);
   console.log('等待秒数:', wait);
   ```

### 问题 3: 轮询停止

**排查步骤：**
1. 检查轮询队列
   ```javascript
   console.log(FigmaRenderManager.pollingQueues);
   console.log(FigmaRenderManager.pollTimer);
   ```

2. 手动重启轮询
   ```javascript
   const app = document.getElementById('project-editor-app').__vue__;
   app.startWatchingProjectRender();
   ```

---

## 测试报告模板

```
测试日期: YYYY-MM-DD
测试人员: [姓名]
测试版本: [版本号]

测试结果摘要:
- 通过用例数: X / Y
- 失败用例数: X / Y
- 未执行: X / Y

详细结果:

[用例编号] [用例名称]
状态: ✅通过 / ❌失败 / ⏸未执行
备注: [说明]

问题列表:
1. [问题描述]
   - 严重程度: 高/中/低
   - 复现步骤: ...
   - 预期结果: ...
   - 实际结果: ...

建议:
- [改进建议]
```

---

## 参考资料

- [渲染进度监控功能文档](./render_progress_feature.md)
- [Figma API文档](https://www.figma.com/developers/api)
- [Vue.js响应式原理](https://vuejs.org/guide/extras/reactivity-in-depth.html)


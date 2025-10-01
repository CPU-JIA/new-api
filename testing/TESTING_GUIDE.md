# Cache Analytics Integration Testing Guide

## 概述

本指南用于系统性地测试Claude Prompt Caching优化系统的完整功能，包括后端API、前端Dashboard和端到端数据流。

**测试目标**：
- ✅ 验证数据库迁移正确
- ✅ 验证数据收集功能正常
- ✅ 验证5个API endpoints返回正确数据
- ✅ 验证前端Dashboard UI显示
- ✅ 验证权限控制（普通用户 vs 管理员）
- ✅ 验证端到端真实请求流程
- ✅ 验证CacheWarmer自动触发

---

## 前置条件

### 环境要求
- [x] Go 1.23.4 已安装
- [x] Bun包管理器已安装（前端）
- [x] 数据库（SQLite/MySQL/PostgreSQL）
- [x] 至少配置1个Claude渠道（channel_type=14或33）

### 编译确认
```bash
# 后端编译（应已完成）
cd E:\1\new-api
go build -o new-api.exe .

# 前端依赖（如需重新安装）
cd web
bun install
```

---

## 测试阶段

### 阶段1：数据库层验证 ⏱️ 预计5分钟

#### 步骤1.1：启动后端服务
```bash
# Windows
cd E:\1\new-api
new-api.exe

# 检查启动日志，确认无错误：
# - [GIN-debug] Listening and serving HTTP on :3000
# - AutoMigrate成功（无ERROR）
```

**预期结果**：
- ✅ 服务启动无报错
- ✅ 日志中看到路由注册：`/api/cache/*`
- ✅ 数据库连接成功

#### 步骤1.2：验证数据库表结构
```sql
-- SQLite
sqlite3 one-api.db

-- MySQL/PostgreSQL
mysql -u root -p

-- 1. 检查表是否存在
.tables  -- SQLite
SHOW TABLES LIKE 'prompt_cache_metrics';  -- MySQL

-- 2. 检查表结构
.schema prompt_cache_metrics  -- SQLite
DESCRIBE prompt_cache_metrics;  -- MySQL

-- 3. 检查索引
.indexes prompt_cache_metrics  -- SQLite
SHOW INDEX FROM prompt_cache_metrics;  -- MySQL
```

**预期结果**：
- ✅ `prompt_cache_metrics` 表存在
- ✅ 所有字段正确（id, created_at, channel_id, cache_hit_rate等）
- ✅ 至少看到以下索引：
  - `idx_prompt_cache_created_at`
  - `idx_prompt_cache_channel_time`
  - `idx_prompt_cache_channel_id`

#### 步骤1.3：插入测试数据
```bash
# 1. 先查询现有渠道ID
# SQLite
sqlite3 one-api.db "SELECT id, name, type FROM channels WHERE type IN (14, 33) LIMIT 3;"

# MySQL
mysql -u root -p -e "SELECT id, name, type FROM channels WHERE type IN (14, 33) LIMIT 3;"

# 2. 编辑SQL脚本
# 打开 testing/insert_test_cache_metrics.sql
# 修改channel_id为实际查询到的ID（如果只有1个渠道，全部改为该ID）

# 3. 执行插入
# SQLite
sqlite3 one-api.db < testing/insert_test_cache_metrics.sql

# MySQL
mysql -u root -p your_database < testing/insert_test_cache_metrics.sql
```

**预期结果**：
- ✅ SQL执行无错误
- ✅ 插入至少40+条记录

#### 步骤1.4：验证数据插入
```sql
-- 1. 检查总记录数
SELECT COUNT(*) as total_records FROM prompt_cache_metrics;
-- 预期：>= 40

-- 2. 检查各渠道统计
SELECT
    channel_id,
    channel_name,
    COUNT(*) as count,
    AVG(cache_hit_rate) as avg_hit_rate,
    SUM(cost_saved) as total_saved
FROM prompt_cache_metrics
WHERE is_warmup = 0
GROUP BY channel_id, channel_name;

-- 预期：至少1个渠道，hit_rate在0-1之间

-- 3. 检查时间分布
SELECT
    DATE(created_at) as date,
    COUNT(*) as count
FROM prompt_cache_metrics
GROUP BY DATE(created_at)
ORDER BY date DESC;

-- 预期：看到今天的日期和记录数

-- 4. 检查Warmup记录
SELECT COUNT(*) as warmup_count FROM prompt_cache_metrics WHERE is_warmup = 1;
-- 预期：至少3条warmup记录
```

---

### 阶段2：后端API测试 ⏱️ 预计10分钟

#### 步骤2.1：执行API测试脚本

**Windows用户**：
```bash
cd E:\1\new-api\testing
test_api_endpoints.bat
```

**Linux/Mac用户**：
```bash
cd /path/to/new-api/testing
chmod +x test_api_endpoints.sh
./test_api_endpoints.sh
```

#### 步骤2.2：手动测试关键API

##### Test 1: Overview API
```bash
curl "http://localhost:3000/api/cache/metrics/overview?period=24h"
```

**预期JSON结构**：
```json
{
  "success": true,
  "data": {
    "total_requests": 40,
    "cache_hit_rate": 0.65,
    "total_cost_saved": 150000.5,
    "estimated_warmup_cost": 3.0,
    "net_savings": 149997.5,
    "active_warmup_channels": 1,
    "period": "24h",
    "start_time": 1234567890,
    "end_time": 1234654290
  }
}
```

**验证点**：
- ✅ `success: true`
- ✅ `total_requests > 0`
- ✅ `cache_hit_rate` 在0-1之间
- ✅ `total_cost_saved >= 0`

##### Test 2: Chart Data API
```bash
curl "http://localhost:3000/api/cache/metrics/chart?period=24h&interval=1h"
```

**预期JSON结构**：
```json
{
  "success": true,
  "data": {
    "data_points": [
      {
        "time": "2025-01-01 10:00",
        "cache_hit_rate": 0.85,
        "cost_saved": 5000.5,
        "total_requests": 5
      },
      // ... more points
    ]
  }
}
```

**验证点**：
- ✅ `data_points` 数组不为空
- ✅ 每个point有 `time`, `cache_hit_rate`, `cost_saved`, `total_requests`
- ✅ 时间顺序正确

##### Test 3: Channel Metrics API（需要管理员权限）
```bash
curl "http://localhost:3000/api/cache/metrics/channels?period=24h"
```

**验证点**：
- ✅ 管理员：返回200，包含渠道列表
- ✅ 普通用户：返回403 Forbidden

##### Test 4: Warmup Status API（需要管理员权限）
```bash
curl "http://localhost:3000/api/cache/warmer/status"
```

**预期JSON结构**：
```json
{
  "success": true,
  "data": {
    "total_channels": 3,
    "channels": [
      {
        "channel_id": 1,
        "channel_name": "Claude Channel 1",
        "warmup_enabled": true,
        "last_warmup": 1234567890,
        "last_request": 1234567895,
        "request_count_5min": 12,
        "window_start": 1234567600
      }
    ]
  }
}
```

**验证点**：
- ✅ 管理员：返回200
- ✅ `channels` 数组存在
- ✅ 如果有活跃渠道（QPS>=10），`warmup_enabled: true`

---

### 阶段3：前端Dashboard测试 ⏱️ 预计15分钟

#### 步骤3.1：启动前端开发服务器
```bash
cd E:\1\new-api\web
bun run dev

# 预期输出：
# VITE v5.x.x  ready in xxx ms
# ➜  Local:   http://localhost:5173/
```

#### 步骤3.2：访问Dashboard
1. 浏览器打开：`http://localhost:5173/console/cache`
2. 打开DevTools（F12）-> Console面板

**验证点**：
- ✅ 页面加载无JavaScript错误
- ✅ 页面无白屏
- ✅ 看到"缓存分析"标题

#### 步骤3.3：检查组件加载

##### 3.3.1 Stats Cards（统计卡片）
**验证点**：
- ✅ 看到4个卡片（总请求数、缓存命中率、节省配额、净节省）
- ✅ 数字合理（非undefined或NaN）
- ✅ 命中率显示百分比（如85.50%）
- ✅ 状态Tag正确（success/warning/danger）

##### 3.3.2 Charts Panel（图表面板）
**验证点**：
- ✅ 看到2个Tab（缓存命中率趋势、节省配额趋势）
- ✅ 图表正常渲染（非空白）
- ✅ X轴显示时间
- ✅ Y轴显示数值
- ✅ 数据点存在
- ✅ 鼠标悬停显示tooltip

##### 3.3.3 Channel Table（渠道表格 - 仅管理员）
**验证点**：
- ✅ 管理员：看到渠道表格
- ✅ 表格有数据行
- ✅ 列：渠道ID、名称、请求数、缓存命中率、节省配额、Warmup状态等
- ✅ 点击列头排序功能正常
- ✅ Warmup状态显示Tag（已启用/未启用）

##### 3.3.4 Warmup Panel（Warmup面板 - 仅管理员）
**验证点**：
- ✅ 管理员：看到Warmup实时状态面板
- ✅ 顶部显示总渠道数和活跃Warmup数
- ✅ 表格显示监控中的渠道
- ✅ 最后Warmup时间显示为相对时间（如"2分钟前"）
- ✅ 5分钟请求数有数字

#### 步骤3.4：测试交互功能

##### 3.4.1 Period切换
1. 点击Period选择器
2. 切换到"1小时"、"7天"、"30天"

**验证点**：
- ✅ 每次切换触发API请求（Network面板）
- ✅ Stats和Charts数据更新
- ✅ Loading状态正确显示

##### 3.4.2 Auto-refresh功能
1. 打开Auto-refresh开关
2. 设置刷新间隔为10秒

**验证点**：
- ✅ 开关状态切换正常
- ✅ 间隔输入框可见
- ✅ 每10秒自动触发API请求
- ✅ 关闭开关后停止自动刷新

##### 3.4.3 手动刷新
1. 点击刷新按钮（🔄图标）

**验证点**：
- ✅ 按钮显示loading状态
- ✅ API请求被触发
- ✅ 数据更新
- ✅ 显示成功提示（Toast）

##### 3.4.4 Chart Tab切换
1. 点击"节省配额趋势"Tab

**验证点**：
- ✅ Tab高亮切换
- ✅ 图表切换为Area chart
- ✅ Y轴显示"节省配额"
- ✅ 数据正确显示

---

### 阶段4：权限控制测试 ⏱️ 预计5分钟

#### 步骤4.1：普通用户测试
1. 创建或切换到普通用户账号
2. 访问 `/console/cache`

**验证点**：
- ✅ 可以访问页面
- ✅ 可以看到：Stats Cards、Charts
- ❌ 不能看到：Channel Table、Warmup Panel
- ✅ User Metrics API只能查看自己的数据

#### 步骤4.2：管理员测试
1. 切换到管理员账号
2. 访问 `/console/cache`

**验证点**：
- ✅ 所有组件都可见
- ✅ Channel Table显示所有渠道
- ✅ Warmup Panel显示服务状态
- ✅ 可以查看任意用户的metrics

#### 步骤4.3：API权限绕过测试（安全测试）
1. 以普通用户登录
2. 打开DevTools -> Console
3. 尝试直接调用管理员API：
```javascript
fetch('/api/cache/metrics/channels?period=24h')
  .then(r => r.json())
  .then(console.log);
```

**预期结果**：
- ✅ 返回403 Forbidden
- ✅ 无法获取渠道数据

---

### 阶段5：端到端真实请求测试 ⏱️ 预计10分钟

#### 步骤5.1：准备测试请求

创建测试脚本 `test_claude_request.py`：
```python
import requests
import json

API_BASE = "http://localhost:3000"
TOKEN = "your_token_here"  # 替换为实际token

headers = {
    "Authorization": f"Bearer {TOKEN}",
    "Content-Type": "application/json"
}

# Claude API请求（带cache_control）
payload = {
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 500,
    "system": [
        {
            "type": "text",
            "text": "You are a helpful assistant. This is a test message for prompt caching.",
            "cache_control": {"type": "ephemeral"}
        }
    ],
    "messages": [
        {
            "role": "user",
            "content": "What is 2+2?"
        }
    ]
}

# 发送请求
print("发送Claude API请求...")
response = requests.post(
    f"{API_BASE}/v1/chat/completions",
    headers=headers,
    json=payload
)

print(f"Status: {response.status_code}")
print(f"Response: {response.json()}")

# 检查usage中的cache tokens
usage = response.json().get("usage", {})
print(f"\nUsage:")
print(f"  Prompt tokens: {usage.get('prompt_tokens')}")
print(f"  Cache read tokens: {usage.get('cache_read_tokens')}")
print(f"  Cache creation tokens: {usage.get('cache_creation_tokens')}")
print(f"  Completion tokens: {usage.get('completion_tokens')}")
```

#### 步骤5.2：执行测试请求
```bash
python test_claude_request.py
```

#### 步骤5.3：验证数据收集
等待3秒（goroutine异步插入），然后查询数据库：
```sql
-- 查看最新插入的记录
SELECT
    id, created_at, channel_id, model_name,
    prompt_tokens, cache_read_tokens, cache_creation_tokens,
    cache_hit_rate, cost_saved, is_warmup
FROM prompt_cache_metrics
ORDER BY id DESC
LIMIT 1;
```

**验证点**：
- ✅ 看到新插入的记录
- ✅ `cache_creation_tokens` 或 `cache_read_tokens` > 0
- ✅ `cache_hit_rate` 合理
- ✅ `cost_saved` >= 0
- ✅ `is_warmup = 0`（非warmup请求）

#### 步骤5.4：验证Dashboard实时更新
1. 回到浏览器Dashboard页面
2. 点击刷新按钮

**验证点**：
- ✅ 总请求数+1
- ✅ 统计数据更新
- ✅ 图表添加新数据点

---

### 阶段6：CacheWarmer功能测试 ⏱️ 预计5分钟

#### 步骤6.1：触发Warmup条件
1. 快速发送10+个Claude请求（模拟QPS>=10）
2. 等待5分钟（CacheWarmer检查周期）

或者手动触发（需要后端代码支持）：
```bash
# 查看CacheWarmer日志
tail -f new-api.log | grep "CacheWarmer"
```

#### 步骤6.2：验证Warmup执行
```sql
-- 查看warmup记录
SELECT
    id, created_at, channel_id,
    prompt_tokens, cache_creation_tokens,
    is_warmup
FROM prompt_cache_metrics
WHERE is_warmup = 1
ORDER BY id DESC
LIMIT 5;
```

**验证点**：
- ✅ 看到 `is_warmup = 1` 的记录
- ✅ `cache_creation_tokens > 0`
- ✅ `user_id = 0`, `token_id = 0`

#### 步骤6.3：验证Warmup面板显示
1. 访问Dashboard（管理员）
2. 查看Warmup Panel

**验证点**：
- ✅ 活跃Warmup渠道数 > 0
- ✅ 最后Warmup时间更新
- ✅ 5分钟请求数 >= 10
- ✅ Warmup状态Tag显示"活跃"

---

## 问题排查

### 问题1：API返回403 Forbidden
**可能原因**：
- Token无效或过期
- 用户权限不足（非管理员访问admin endpoint）

**解决方案**：
1. 检查浏览器Cookie中的session token
2. 确认用户角色（`SELECT role FROM users WHERE id=X`）
3. 确认middleware配置正确

### 问题2：Dashboard页面白屏
**可能原因**：
- JavaScript错误
- API请求失败
- 组件加载失败

**解决方案**：
1. 打开DevTools Console查看错误
2. 检查Network面板API请求状态
3. 确认VChart依赖已安装（`bun install`）

### 问题3：图表不显示数据
**可能原因**：
- API返回空数据
- 数据格式不符合VChart spec
- 时间区间无数据

**解决方案**：
1. 检查API返回的`data_points`数组
2. 确认数据库中有对应时间区间的数据
3. 切换Period到"24h"（数据最多）

### 问题4：数据收集不工作
**可能原因**：
- `recordPromptCacheMetrics` 未被调用
- 条件判断错误（`cacheTokens == 0`）
- Goroutine异常

**解决方案**：
1. 检查后端日志是否有SysError
2. 确认Claude请求有cache_control参数
3. 直接查询数据库验证是否有新记录

### 问题5：Warmup不触发
**可能原因**：
- QPS未达到阈值（默认10/5min）
- CacheWarmer服务未启动
- 时间窗口计算错误

**解决方案**：
1. 检查CacheWarmer日志
2. 手动调用 `GetCacheWarmerService().checkAndWarmup()`
3. 降低QPS阈值进行测试

---

## 测试完成Checklist

完成以下检查后，可以认为集成测试通过：

- [ ] 数据库表结构正确
- [ ] 测试数据成功插入
- [ ] 5个API endpoints全部返回200
- [ ] API数据格式符合前端期望
- [ ] Dashboard页面正常加载
- [ ] 4个Stats Cards显示正确
- [ ] 2个Charts正常渲染
- [ ] Period切换功能正常
- [ ] Auto-refresh功能正常
- [ ] 手动刷新功能正常
- [ ] 管理员可见所有组件
- [ ] 普通用户看不到admin组件
- [ ] API权限控制正常（403验证）
- [ ] 真实Claude请求后数据被记录
- [ ] Dashboard实时更新新数据
- [ ] CacheWarmer自动触发（可选）
- [ ] Warmup Panel显示正确状态

---

## 下一步

测试通过后，可以进入Phase 5（优化和监控）：
1. 性能优化（批量插入、Redis缓存）
2. 数据归档策略
3. 监控告警配置
4. 用户文档编写

---

**测试执行人**: _____________
**测试日期**: _____________
**测试结果**: ✅ PASS / ❌ FAIL
**备注**: _____________
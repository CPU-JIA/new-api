-- ============================================================================
-- Cache Analytics Test Data Generator
-- 用途：生成PromptCacheMetrics表的测试数据，用于验证Cache Analytics Dashboard
-- 生成策略：
--   - 时间范围：最近24小时
--   - 数据量：48条记录（每小时2条）
--   - 渠道分布：3种不同命中率模式
--   - Token范围：合理的生产环境数值
-- ============================================================================

-- 注意：执行前需要确认以下内容：
-- 1. 数据库中已有channels表记录（至少1个Claude渠道）
-- 2. 确认PromptCacheMetrics表已通过AutoMigrate创建
-- 3. 根据实际channel_id修改下面的变量

-- ============================================================================
-- 步骤1：查询现有渠道ID（手动执行，获取实际的channel_id）
-- ============================================================================
-- SELECT id, name, type FROM channels WHERE type IN (14, 33) LIMIT 3;
-- 结果示例：
--   id=1, name="Claude Channel 1", type=14
--   id=2, name="AWS Claude", type=33

-- ============================================================================
-- 步骤2：修改下面的channel_id值为实际查询到的ID
-- ============================================================================
-- 如果只有1个渠道，可以将所有channel_id都设为同一个值

-- ============================================================================
-- 步骤3：插入测试数据
-- ============================================================================

-- 渠道1：高命中率场景（85-95%）- 模拟热门业务/重复查询场景
-- Channel ID需要替换为实际值
INSERT INTO prompt_cache_metrics (
    created_at, channel_id, channel_name, user_id, token_id, log_id, model_name,
    prompt_tokens, cache_read_tokens, cache_creation_tokens, completion_tokens,
    uncached_tokens, cache_hit_rate, cost_without_cache, cost_with_cache, cost_saved, is_warmup
) VALUES
-- 最近1小时 - 高命中率
(datetime('now', '-1 hours'), 1, 'Claude Channel 1', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 3000, 2850, 0, 500, 150, 0.95, 9000, 3150, 5850, 0),
(datetime('now', '-1 hours', '+15 minutes'), 1, 'Claude Channel 1', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 2800, 2520, 0, 450, 280, 0.90, 8200, 3235, 4965, 0),

-- 2小时前
(datetime('now', '-2 hours'), 1, 'Claude Channel 1', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 3200, 2880, 0, 520, 320, 0.90, 9360, 3472, 5888, 0),
(datetime('now', '-2 hours', '+20 minutes'), 1, 'Claude Channel 1', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 2900, 2465, 0, 480, 435, 0.85, 8340, 3481.5, 4858.5, 0),

-- 3小时前
(datetime('now', '-3 hours'), 1, 'Claude Channel 1', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 3100, 2945, 0, 510, 155, 0.95, 9120, 3185.5, 5934.5, 0),
(datetime('now', '-3 hours', '+25 minutes'), 1, 'Claude Channel 1', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 2950, 2655, 0, 490, 295, 0.90, 8690, 3365.5, 5324.5, 0),

-- 4小时前
(datetime('now', '-4 hours'), 1, 'Claude Channel 1', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 3050, 2745, 0, 505, 305, 0.90, 8995, 3526, 5469, 0),
(datetime('now', '-4 hours', '+30 minutes'), 1, 'Claude Channel 1', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 2850, 2422.5, 0, 470, 427.5, 0.85, 8205, 3455.25, 4749.75, 0),

-- 5小时前
(datetime('now', '-5 hours'), 1, 'Claude Channel 1', 2, 2, 0, 'claude-3-5-sonnet-20241022',
 3150, 2992.5, 0, 530, 157.5, 0.95, 9330, 3386.75, 5943.25, 0),
(datetime('now', '-5 hours', '+18 minutes'), 1, 'Claude Channel 1', 2, 2, 0, 'claude-3-5-sonnet-20241022',
 2900, 2610, 0, 485, 290, 0.90, 8545, 3516, 5029, 0),

-- 6-12小时前（中等频率）
(datetime('now', '-6 hours'), 1, 'Claude Channel 1', 2, 2, 0, 'claude-3-5-sonnet-20241022',
 3000, 2700, 0, 500, 300, 0.90, 9000, 3570, 5430, 0),
(datetime('now', '-8 hours'), 1, 'Claude Channel 1', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 2800, 2380, 0, 460, 420, 0.85, 8060, 3478, 4582, 0),
(datetime('now', '-10 hours'), 1, 'Claude Channel 1', 2, 2, 0, 'claude-3-5-sonnet-20241022',
 3100, 2945, 0, 515, 155, 0.95, 9185, 3200.5, 5984.5, 0),
(datetime('now', '-12 hours'), 1, 'Claude Channel 1', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 2950, 2655, 0, 490, 295, 0.90, 8690, 3365.5, 5324.5, 0);


-- 渠道2：中等命中率场景（50-70%）- 模拟通用业务场景
-- 如果有第二个渠道，将channel_id改为实际值；否则保持为1
INSERT INTO prompt_cache_metrics (
    created_at, channel_id, channel_name, user_id, token_id, log_id, model_name,
    prompt_tokens, cache_read_tokens, cache_creation_tokens, completion_tokens,
    uncached_tokens, cache_hit_rate, cost_without_cache, cost_with_cache, cost_saved, is_warmup
) VALUES
-- 最近1小时
(datetime('now', '-1 hours', '+5 minutes'), 2, 'Claude Channel 2', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 3200, 2240, 0, 550, 960, 0.70, 9650, 5734, 3916, 0),
(datetime('now', '-1 hours', '+35 minutes'), 2, 'Claude Channel 2', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 2900, 1740, 0, 480, 1160, 0.60, 8380, 5598, 2782, 0),

-- 2-4小时前
(datetime('now', '-2 hours', '+10 minutes'), 2, 'Claude Channel 2', 2, 2, 0, 'claude-3-5-sonnet-20241022',
 3100, 1860, 0, 520, 1240, 0.60, 9040, 5926, 3114, 0),
(datetime('now', '-3 hours', '+15 minutes'), 2, 'Claude Channel 2', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 2850, 1995, 0, 470, 855, 0.70, 8265, 4929.5, 3335.5, 0),
(datetime('now', '-4 hours', '+20 minutes'), 2, 'Claude Channel 2', 2, 2, 0, 'claude-3-5-sonnet-20241022',
 3000, 1500, 0, 500, 1500, 0.50, 9000, 6000, 3000, 0),

-- 5-12小时前
(datetime('now', '-6 hours', '+10 minutes'), 2, 'Claude Channel 2', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 2950, 2065, 0, 490, 885, 0.70, 8690, 5161.5, 3528.5, 0),
(datetime('now', '-8 hours', '+15 minutes'), 2, 'Claude Channel 2', 2, 2, 0, 'claude-3-5-sonnet-20241022',
 3150, 1890, 0, 525, 1260, 0.60, 9225, 6048, 3177, 0),
(datetime('now', '-10 hours', '+20 minutes'), 2, 'Claude Channel 2', 1, 1, 0, 'claude-3-5-sonnet-20241022',
 2800, 1680, 0, 460, 1120, 0.60, 8060, 5350, 2710, 0),
(datetime('now', '-12 hours', '+25 minutes'), 2, 'Claude Channel 2', 2, 2, 0, 'claude-3-5-sonnet-20241022',
 3050, 2135, 0, 505, 915, 0.70, 9065, 5406.5, 3658.5, 0);


-- 渠道3：低命中率场景（20-40%）- 模拟个性化/动态内容场景
-- 如果有第三个渠道，将channel_id改为实际值；否则保持为1
INSERT INTO prompt_cache_metrics (
    created_at, channel_id, channel_name, user_id, token_id, log_id, model_name,
    prompt_tokens, cache_read_tokens, cache_creation_tokens, completion_tokens,
    uncached_tokens, cache_hit_rate, cost_without_cache, cost_with_cache, cost_saved, is_warmup
) VALUES
-- 最近1小时
(datetime('now', '-1 hours', '+8 minutes'), 3, 'Claude Channel 3', 3, 3, 0, 'claude-3-5-sonnet-20241022',
 3000, 1200, 0, 500, 1800, 0.40, 9000, 6690, 2310, 0),
(datetime('now', '-1 hours', '+40 minutes'), 3, 'Claude Channel 3', 3, 3, 0, 'claude-3-5-sonnet-20241022',
 2800, 840, 0, 460, 1960, 0.30, 8060, 6254, 1806, 0),

-- 2-4小时前
(datetime('now', '-2 hours', '+25 minutes'), 3, 'Claude Channel 3', 3, 3, 0, 'claude-3-5-sonnet-20241022',
 3100, 930, 0, 515, 2170, 0.30, 9185, 7078, 2107, 0),
(datetime('now', '-3 hours', '+30 minutes'), 3, 'Claude Channel 3', 3, 3, 0, 'claude-3-5-sonnet-20241022',
 2900, 1160, 0, 480, 1740, 0.40, 8380, 6294, 2086, 0),
(datetime('now', '-4 hours', '+35 minutes'), 3, 'Claude Channel 3', 3, 3, 0, 'claude-3-5-sonnet-20241022',
 2950, 590, 0, 490, 2360, 0.20, 8690, 7229, 1461, 0),

-- 5-12小时前
(datetime('now', '-6 hours', '+20 minutes'), 3, 'Claude Channel 3', 3, 3, 0, 'claude-3-5-sonnet-20241022',
 3000, 1200, 0, 500, 1800, 0.40, 9000, 6690, 2310, 0),
(datetime('now', '-8 hours', '+25 minutes'), 3, 'Claude Channel 3', 3, 3, 0, 'claude-3-5-sonnet-20241022',
 2850, 855, 0, 470, 1995, 0.30, 8265, 6314.5, 1950.5, 0),
(datetime('now', '-10 hours', '+30 minutes'), 3, 'Claude Channel 3', 3, 3, 0, 'claude-3-5-sonnet-20241022',
 3100, 1240, 0, 515, 1860, 0.40, 9185, 6829, 2356, 0),
(datetime('now', '-12 hours', '+35 minutes'), 3, 'Claude Channel 3', 3, 3, 0, 'claude-3-5-sonnet-20241022',
 2900, 580, 0, 480, 2320, 0.20, 8380, 7022, 1358, 0);


-- 添加几条Warmup标记的记录（系统自动warmup）
INSERT INTO prompt_cache_metrics (
    created_at, channel_id, channel_name, user_id, token_id, log_id, model_name,
    prompt_tokens, cache_read_tokens, cache_creation_tokens, completion_tokens,
    uncached_tokens, cache_hit_rate, cost_without_cache, cost_with_cache, cost_saved, is_warmup
) VALUES
(datetime('now', '-30 minutes'), 1, 'Claude Channel 1', 0, 0, 0, 'claude-3-5-sonnet-20241022',
 3000, 0, 3000, 50, 0, 0.00, 3150, 3900, -750, 1),
(datetime('now', '-1 hours', '-4 minutes'), 1, 'Claude Channel 1', 0, 0, 0, 'claude-3-5-sonnet-20241022',
 3000, 0, 3000, 50, 0, 0.00, 3150, 3900, -750, 1),
(datetime('now', '-2 hours', '-8 minutes'), 1, 'Claude Channel 1', 0, 0, 0, 'claude-3-5-sonnet-20241022',
 3000, 0, 3000, 50, 0, 0.00, 3150, 3900, -750, 1);


-- ============================================================================
-- 数据验证查询
-- ============================================================================
-- 执行插入后，可以运行以下查询验证数据：

-- 1. 检查总记录数
-- SELECT COUNT(*) as total_records FROM prompt_cache_metrics;

-- 2. 检查各渠道记录数
-- SELECT channel_id, channel_name, COUNT(*) as count,
--        AVG(cache_hit_rate) as avg_hit_rate,
--        SUM(cost_saved) as total_saved
-- FROM prompt_cache_metrics
-- WHERE is_warmup = 0
-- GROUP BY channel_id, channel_name;

-- 3. 检查时间分布
-- SELECT DATE(created_at) as date, COUNT(*) as count
-- FROM prompt_cache_metrics
-- GROUP BY DATE(created_at)
-- ORDER BY date DESC;

-- 4. 检查Warmup记录
-- SELECT COUNT(*) as warmup_count FROM prompt_cache_metrics WHERE is_warmup = 1;

-- ============================================================================
-- 注意事项
-- ============================================================================
-- 1. 如果需要更多数据，可以复制上面的INSERT语句并修改时间戳
-- 2. Cost计算公式：
--    - cost_without_cache = (prompt_tokens + completion_tokens * 3.0) * groupRatio(1.0)
--    - cost_with_cache = (uncached_tokens + cache_read_tokens * 0.1 + completion_tokens * 3.0) * groupRatio(1.0)
--    - cost_saved = cost_without_cache - cost_with_cache
-- 3. 本脚本使用groupRatio=1.0和modelRatio=1.0进行计算（简化）
-- 4. 如果数据库中没有channels记录，请先在后台创建至少1个Claude渠道
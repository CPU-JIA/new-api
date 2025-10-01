#!/bin/bash

# ============================================================================
# Cache Analytics API Endpoints Test Script
# 用途：测试5个Cache Analytics API endpoints的功能和权限控制
# 使用前提：
#   1. new-api服务已启动（默认端口3000）
#   2. 已登录并获取session token
#   3. 数据库中有测试数据（执行insert_test_cache_metrics.sql）
# ============================================================================

# 配置变量
BASE_URL="http://localhost:3000"
TOKEN=""  # 需要替换为实际的session token或Authorization token

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ============================================================================
# 辅助函数
# ============================================================================

print_header() {
    echo ""
    echo -e "${BLUE}======================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}======================================${NC}"
}

print_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

# 检查服务是否启动
check_service() {
    print_test "检查服务连接..."
    if curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/status" | grep -q "200\|404"; then
        print_success "服务运行中"
        return 0
    else
        print_error "服务未启动或无法连接 ($BASE_URL)"
        echo "请先启动 new-api 服务: ./new-api.exe"
        exit 1
    fi
}

# 通用API测试函数
test_api() {
    local method=$1
    local endpoint=$2
    local description=$3
    local expected_status=$4

    print_test "$description"
    echo "  URL: $method $BASE_URL$endpoint"

    # 构建curl命令
    local headers=""
    if [ -n "$TOKEN" ]; then
        headers="-H \"Authorization: Bearer $TOKEN\""
    fi

    # 执行请求并保存响应
    local response=$(eval "curl -s -w \"\\n%{http_code}\" -X $method $headers \"$BASE_URL$endpoint\"")

    # 分离响应体和状态码
    local body=$(echo "$response" | sed '$d')
    local status=$(echo "$response" | tail -n1)

    # 检查状态码
    if [ "$status" = "$expected_status" ]; then
        print_success "状态码: $status (预期: $expected_status)"

        # 格式化JSON输出（如果安装了jq）
        if command -v jq &> /dev/null; then
            echo "$body" | jq '.' 2>/dev/null || echo "$body"
        else
            echo "$body"
        fi
    else
        print_error "状态码: $status (预期: $expected_status)"
        echo "$body"
    fi

    echo ""
}

# ============================================================================
# 主测试流程
# ============================================================================

print_header "Cache Analytics API 测试"

# 检查依赖
if ! command -v curl &> /dev/null; then
    print_error "需要安装 curl 命令"
    exit 1
fi

# 检查TOKEN配置
if [ -z "$TOKEN" ]; then
    echo -e "${YELLOW}注意：TOKEN未配置，可能需要手动添加cookie或header${NC}"
    echo "获取TOKEN方法："
    echo "  1. 浏览器登录后台，F12打开DevTools"
    echo "  2. Application -> Cookies -> 复制session token"
    echo "  3. 或Network面板查看Authorization header"
    echo ""
    read -p "是否继续测试？(y/n): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# 检查服务状态
check_service

# ============================================================================
# Test 1: Overview API - 获取缓存概览
# ============================================================================
print_header "Test 1: Cache Overview API"

test_api "GET" "/api/cache/metrics/overview?period=24h" \
    "获取24小时缓存概览" "200"

test_api "GET" "/api/cache/metrics/overview?period=7d" \
    "获取7天缓存概览" "200"

test_api "GET" "/api/cache/metrics/overview?period=invalid" \
    "测试无效period参数" "400"

# ============================================================================
# Test 2: Chart Data API - 获取时间序列数据
# ============================================================================
print_header "Test 2: Cache Chart Data API"

test_api "GET" "/api/cache/metrics/chart?period=24h&interval=1h" \
    "获取24小时图表数据（1小时间隔）" "200"

test_api "GET" "/api/cache/metrics/chart?period=7d&interval=1d" \
    "获取7天图表数据（1天间隔）" "200"

# ============================================================================
# Test 3: Channel Metrics API - 渠道分组数据（管理员）
# ============================================================================
print_header "Test 3: Channel Metrics API (Admin Only)"

test_api "GET" "/api/cache/metrics/channels?period=24h" \
    "获取渠道缓存统计（需要管理员权限）" "200"

echo -e "${YELLOW}注意：如果返回403，说明当前用户不是管理员${NC}"
echo ""

# ============================================================================
# Test 4: User Metrics API - 用户个人数据
# ============================================================================
print_header "Test 4: User-Specific Metrics API"

# 需要替换为实际的user_id
USER_ID=1

test_api "GET" "/api/cache/metrics/user/$USER_ID?period=24h" \
    "获取用户ID=$USER_ID的缓存数据" "200"

echo -e "${YELLOW}注意：普通用户只能查看自己的数据，管理员可查看所有用户${NC}"
echo ""

# ============================================================================
# Test 5: Warmup Status API - CacheWarmer实时状态（管理员）
# ============================================================================
print_header "Test 5: CacheWarmer Status API (Admin Only)"

test_api "GET" "/api/cache/warmer/status" \
    "获取CacheWarmer实时状态（需要管理员权限）" "200"

echo -e "${YELLOW}注意：如果返回403，说明当前用户不是管理员${NC}"
echo ""

# ============================================================================
# 测试总结
# ============================================================================
print_header "测试完成"

echo "后续步骤："
echo "  1. 检查上面的测试结果，确认API返回数据格式正确"
echo "  2. 启动前端dev server：cd web && bun run dev"
echo "  3. 浏览器访问：http://localhost:3000/console/cache"
echo "  4. 验证Dashboard UI显示正确"
echo ""
echo "问题排查："
echo "  - 如果API返回403：检查TOKEN是否有效，或用户权限是否足够"
echo "  - 如果API返回404：检查路由是否正确注册"
echo "  - 如果API返回500：检查后端日志，可能是数据库或数据格式问题"
echo "  - 如果返回空数据：检查数据库中是否有prompt_cache_metrics表数据"
echo ""
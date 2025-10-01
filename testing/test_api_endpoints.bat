@echo off
REM ============================================================================
REM Cache Analytics API Endpoints Test Script (Windows)
REM 用途：测试5个Cache Analytics API endpoints的功能和权限控制
REM 使用前提：
REM   1. new-api服务已启动（默认端口3000）
REM   2. 已登录并获取session token
REM   3. 数据库中有测试数据（执行insert_test_cache_metrics.sql）
REM   4. 系统已安装curl命令（Windows 10 1803+自带）
REM ============================================================================

setlocal enabledelayedexpansion

REM 配置变量
set BASE_URL=http://localhost:3000
set TOKEN=

REM ============================================================================
REM 检查curl是否可用
REM ============================================================================
where curl >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo [错误] curl命令不可用，请确认Windows版本或手动安装curl
    exit /b 1
)

REM ============================================================================
REM 检查服务状态
REM ============================================================================
echo.
echo ======================================
echo 检查服务连接
echo ======================================
curl -s -o nul -w "%%{http_code}" "%BASE_URL%/api/status" | findstr /C:"200" /C:"404" >nul
if %ERRORLEVEL% equ 0 (
    echo [成功] 服务运行中
) else (
    echo [错误] 服务未启动或无法连接 (%BASE_URL%)
    echo 请先启动 new-api 服务: new-api.exe
    exit /b 1
)

REM ============================================================================
REM 检查TOKEN配置
REM ============================================================================
if "%TOKEN%"=="" (
    echo.
    echo [注意] TOKEN未配置，可能需要手动添加cookie或header
    echo 获取TOKEN方法：
    echo   1. 浏览器登录后台，F12打开DevTools
    echo   2. Application -^> Cookies -^> 复制session token
    echo   3. 或Network面板查看Authorization header
    echo.
    set /p CONTINUE="是否继续测试？(Y/N): "
    if /i not "!CONTINUE!"=="Y" exit /b 1
)

REM ============================================================================
REM Test 1: Overview API - 获取缓存概览
REM ============================================================================
echo.
echo ======================================
echo Test 1: Cache Overview API
echo ======================================

echo [TEST] 获取24小时缓存概览
echo URL: GET %BASE_URL%/api/cache/metrics/overview?period=24h
curl -s -w "\nHTTP Status: %%{http_code}\n" "%BASE_URL%/api/cache/metrics/overview?period=24h"
echo.

echo [TEST] 获取7天缓存概览
echo URL: GET %BASE_URL%/api/cache/metrics/overview?period=7d
curl -s -w "\nHTTP Status: %%{http_code}\n" "%BASE_URL%/api/cache/metrics/overview?period=7d"
echo.

echo [TEST] 测试无效period参数（应返回400）
echo URL: GET %BASE_URL%/api/cache/metrics/overview?period=invalid
curl -s -w "\nHTTP Status: %%{http_code}\n" "%BASE_URL%/api/cache/metrics/overview?period=invalid"
echo.

REM ============================================================================
REM Test 2: Chart Data API - 获取时间序列数据
REM ============================================================================
echo.
echo ======================================
echo Test 2: Cache Chart Data API
echo ======================================

echo [TEST] 获取24小时图表数据（1小时间隔）
echo URL: GET %BASE_URL%/api/cache/metrics/chart?period=24h^&interval=1h
curl -s -w "\nHTTP Status: %%{http_code}\n" "%BASE_URL%/api/cache/metrics/chart?period=24h&interval=1h"
echo.

echo [TEST] 获取7天图表数据（1天间隔）
echo URL: GET %BASE_URL%/api/cache/metrics/chart?period=7d^&interval=1d
curl -s -w "\nHTTP Status: %%{http_code}\n" "%BASE_URL%/api/cache/metrics/chart?period=7d&interval=1d"
echo.

REM ============================================================================
REM Test 3: Channel Metrics API - 渠道分组数据（管理员）
REM ============================================================================
echo.
echo ======================================
echo Test 3: Channel Metrics API (Admin)
echo ======================================

echo [TEST] 获取渠道缓存统计（需要管理员权限）
echo URL: GET %BASE_URL%/api/cache/metrics/channels?period=24h
curl -s -w "\nHTTP Status: %%{http_code}\n" "%BASE_URL%/api/cache/metrics/channels?period=24h"
echo.
echo [注意] 如果返回403，说明当前用户不是管理员
echo.

REM ============================================================================
REM Test 4: User Metrics API - 用户个人数据
REM ============================================================================
echo.
echo ======================================
echo Test 4: User-Specific Metrics API
echo ======================================

REM 需要替换为实际的user_id
set USER_ID=1

echo [TEST] 获取用户ID=%USER_ID%的缓存数据
echo URL: GET %BASE_URL%/api/cache/metrics/user/%USER_ID%?period=24h
curl -s -w "\nHTTP Status: %%{http_code}\n" "%BASE_URL%/api/cache/metrics/user/%USER_ID%?period=24h"
echo.
echo [注意] 普通用户只能查看自己的数据，管理员可查看所有用户
echo.

REM ============================================================================
REM Test 5: Warmup Status API - CacheWarmer实时状态（管理员）
REM ============================================================================
echo.
echo ======================================
echo Test 5: CacheWarmer Status API (Admin)
echo ======================================

echo [TEST] 获取CacheWarmer实时状态（需要管理员权限）
echo URL: GET %BASE_URL%/api/cache/warmer/status
curl -s -w "\nHTTP Status: %%{http_code}\n" "%BASE_URL%/api/cache/warmer/status"
echo.
echo [注意] 如果返回403，说明当前用户不是管理员
echo.

REM ============================================================================
REM 测试总结
REM ============================================================================
echo.
echo ======================================
echo 测试完成
echo ======================================
echo.
echo 后续步骤：
echo   1. 检查上面的测试结果，确认API返回数据格式正确
echo   2. 启动前端dev server：cd web ^&^& bun run dev
echo   3. 浏览器访问：http://localhost:3000/console/cache
echo   4. 验证Dashboard UI显示正确
echo.
echo 问题排查：
echo   - 如果API返回403：检查TOKEN是否有效，或用户权限是否足够
echo   - 如果API返回404：检查路由是否正确注册
echo   - 如果API返回500：检查后端日志，可能是数据库或数据格式问题
echo   - 如果返回空数据：检查数据库中是否有prompt_cache_metrics表数据
echo.

pause
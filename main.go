package main

import (
	"bytes"
	"embed"
	"fmt"
	"log"
	"net/http"
	"one-api/common"
	"one-api/constant"
	"one-api/controller"
	"one-api/logger"
	"one-api/middleware"
	"one-api/model"
	"one-api/router"
	"one-api/service"
	"one-api/setting/ratio_setting"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	_ "net/http/pprof"
)

//go:embed web/dist
var buildFS embed.FS

//go:embed web/dist/index.html
var indexPage []byte

func main() {
	startTime := time.Now()

	err := InitResources()
	if err != nil {
		common.FatalLog("failed to initialize resources: " + err.Error())
		return
	}

	common.SysLog("New API " + common.Version + " started")
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	if common.DebugEnabled {
		common.SysLog("running in debug mode")
	}

	defer func() {
		err := model.CloseDB()
		if err != nil {
			common.FatalLog("failed to close database: " + err.Error())
		}
	}()

	if common.RedisEnabled {
		// for compatibility with old versions
		common.MemoryCacheEnabled = true
	}
	if common.MemoryCacheEnabled {
		common.SysLog("memory cache enabled")
		common.SysLog(fmt.Sprintf("sync frequency: %d seconds", common.SyncFrequency))

		// Add panic recovery and retry for InitChannelCache
		func() {
			defer func() {
				if r := recover(); r != nil {
					common.SysLog(fmt.Sprintf("InitChannelCache panic: %v, retrying once", r))
					// Retry once
					_, _, fixErr := model.FixAbility()
					if fixErr != nil {
						common.FatalLog(fmt.Sprintf("InitChannelCache failed: %s", fixErr.Error()))
					}
				}
			}()
			model.InitChannelCache()
		}()

		go model.SyncChannelCache(common.SyncFrequency)
	}

	// 热更新配置
	go model.SyncOptions(common.SyncFrequency)

	// 数据看板
	go model.UpdateQuotaData()

	if os.Getenv("CHANNEL_UPDATE_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_UPDATE_FREQUENCY"))
		if err != nil {
			common.FatalLog("failed to parse CHANNEL_UPDATE_FREQUENCY: " + err.Error())
		}
		go controller.AutomaticallyUpdateChannels(frequency)
	}

	go controller.AutomaticallyTestChannels()

	// Start Cache Warmer Service for pool cache optimization
	service.GetCacheWarmerService().Start()
	common.SysLog("Cache Warmer service started for intelligent pool cache keep-alive")

	if common.IsMasterNode && constant.UpdateTask {
		gopool.Go(func() {
			controller.UpdateMidjourneyTaskBulk()
		})
		gopool.Go(func() {
			controller.UpdateTaskBulk()
		})
	}
	if os.Getenv("BATCH_UPDATE_ENABLED") == "true" {
		common.BatchUpdateEnabled = true
		common.SysLog("batch update enabled with interval " + strconv.Itoa(common.BatchUpdateInterval) + "s")
		model.InitBatchUpdater()
	}

	if os.Getenv("ENABLE_PPROF") == "true" {
		gopool.Go(func() {
			log.Println(http.ListenAndServe("0.0.0.0:8005", nil))
		})
		go common.Monitor()
		common.SysLog("pprof enabled")
	}

	// Initialize HTTP server
	server := gin.New()
	server.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		// Log detailed panic information for debugging
		common.SysLog(fmt.Sprintf("panic detected: %v", err))
		common.SysLog(fmt.Sprintf("Please submit an issue: https://github.com/Calcium-Ion/new-api"))

		// Return generic error message to client (security best practice)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "An internal server error occurred. Please try again later or contact support.",
				"type":    "server_error",
			},
		})
	}))
	// This will cause SSE not to work!!!
	//server.Use(gzip.Gzip(gzip.DefaultCompression))
	server.Use(middleware.RequestId())
	middleware.SetUpLogger(server)
	// Initialize session store
	store := cookie.NewStore([]byte(common.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   2592000, // 30 days
		HttpOnly: true,
		Secure:   os.Getenv("HTTPS_ENABLED") == "true", // Only enable Secure flag when explicitly using HTTPS
		SameSite: http.SameSiteLaxMode,                 // Lax mode: allows same-site navigation while protecting against CSRF
	})
	server.Use(sessions.Sessions("session", store))

	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("UMAMI_WEBSITE_ID") != "" {
		umamiSiteID := os.Getenv("UMAMI_WEBSITE_ID")
		umamiScriptURL := os.Getenv("UMAMI_SCRIPT_URL")
		if umamiScriptURL == "" {
			umamiScriptURL = "https://analytics.umami.is/script.js"
		}
		analyticsInjectBuilder.WriteString("<script defer src=\"")
		analyticsInjectBuilder.WriteString(umamiScriptURL)
		analyticsInjectBuilder.WriteString("\" data-website-id=\"")
		analyticsInjectBuilder.WriteString(umamiSiteID)
		analyticsInjectBuilder.WriteString("\"></script>")
	}
	analyticsInject := analyticsInjectBuilder.String()
	indexPage = bytes.ReplaceAll(indexPage, []byte("<analytics></analytics>\n"), []byte(analyticsInject))

	router.SetRouter(server, buildFS, indexPage)
	var port = os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}

	// Log startup success message
	common.LogStartupSuccess(startTime, port)

	err = server.Run(":" + port)
	if err != nil {
		common.FatalLog("failed to start HTTP server: " + err.Error())
	}
}

// validateCriticalEnvVars checks critical environment variables to prevent runtime errors
func validateCriticalEnvVars() {
	// Check database configuration
	sqlDSN := os.Getenv("SQL_DSN")
	sqlitePath := os.Getenv("SQLITE_PATH")
	if sqlDSN == "" && sqlitePath == "" {
		common.SysLog("⚠️  Warning: No database configuration found (SQL_DSN or SQLITE_PATH)")
		common.SysLog("⚠️  警告：未找到数据库配置（SQL_DSN 或 SQLITE_PATH）")
		common.SysLog("    Using default SQLite path: ./data/")
	}

	// Check SESSION_SECRET for multi-node deployment
	if os.Getenv("NODE_TYPE") != "" || os.Getenv("REDIS_CONN_STRING") != "" {
		if os.Getenv("SESSION_SECRET") == "" {
			common.SysLog("⚠️  Warning: SESSION_SECRET not set for multi-node/Redis deployment")
			common.SysLog("⚠️  警告：多节点或Redis部署未设置SESSION_SECRET")
			common.SysLog("    This will cause session inconsistency across nodes")
		}
	}

	// Check CRYPTO_SECRET if using Redis
	if os.Getenv("REDIS_CONN_STRING") != "" {
		if os.Getenv("CRYPTO_SECRET") == "" {
			common.SysLog("⚠️  Warning: CRYPTO_SECRET not set while using Redis")
			common.SysLog("⚠️  警告：使用Redis时未设置CRYPTO_SECRET")
			common.SysLog("    This may cause encryption/decryption issues")
		}
	}

	// Warn if DEBUG is enabled (potential production security issue)
	if os.Getenv("DEBUG") == "true" {
		common.SysLog("⚠️  Warning: DEBUG mode is ENABLED")
		common.SysLog("⚠️  警告：调试模式已启用")
		common.SysLog("    - Session Secure flag is DISABLED (allows HTTP)")
		common.SysLog("    - This should NOT be used in production environment!")
		common.SysLog("    - 生产环境不应启用DEBUG模式！")
	}

	// Check FRONTEND_BASE_URL for multi-node deployment
	if os.Getenv("NODE_TYPE") != "" && os.Getenv("FRONTEND_BASE_URL") == "" {
		common.SysLog("⚠️  Warning: FRONTEND_BASE_URL not set for multi-node deployment")
		common.SysLog("⚠️  警告：多节点部署未设置FRONTEND_BASE_URL")
	}
}

func InitResources() error {
	// Initialize resources here if needed
	// This is a placeholder function for future resource initialization
	err := godotenv.Load(".env")
	if err != nil {
		common.SysLog("未找到 .env 文件，使用默认环境变量，如果需要，请创建 .env 文件并设置相关变量")
		common.SysLog("No .env file found, using default environment variables. If needed, please create a .env file and set the relevant variables.")
	}

	// 加载环境变量
	common.InitEnv()

	// Validate critical environment variables
	validateCriticalEnvVars()

	logger.SetupLogger()

	// Initialize model settings
	ratio_setting.InitRatioSettings()

	service.InitHttpClient()

	service.InitTokenEncoders()

	// Initialize SQL Database
	err = model.InitDB()
	if err != nil {
		common.FatalLog("failed to initialize database: " + err.Error())
		return err
	}

	model.CheckSetup()

	// Initialize options, should after model.InitDB()
	model.InitOptionMap()

	// 初始化模型
	model.GetPricing()

	// Initialize SQL Database
	err = model.InitLogDB()
	if err != nil {
		return err
	}

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		return err
	}
	return nil
}

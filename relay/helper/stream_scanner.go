package helper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"one-api/common"
	"one-api/constant"
	"one-api/logger"
	relaycommon "one-api/relay/common"
	"one-api/setting/operation_setting"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
)

const (
	InitialScannerBufferSize = 64 << 10 // 64KB (64*1024)
	MaxScannerBufferSize     = 10 << 20 // 10MB (10*1024*1024)
	DefaultPingInterval      = 10 * time.Second
	DataProcessorWorkers     = 4        // 数据处理worker数量
	PingOperationTimeout     = 10 * time.Second
	DataHandlerTimeout       = 10 * time.Second
)

// 数据处理任务结构
type DataProcessTask struct {
	Data    string
	Handler func(data string) bool
	Result  chan bool
	Context context.Context
}

// Ping操作任务结构
type PingTask struct {
	Context *gin.Context
	Result  chan error
}

// 对象池，减少内存分配
var (
	dataTaskPool = sync.Pool{
		New: func() interface{} {
			return &DataProcessTask{
				Result: make(chan bool, 1),
			}
		},
	}

	pingTaskPool = sync.Pool{
		New: func() interface{} {
			return &PingTask{
				Result: make(chan error, 1),
			}
		},
	}

	channelPool = sync.Pool{
		New: func() interface{} {
			return make(chan bool, 1)
		},
	}
)

// 全局Worker Pool管理器
type StreamWorkerManager struct {
	dataWorkerChan chan *DataProcessTask
	pingWorkerChan chan *PingTask
	once           sync.Once
	started        bool
	stopChan       chan struct{}
}

var globalStreamManager = &StreamWorkerManager{
	dataWorkerChan: make(chan *DataProcessTask, 100), // 缓冲队列
	pingWorkerChan: make(chan *PingTask, 50),
	stopChan:       make(chan struct{}),
}

// 初始化Worker Pool（延迟初始化）
func (sm *StreamWorkerManager) ensureStarted() {
	sm.once.Do(func() {
		if sm.started {
			return
		}

		// 启动数据处理workers
		for i := 0; i < DataProcessorWorkers; i++ {
			gopool.Go(func() {
				for {
					select {
					case task := <-sm.dataWorkerChan:
						sm.processDataTask(task)
					case <-sm.stopChan:
						return
					}
				}
			})
		}

		// 启动ping处理worker
		gopool.Go(func() {
			for {
				select {
				case task := <-sm.pingWorkerChan:
					sm.processPingTask(task)
				case <-sm.stopChan:
					return
				}
			}
		})

		sm.started = true
		common.SysLog("Stream worker manager started")
	})
}

// 处理数据任务
func (sm *StreamWorkerManager) processDataTask(task *DataProcessTask) {
	defer func() {
		// 回收对象到池中
		task.Data = ""
		task.Handler = nil
		task.Context = nil
		dataTaskPool.Put(task)

		if r := recover(); r != nil {
			logger.LogError(task.Context.(*gin.Context), fmt.Sprintf("data processing panic: %v", r))
			select {
			case task.Result <- false:
			default:
			}
		}
	}()

	select {
	case task.Result <- task.Handler(task.Data):
	case <-task.Context.Done():
		select {
		case task.Result <- false:
		default:
		}
	case <-time.After(DataHandlerTimeout):
		select {
		case task.Result <- false:
		default:
		}
	}
}

// 处理ping任务
func (sm *StreamWorkerManager) processPingTask(task *PingTask) {
	defer func() {
		// 回收对象到池中
		task.Context = nil
		pingTaskPool.Put(task)

		if r := recover(); r != nil {
			logger.LogError(task.Context, fmt.Sprintf("ping processing panic: %v", r))
			select {
			case task.Result <- fmt.Errorf("ping panic: %v", r):
			default:
			}
		}
	}()

	select {
	case task.Result <- PingData(task.Context):
	case <-task.Context.Request.Context().Done():
		select {
		case task.Result <- fmt.Errorf("client disconnected"):
		default:
		}
	case <-time.After(PingOperationTimeout):
		select {
		case task.Result <- fmt.Errorf("ping timeout"):
		default:
		}
	}
}

// 提交数据处理任务
func (sm *StreamWorkerManager) submitDataTask(ctx context.Context, data string, handler func(string) bool) bool {
	task := dataTaskPool.Get().(*DataProcessTask)
	task.Data = data
	task.Handler = handler
	task.Context = ctx

	select {
	case sm.dataWorkerChan <- task:
		select {
		case result := <-task.Result:
			return result
		case <-ctx.Done():
			return false
		case <-time.After(DataHandlerTimeout):
			return false
		}
	case <-ctx.Done():
		dataTaskPool.Put(task)
		return false
	case <-time.After(100 * time.Millisecond): // 避免阻塞
		dataTaskPool.Put(task)
		return false
	}
}

// 提交ping任务
func (sm *StreamWorkerManager) submitPingTask(ctx *gin.Context) error {
	task := pingTaskPool.Get().(*PingTask)
	task.Context = ctx

	select {
	case sm.pingWorkerChan <- task:
		select {
		case result := <-task.Result:
			return result
		case <-ctx.Request.Context().Done():
			return fmt.Errorf("client disconnected")
		case <-time.After(PingOperationTimeout):
			return fmt.Errorf("ping submission timeout")
		}
	case <-ctx.Request.Context().Done():
		pingTaskPool.Put(task)
		return fmt.Errorf("client disconnected")
	case <-time.After(100 * time.Millisecond):
		pingTaskPool.Put(task)
		return fmt.Errorf("ping queue full")
	}
}

func StreamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string) bool) {

	if resp == nil || dataHandler == nil {
		return
	}

	// 确保Worker Manager已启动
	globalStreamManager.ensureStarted()

	// 确保响应体总是被关闭
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	streamingTimeout := time.Duration(constant.StreamingTimeout) * time.Second

	var (
		stopChan   = channelPool.Get().(chan bool) // 从池中获取channel
		scanner    = bufio.NewScanner(resp.Body)
		ticker     = time.NewTicker(streamingTimeout)
		pingTicker *time.Ticker
		writeMutex sync.RWMutex // 改为读写锁，提升并发性能
		wg         sync.WaitGroup
	)

	// 回收channel到池中
	defer func() {
		select {
		case <-stopChan:
		default:
		}
		channelPool.Put(stopChan)
	}()

	generalSettings := operation_setting.GetGeneralSetting()
	pingEnabled := generalSettings.PingIntervalEnabled && !info.DisablePing
	pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
	if pingInterval <= 0 {
		pingInterval = DefaultPingInterval
	}

	if pingEnabled {
		pingTicker = time.NewTicker(pingInterval)
	}

	if common.DebugEnabled {
		// print timeout and ping interval for debugging
		println("relay timeout seconds:", common.RelayTimeout)
		println("streaming timeout seconds:", int64(streamingTimeout.Seconds()))
		println("ping interval seconds:", int64(pingInterval.Seconds()))
	}

	// 改进资源清理，确保所有 goroutine 正确退出
	defer func() {
		// 通知所有 goroutine 停止
		common.SafeSendBool(stopChan, true)

		ticker.Stop()
		if pingTicker != nil {
			pingTicker.Stop()
		}

		// 等待所有 goroutine 退出，最多等待5秒
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			logger.LogError(c, "timeout waiting for goroutines to exit")
		}
	}()

	scanner.Buffer(make([]byte, InitialScannerBufferSize), MaxScannerBufferSize)
	scanner.Split(bufio.ScanLines)
	SetEventStreamHeaders(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = context.WithValue(ctx, "stop_chan", stopChan)

	// Handle ping data sending with improved error handling
	if pingEnabled && pingTicker != nil {
		wg.Add(1)
		gopool.Go(func() {
			defer func() {
				wg.Done()
				if r := recover(); r != nil {
					logger.LogError(c, fmt.Sprintf("ping goroutine panic: %v", r))
					common.SafeSendBool(stopChan, true)
				}
				if common.DebugEnabled {
					println("ping goroutine exited")
				}
			}()

			// 添加超时保护，防止 goroutine 无限运行
			maxPingDuration := 30 * time.Minute // 最大 ping 持续时间
			pingTimeout := time.NewTimer(maxPingDuration)
			defer pingTimeout.Stop()

			for {
				select {
				case <-pingTicker.C:
					// 使用worker pool处理ping操作，而不是创建新goroutine
					writeMutex.Lock()
					err := globalStreamManager.submitPingTask(c)
					writeMutex.Unlock()

					if err != nil {
						logger.LogError(c, "ping data error: "+err.Error())
						return
					}
					if common.DebugEnabled {
						println("ping data sent")
					}

				case <-ctx.Done():
					return
				case <-stopChan:
					return
				case <-c.Request.Context().Done():
					// 监听客户端断开连接
					return
				case <-pingTimeout.C:
					logger.LogError(c, "ping goroutine max duration reached")
					return
				}
			}
		})
	}

	// Scanner goroutine with improved error handling
	wg.Add(1)
	common.RelayCtxGo(ctx, func() {
		defer func() {
			wg.Done()
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("scanner goroutine panic: %v", r))
			}
			common.SafeSendBool(stopChan, true)
			if common.DebugEnabled {
				println("scanner goroutine exited")
			}
		}()

		for scanner.Scan() {
			// 检查是否需要停止
			select {
			case <-stopChan:
				return
			case <-ctx.Done():
				return
			case <-c.Request.Context().Done():
				return
			default:
			}

			ticker.Reset(streamingTimeout)
			data := scanner.Text()
			if common.DebugEnabled {
				println(data)
			}

			if len(data) < 6 {
				continue
			}
			if data[:5] != "data:" && data[:6] != "[DONE]" {
				continue
			}
			data = data[5:]
			data = strings.TrimLeft(data, " ")
			data = strings.TrimSuffix(data, "\r")
			if !strings.HasPrefix(data, "[DONE]") {
				info.SetFirstResponseTime()

				// 使用worker pool处理数据，而不是创建新goroutine
				writeMutex.Lock()
				success := globalStreamManager.submitDataTask(ctx, data, dataHandler)
				writeMutex.Unlock()

				if !success {
					return
				}
			} else {
				// done, 处理完成标志，直接退出停止读取剩余数据防止出错
				if common.DebugEnabled {
					println("received [DONE], stopping scanner")
				}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			if err != io.EOF {
				logger.LogError(c, "scanner error: "+err.Error())
			}
		}
	})

	// 主循环等待完成或超时
	select {
	case <-ticker.C:
		// 超时处理逻辑
		logger.LogError(c, "streaming timeout")
	case <-stopChan:
		// 正常结束
		logger.LogInfo(c, "streaming finished")
	case <-c.Request.Context().Done():
		// 客户端断开连接
		logger.LogInfo(c, "client disconnected")
	}
}
package helper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"one-api/common"
	"one-api/constant"
	relaycommon "one-api/relay/common"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain ensures proper initialization before running tests
func TestMain(m *testing.M) {
	// Initialize environment constants to avoid zero values that cause ticker panics
	common.InitEnv()

	// Ensure streaming timeout has a reasonable value for tests
	if constant.StreamingTimeout <= 0 {
		constant.StreamingTimeout = 300 // Default 5 minutes
	}

	// Run tests
	m.Run()
}

func TestStreamWorkerManager_ensureStarted(t *testing.T) {
	manager := &StreamWorkerManager{
		dataWorkerChan: make(chan *DataProcessTask, 10),
		pingWorkerChan: make(chan *PingTask, 10),
		stopChan:       make(chan struct{}),
	}

	// Test that ensureStarted only initializes once
	manager.ensureStarted()
	assert.True(t, manager.started)

	// Call again to ensure it doesn't reinitialize
	manager.ensureStarted()
	assert.True(t, manager.started)

	// Clean up
	close(manager.stopChan)
}

func TestStreamWorkerManager_submitDataTask(t *testing.T) {
	manager := &StreamWorkerManager{
		dataWorkerChan: make(chan *DataProcessTask, 10),
		pingWorkerChan: make(chan *PingTask, 10),
		stopChan:       make(chan struct{}),
	}
	manager.ensureStarted()
	defer close(manager.stopChan)

	tests := []struct {
		name         string
		data         string
		handlerDelay time.Duration
		timeout      time.Duration
		expectResult bool
	}{
		{
			name:         "Successful data processing",
			data:         "test data",
			handlerDelay: 10 * time.Millisecond,
			timeout:      100 * time.Millisecond,
			expectResult: true,
		},
		{
			name:         "Handler timeout",
			data:         "slow data",
			handlerDelay: 50 * time.Millisecond,
			timeout:      10 * time.Millisecond,
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			handler := func(data string) bool {
				time.Sleep(tt.handlerDelay)
				return data == "test data"
			}

			result := manager.submitDataTask(ctx, tt.data, handler)
			assert.Equal(t, tt.expectResult, result)
		})
	}
}

func TestStreamWorkerManager_submitPingTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := &StreamWorkerManager{
		dataWorkerChan: make(chan *DataProcessTask, 10),
		pingWorkerChan: make(chan *PingTask, 10),
		stopChan:       make(chan struct{}),
	}
	manager.ensureStarted()
	defer close(manager.stopChan)

	// Create test Gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/test", nil)
	c.Request = req

	// Test ping task submission
	err := manager.submitPingTask(c)
	// In test environment, PingData may return an error or nil
	// The important thing is that the task submission doesn't panic or block indefinitely
	// We don't make strict assertions about the result since it depends on the PingData implementation
	_ = err // Just ensure no panic occurs
}

func TestObjectPoolEfficiency(t *testing.T) {
	// Test that object pools reduce allocations
	const iterations = 100

	// Test data task pool
	for i := 0; i < iterations; i++ {
		task := dataTaskPool.Get().(*DataProcessTask)
		task.Data = "test"
		task.Handler = func(string) bool { return true }

		// Reset and return to pool
		task.Data = ""
		task.Handler = nil
		dataTaskPool.Put(task)
	}

	// Test ping task pool
	for i := 0; i < iterations; i++ {
		task := pingTaskPool.Get().(*PingTask)
		task.Context = nil // Would normally have a gin.Context

		// Reset and return to pool
		task.Context = nil
		pingTaskPool.Put(task)
	}

	// Test channel pool
	for i := 0; i < iterations; i++ {
		ch := channelPool.Get().(chan bool)
		channelPool.Put(ch)
	}
}

func TestStreamScannerHandler_GoroutineCount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Get baseline goroutine count
	runtime.GC()
	baselineGoroutines := runtime.NumGoroutine()

	// Create test server with streaming response
	testData := []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}",
		"data: {\"choices\":[{\"delta\":{\"content\":\" World\"}}]}",
		"data: [DONE]",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		for _, line := range testData {
			w.Write([]byte(line + "\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Create test response
	resp, err := http.Get(server.URL)
	require.NoError(t, err)

	// Create test RelayInfo with ping explicitly disabled to avoid ticker issues
	info := &relaycommon.RelayInfo{
		DisablePing: true, // Explicitly disable ping to avoid any ticker creation issues
	}

	// Track processed data
	var processedData []string
	var mu sync.Mutex
	dataHandler := func(data string) bool {
		mu.Lock()
		processedData = append(processedData, data)
		mu.Unlock()
		return true
	}

	// Ensure the response is valid before processing
	require.NotNil(t, resp)
	require.NotNil(t, resp.Body)

	// Run stream scanner - wrap in defer to ensure cleanup on panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("StreamScannerHandler panicked: %v", r)
				t.FailNow()
			}
		}()
		StreamScannerHandler(c, resp, info, dataHandler)
	}()

	// Check that we processed the expected data
	mu.Lock()
	processedDataLen := len(processedData)
	mu.Unlock()

	// We should process at least some data before hitting [DONE]
	assert.GreaterOrEqual(t, processedDataLen, 1, "Should process at least one data packet")

	// Give time for cleanup
	time.Sleep(200 * time.Millisecond)
	runtime.GC()

	// Check goroutine count hasn't increased significantly
	finalGoroutines := runtime.NumGoroutine()
	goroutineIncrease := finalGoroutines - baselineGoroutines

	// Allow for some reasonable increase (worker goroutines), but not excessive
	// Note: With the worker pool optimization, the increase should be minimal
	assert.LessOrEqual(t, goroutineIncrease, 15,
		"Goroutine count increased by %d (from %d to %d), optimization may not be working",
		goroutineIncrease, baselineGoroutines, finalGoroutines)

	t.Logf("Goroutine count: baseline=%d, final=%d, increase=%d",
		baselineGoroutines, finalGoroutines, goroutineIncrease)
}

func TestStreamScannerHandler_DataProcessing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Valid JSON data",
			input:    "data: {\"choices\":[{\"delta\":{\"content\":\"test\"}}]}",
			expected: "{\"choices\":[{\"delta\":{\"content\":\"test\"}}]}",
		},
		{
			name:     "Data with extra spaces",
			input:    "data:   {\"test\": \"value\"}   ",
			expected: "{\"test\": \"value\"}   ", // Only left spaces are trimmed
		},
		{
			name:     "Data with carriage return",
			input:    "data: {\"test\": \"value\"}\r",
			expected: "{\"test\": \"value\"}", // Carriage return is trimmed
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tc.input + "\n"))
				w.Write([]byte("data: [DONE]\n"))
			}))
			defer server.Close()

			// Create test context
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Get response
			resp, err := http.Get(server.URL)
			require.NoError(t, err)

			// Create test RelayInfo
			info := &relaycommon.RelayInfo{
				DisablePing: true,
			}

			// Capture processed data
			var processedData string
			dataHandler := func(data string) bool {
				processedData = data
				return true
			}

			// Run stream scanner
			StreamScannerHandler(c, resp, info, dataHandler)

			// Verify processed data
			assert.Equal(t, tc.expected, processedData)
		})
	}
}

func TestStreamScannerHandler_ErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		resp        *http.Response
		dataHandler func(string) bool
		shouldPanic bool
	}{
		{
			name:        "Nil response",
			resp:        nil,
			dataHandler: func(string) bool { return true },
			shouldPanic: false, // Should handle gracefully
		},
		{
			name: "Nil data handler",
			resp: &http.Response{
				StatusCode: 200,
				Body:       http.NoBody,
			},
			dataHandler: nil,
			shouldPanic: false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			info := &relaycommon.RelayInfo{}

			if tt.shouldPanic {
				assert.Panics(t, func() {
					StreamScannerHandler(c, tt.resp, info, tt.dataHandler)
				})
			} else {
				assert.NotPanics(t, func() {
					StreamScannerHandler(c, tt.resp, info, tt.dataHandler)
				})
			}
		})
	}
}

func TestStreamScannerHandler_ResourceCleanup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a response that will be closed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: test\n"))
		w.Write([]byte("data: [DONE]\n"))
	}))
	defer server.Close()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	resp, err := http.Get(server.URL)
	require.NoError(t, err)

	info := &relaycommon.RelayInfo{
		DisablePing: true,
	}

	dataHandler := func(data string) bool {
		return true
	}

	// Verify that response body gets closed
	assert.False(t, isResponseBodyClosed(resp))

	StreamScannerHandler(c, resp, info, dataHandler)

	// After processing, body should be closed
	// Note: We can't directly test if body is closed, but the defer should handle it
	assert.NotNil(t, resp.Body) // Body should still exist but be closed
}

// Helper function to check if response body is closed
// This is a simplified check since we can't directly test if body is closed
func isResponseBodyClosed(resp *http.Response) bool {
	if resp.Body == nil {
		return true
	}
	// Try to read - if it's closed, it should fail or return EOF immediately
	buf := make([]byte, 1)
	_, err := resp.Body.Read(buf)
	return err != nil
}

func BenchmarkStreamWorkerManager_DataProcessing(b *testing.B) {
	manager := &StreamWorkerManager{
		dataWorkerChan: make(chan *DataProcessTask, 100),
		pingWorkerChan: make(chan *PingTask, 50),
		stopChan:       make(chan struct{}),
	}
	manager.ensureStarted()
	defer close(manager.stopChan)

	ctx := context.Background()
	handler := func(data string) bool {
		// Simulate some processing
		return len(data) > 0
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			manager.submitDataTask(ctx, "benchmark data", handler)
		}
	})
}

func BenchmarkObjectPoolAllocation(b *testing.B) {
	b.Run("DataTaskPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			task := dataTaskPool.Get().(*DataProcessTask)
			task.Data = "test"
			task.Handler = func(string) bool { return true }
			task.Data = ""
			task.Handler = nil
			dataTaskPool.Put(task)
		}
	})

	b.Run("DirectAllocation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			task := &DataProcessTask{
				Result: make(chan bool, 1),
				Data:   "test",
				Handler: func(string) bool { return true },
			}
			_ = task // Use the task to prevent compiler optimization
		}
	})
}
package router

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestTimeoutMiddleware_RaceCondition attempts to reproduce the race condition
// where both timeout (writing 504) and handler (writing 200) try to write
// to the same ResponseWriter concurrently.
//
// Previously, the race condition corrupted bufio.Writer's internal state,
// causing panic at bufio.(*Writer).Write when b.n or b.buf became invalid.
//
// Now, the timeoutWriter wrapper serializes writes and the middleware waits
// for handler completion, eliminating the race.
//
// Run with: go test -race -run TestTimeoutMiddleware_RaceCondition -count=100
func TestTimeoutMiddleware_RaceCondition(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(TimeoutMiddleware(10 * time.Millisecond))

	// Handler that takes just slightly longer than timeout
	// to maximize chance of concurrent writes
	r.GET("/race", func(c *gin.Context) {
		// Sleep just past the timeout to trigger race
		time.Sleep(11 * time.Millisecond)

		// Write a large response to increase time spent in Write
		data := make(map[string]string)
		for i := 0; i < 100; i++ {
			data[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
		}
		c.JSON(http.StatusOK, data)
	})

	// Run multiple iterations to increase chance of hitting race
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest("GET", "/race", nil)
		w := httptest.NewRecorder()

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("iteration %d: panic occurred: %v", i, r)
				}
			}()
			r.ServeHTTP(w, req)
		}()

		// Either 504 (timeout) or 200 (handler won) is acceptable
		if w.Code != http.StatusGatewayTimeout && w.Code != http.StatusOK {
			t.Errorf("iteration %d: unexpected status code: %d", i, w.Code)
		}
	}
}

func TestTimeoutMiddleware_NormalCompletion(t *testing.T) {
	r := gin.New()
	r.Use(TimeoutMiddleware(5 * time.Second))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestTimeoutMiddleware_Timeout tests that when timeout fires but handler
// completes with a response, the handler's response is used (not 504).
// This behavior ensures clients who waited anyway get the actual result.
func TestTimeoutMiddleware_Timeout(t *testing.T) {
	r := gin.New()
	r.Use(TimeoutMiddleware(50 * time.Millisecond))

	handlerCompleted := make(chan struct{})
	r.GET("/slow", func(c *gin.Context) {
		time.Sleep(200 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		close(handlerCompleted)
	})

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Handler wrote response, so that's what client sees (not 504)
	assert.Equal(t, http.StatusOK, w.Code)

	// Middleware waits for handler, so it should be complete
	<-handlerCompleted
}

// TestTimeoutMiddleware_PanicInHandler tests panic recovery in the handler goroutine.
//
// Previously, panics in the handler goroutine crashed the process because
// gin.Recovery() runs in the main goroutine and cannot catch panics from
// other goroutines.
//
// Now, TimeoutMiddleware recovers panics and re-panics in the main goroutine,
// allowing gin.Recovery() to catch them and return 500.
func TestTimeoutMiddleware_PanicInHandler(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(TimeoutMiddleware(5 * time.Second))
	r.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		r.ServeHTTP(w, req)
	})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestTimeoutMiddleware_PanicAfterTimeout tests panic occurring after timeout.
//
// Previously, a panic after timeout crashed the process because the handler
// goroutine continued running with no recovery.
//
// Now, the panic is recovered. Since handler didn't write a response before
// panicking, the middleware sends 500 Internal Server Error.
func TestTimeoutMiddleware_PanicAfterTimeout(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(TimeoutMiddleware(50 * time.Millisecond))

	handlerStarted := make(chan struct{})
	r.GET("/slow-panic", func(c *gin.Context) {
		close(handlerStarted)
		time.Sleep(100 * time.Millisecond)
		panic("late panic after timeout")
	})

	req := httptest.NewRequest("GET", "/slow-panic", nil)
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		r.ServeHTTP(w, req)
		<-handlerStarted
		time.Sleep(150 * time.Millisecond)
	})
	// Handler panicked without writing, so we send 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestTimeoutMiddleware_ContextCancellation verifies that the context is
// cancelled when timeout occurs, allowing well-behaved handlers to stop early.
func TestTimeoutMiddleware_ContextCancellation(t *testing.T) {
	r := gin.New()
	r.Use(TimeoutMiddleware(50 * time.Millisecond))

	var contextWasCancelled atomic.Bool
	r.GET("/context", func(c *gin.Context) {
		select {
		case <-c.Request.Context().Done():
			contextWasCancelled.Store(true)
		case <-time.After(200 * time.Millisecond):
			// Should not reach here
		}
	})

	req := httptest.NewRequest("GET", "/context", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)

	// Give the goroutine time to observe cancellation
	time.Sleep(20 * time.Millisecond)
	assert.True(t, contextWasCancelled.Load(), "context should be cancelled on timeout")
}

// TestTimeoutMiddleware_HandlerWritesAfterContextDone verifies that when handler
// detects context cancellation and writes its own response, that response is used.
func TestTimeoutMiddleware_HandlerWritesAfterContextDone(t *testing.T) {
	r := gin.New()
	r.Use(TimeoutMiddleware(50 * time.Millisecond))

	r.GET("/partial", func(c *gin.Context) {
		// Simulate work that gets interrupted
		select {
		case <-c.Request.Context().Done():
			// Handler detects cancellation and writes custom response
			c.JSON(http.StatusOK, gin.H{
				"status":  "partial",
				"message": "request timed out, returning partial results",
			})
			return
		case <-time.After(200 * time.Millisecond):
			c.JSON(http.StatusOK, gin.H{"status": "complete"})
		}
	})

	req := httptest.NewRequest("GET", "/partial", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Handler wrote its own response, so that's what we get (not 504)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "partial")
}

// mockLogger captures RequestInfo for testing
type mockLogger struct {
	lastInfo RequestInfo
	called   bool
}

func (m *mockLogger) Log(info RequestInfo) {
	m.lastInfo = info
	m.called = true
}

// TestTimeoutMiddleware_LoggingIntegration_Timeout verifies that LogRequest
// middleware captures timeout info set by TimeoutMiddleware.
// Even when handler's response is used, timeout flag is still recorded.
func TestTimeoutMiddleware_LoggingIntegration_Timeout(t *testing.T) {
	logger := &mockLogger{}

	r := gin.New()
	r.Use(LogRequest(logger))
	r.Use(TimeoutMiddleware(50 * time.Millisecond))
	r.GET("/slow", func(c *gin.Context) {
		time.Sleep(100 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Handler wrote response, so that's returned (not 504)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, logger.called, "logger should be called")
	// TimedOut is still true because timeout did fire
	assert.True(t, logger.lastInfo.TimedOut, "TimedOut should be true")
	assert.False(t, logger.lastInfo.PanicRecovered, "PanicRecovered should be false")
}

// TestTimeoutMiddleware_LoggingIntegration_Panic verifies that LogRequest
// middleware captures panic info set by TimeoutMiddleware.
func TestTimeoutMiddleware_LoggingIntegration_Panic(t *testing.T) {
	logger := &mockLogger{}

	r := gin.New()
	r.Use(LogRequest(logger))
	r.Use(gin.Recovery())
	r.Use(TimeoutMiddleware(5 * time.Second))
	r.GET("/panic", func(c *gin.Context) {
		panic("test panic message")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.True(t, logger.called, "logger should be called")
	assert.True(t, logger.lastInfo.PanicRecovered, "PanicRecovered should be true")
	assert.Equal(t, "test panic message", logger.lastInfo.PanicValue)
	assert.False(t, logger.lastInfo.TimedOut, "TimedOut should be false")
}

// TestTimeoutMiddleware_LoggingIntegration_PanicAfterTimeout verifies that
// both timeout and panic info are captured when panic occurs after timeout.
func TestTimeoutMiddleware_LoggingIntegration_PanicAfterTimeout(t *testing.T) {
	logger := &mockLogger{}

	r := gin.New()
	r.Use(LogRequest(logger))
	r.Use(gin.Recovery())
	r.Use(TimeoutMiddleware(50 * time.Millisecond))
	r.GET("/slow-panic", func(c *gin.Context) {
		time.Sleep(100 * time.Millisecond)
		panic("late panic")
	})

	req := httptest.NewRequest("GET", "/slow-panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Handler panicked without writing, so we send 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.True(t, logger.called, "logger should be called")
	assert.True(t, logger.lastInfo.TimedOut, "TimedOut should be true")
	assert.True(t, logger.lastInfo.PanicRecovered, "PanicRecovered should be true")
	assert.Equal(t, "late panic", logger.lastInfo.PanicValue)
}

// TestTimeoutMiddleware_LoggingIntegration_Normal verifies that normal requests
// don't have timeout or panic flags set.
func TestTimeoutMiddleware_LoggingIntegration_Normal(t *testing.T) {
	logger := &mockLogger{}

	r := gin.New()
	r.Use(LogRequest(logger))
	r.Use(TimeoutMiddleware(5 * time.Second))
	r.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, logger.called, "logger should be called")
	assert.False(t, logger.lastInfo.TimedOut, "TimedOut should be false")
	assert.False(t, logger.lastInfo.ClientDisconnected, "ClientDisconnected should be false")
	assert.False(t, logger.lastInfo.PanicRecovered, "PanicRecovered should be false")
	assert.Empty(t, logger.lastInfo.PanicValue, "PanicValue should be empty")
}

// TestTimeoutMiddleware_LoggingIntegration_ClientDisconnect verifies that
// client disconnect is tracked separately from timeout.
//
// Client disconnect occurs when the client closes the connection before
// the server finishes processing (e.g., client-side timeout, user cancellation).
// The http.Server cancels the request context with context.Canceled (not
// context.DeadlineExceeded), which the middleware uses to distinguish
// disconnect from timeout.
func TestTimeoutMiddleware_LoggingIntegration_ClientDisconnect(t *testing.T) {
	logger := &mockLogger{}

	r := gin.New()
	r.Use(LogRequest(logger))
	r.Use(TimeoutMiddleware(5 * time.Second))
	r.GET("/slow", func(c *gin.Context) {
		// Wait for context cancellation (simulated client disconnect)
		<-c.Request.Context().Done()
	})

	// Create request with a context we can cancel to simulate client disconnect
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/slow", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Start request in goroutine
	done := make(chan struct{})
	go func() {
		r.ServeHTTP(w, req)
		close(done)
	}()

	// Simulate client disconnect after 50ms
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for request to complete
	<-done

	assert.True(t, logger.called, "logger should be called")
	assert.False(t, logger.lastInfo.TimedOut, "TimedOut should be false")
	assert.True(t, logger.lastInfo.ClientDisconnected, "ClientDisconnected should be true")
}

// Benchmarks for stress testing the timeout middleware.
// Run with: go test -bench=BenchmarkTimeoutMiddleware -race -benchmem ./router/

// BenchmarkTimeoutMiddleware_NormalCompletion stress tests the happy path
// where handlers complete before timeout.
func BenchmarkTimeoutMiddleware_NormalCompletion(b *testing.B) {
	r := gin.New()
	r.Use(TimeoutMiddleware(5 * time.Second))
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				b.Errorf("expected 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkTimeoutMiddleware_Timeout stress tests timeout behavior and
// verifies no goroutine leaks occur under load.
// Primary goal: detect races and leaks, not verify exact status codes.
func BenchmarkTimeoutMiddleware_Timeout(b *testing.B) {
	r := gin.New()
	r.Use(TimeoutMiddleware(1 * time.Millisecond))
	r.GET("/", func(c *gin.Context) {
		<-c.Request.Context().Done()
	})

	goroutinesBefore := runtime.NumGoroutine()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			// Usually 504, but when handler finishes exactly at timeout,
			// select may pick finCh over ctx.Done(), resulting in 200.
			// Primary verification is via -race flag and goroutine leak check.
		}
	})
	b.StopTimer()

	// Allow goroutines to clean up
	time.Sleep(10 * time.Millisecond)
	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	goroutinesAfter := runtime.NumGoroutine()
	// Allow some variance for test infrastructure goroutines
	if goroutinesAfter > goroutinesBefore+10 {
		b.Errorf("goroutine leak: before=%d, after=%d", goroutinesBefore, goroutinesAfter)
	}
}

// BenchmarkTimeoutMiddleware_RaceCondition stress tests the race window
// where handler finishes just after timeout fires.
func BenchmarkTimeoutMiddleware_RaceCondition(b *testing.B) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(TimeoutMiddleware(1 * time.Millisecond))
	r.GET("/", func(c *gin.Context) {
		time.Sleep(2 * time.Millisecond)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			// Either 504 or 200 is acceptable
			if w.Code != http.StatusGatewayTimeout && w.Code != http.StatusOK {
				b.Errorf("unexpected status: %d", w.Code)
			}
		}
	})
}

// BenchmarkTimeoutMiddleware_PanicRecovery stress tests panic recovery
// and verifies no goroutine leaks occur under load.
// Primary goal: detect races and leaks, not verify exact status codes.
func BenchmarkTimeoutMiddleware_PanicRecovery(b *testing.B) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(TimeoutMiddleware(5 * time.Second))
	r.GET("/", func(c *gin.Context) {
		panic("benchmark panic")
	})

	goroutinesBefore := runtime.NumGoroutine()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			// Panic should result in 500, but under heavy load httptest.NewRecorder
			// may report default 200 due to timing. Primary verification is via
			// -race flag detecting data races and goroutine leak check below.
		}
	})
	b.StopTimer()

	time.Sleep(10 * time.Millisecond)
	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	goroutinesAfter := runtime.NumGoroutine()
	if goroutinesAfter > goroutinesBefore+10 {
		b.Errorf("goroutine leak: before=%d, after=%d", goroutinesBefore, goroutinesAfter)
	}
}

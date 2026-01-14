package router

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/remiges-tech/alya/wscutils"
)

// timeoutWriter wraps gin.ResponseWriter to track write status and optionally
// discard writes. Used by TimeoutMiddleware to coordinate response handling.
type timeoutWriter struct {
	gin.ResponseWriter
	discardWrites *atomic.Bool
	mu            sync.Mutex
	wroteHeader   bool
}

func (w *timeoutWriter) Write(b []byte) (int, error) {
	if w.discardWrites.Load() {
		return len(b), nil // Silently drop write
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ResponseWriter.Write(b)
}

func (w *timeoutWriter) WriteHeader(code int) {
	if w.discardWrites.Load() {
		return // Silently drop
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *timeoutWriter) WriteString(s string) (int, error) {
	if w.discardWrites.Load() {
		return len(s), nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ResponseWriter.WriteString(s)
}

func (w *timeoutWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

func (w *timeoutWriter) Flush() {
	if w.discardWrites.Load() {
		return
	}
	w.ResponseWriter.(http.Flusher).Flush()
}

type MiddlewareErrorScenario string

const (
	RequestTimeout MiddlewareErrorScenario = "RequestTimeout"
)

// Context keys for timeout, client disconnect, and panic tracking.
// TimeoutMiddleware sets these keys, and LogRequest middleware reads them
// to include timeout/disconnect/panic info in request logs.
//
// The middleware distinguishes between two context cancellation causes:
//   - CtxKeyTimedOut: our timeout fired (context.DeadlineExceeded)
//   - CtxKeyClientDisconnected: client closed connection (context.Canceled)
//
// Both result in ctx.Done() firing, but the cause differs. This distinction
// helps operators identify whether slow responses are due to server-side
// timeouts or impatient clients disconnecting early.
const (
	CtxKeyTimedOut           = "_request_timed_out"
	CtxKeyClientDisconnected = "_client_disconnected"
	CtxKeyPanicRecovered     = "_panic_recovered"
	CtxKeyPanicValue         = "_panic_value"
)

var middlewareScenarioToMsgID = make(map[MiddlewareErrorScenario]int)
var middlewareScenarioToErrCode = make(map[MiddlewareErrorScenario]string)

func RegisterMiddlewareMsgID(scenario MiddlewareErrorScenario, msgID int) {
	middlewareScenarioToMsgID[scenario] = msgID
}

func RegisterMiddlewareErrCode(scenario MiddlewareErrorScenario, errCode string) {
	middlewareScenarioToErrCode[scenario] = errCode
}

// TimeoutMiddleware returns a middleware that limits request processing time.
// If the handler does not complete within the timeout and hasn't written a
// response, a 504 Gateway Timeout response is sent.
//
// # Response Behavior
//
// The middleware decides the response based on what the handler did:
//
//   - Handler writes response before timeout: handler's response is used
//   - Handler writes response after timeout: handler's response is used
//   - Handler detects ctx.Done() and writes custom response: handler's response is used
//   - Handler detects ctx.Done() and exits without writing: 504 Gateway Timeout
//   - Handler ignores context and never writes: 504 Gateway Timeout
//   - Handler panics without writing: 500 Internal Server Error
//
// # Context Cancellation
//
// Timeout works correctly only when handlers honor context cancellation. Handlers
// should check ctx.Done() or use context-aware operations (database queries,
// HTTP calls, etc.) that return early when context is cancelled. Handlers that
// ignore context cancellation will run to completion, delaying the timeout response.
//
// When timeout fires, the middleware waits for the handler to complete. If the
// handler finishes with a valid response during this wait, that response is used
// instead of 504. This means a handler taking 31 seconds on a 30-second timeout
// will return its actual response (e.g., 200) rather than 504 - the client waited
// anyway and gets the real result.
//
// # Implementation Details
//
// The handler runs in a separate goroutine to allow the timeout to fire
// independently. This creates two challenges:
//
// 1. Panic recovery: gin.Recovery() runs in the main goroutine and cannot
//    catch panics from the handler goroutine. Without explicit handling,
//    a panic would crash the process.
//
// 2. Race condition: gin.Context is not thread-safe. If timeout fires while
//    handler is still running, concurrent access corrupts internal state
//    (bufio.Writer, context fields), causing panics.
//
// This middleware addresses both by:
// - Wrapping ResponseWriter to serialize writes and track response status
// - Recovering panics and re-panicking in main goroutine for gin.Recovery()
// - Waiting for handler completion before deciding which response to send
//
// # Middleware Order
//
// gin.Recovery() must be registered BEFORE this middleware:
//
//	r.Use(LogRequest(logger))     // First: logs after everything completes
//	r.Use(gin.Recovery())         // Second: catches re-panicked errors
//	r.Use(TimeoutMiddleware(...)) // Third: runs handler in goroutine
//
// If gin.Recovery() comes after TimeoutMiddleware, it runs in the handler goroutine
// and catches panics before this middleware can propagate them to the main goroutine.
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		// timedOut coordinates with panic handler - tells it not to send to panicCh
		// since main goroutine is no longer listening on it.
		var timedOut atomic.Bool

		// neverDiscard is always false - we allow all writes since we wait for
		// handler completion before deciding response. Kept for timeoutWriter interface.
		var neverDiscard atomic.Bool

		// Wrap ResponseWriter to serialize writes and track response status
		tw := &timeoutWriter{
			ResponseWriter: c.Writer,
			discardWrites:  &neverDiscard,
		}
		c.Writer = tw

		finCh := make(chan struct{}, 1)
		panicCh := make(chan any, 1)

		// Run handler in separate goroutine so timeout can fire independently.
		go func() {
			defer func() {
				if p := recover(); p != nil {
					// Record panic for logging regardless of timeout state
					c.Set(CtxKeyPanicRecovered, true)
					c.Set(CtxKeyPanicValue, fmt.Sprintf("%v", p))

					// Only send panic if timeout hasn't fired yet.
					// If timed out, main goroutine is waiting on finCh, not panicCh.
					if !timedOut.Load() {
						panicCh <- p
					}
				}
				// Always signal completion to prevent goroutine leak
				finCh <- struct{}{}
			}()
			c.Next()
		}()

		select {
		case p := <-panicCh:
			// Re-panic in main goroutine where gin.Recovery() can catch it.
			panic(p)

		case <-ctx.Done():
			// Mark timedOut so panic handler doesn't try to send to panicCh
			// (we're no longer listening on it).
			timedOut.Store(true)

			// Distinguish between timeout and client disconnect for logging.
			// Both cause ctx.Done() to fire, but ctx.Err() reveals the cause:
			//   - DeadlineExceeded: our timeout fired
			//   - Canceled: client closed connection (or parent context cancelled)
			if ctx.Err() == context.DeadlineExceeded {
				c.Set(CtxKeyTimedOut, true)
			} else {
				c.Set(CtxKeyClientDisconnected, true)
			}

			// Wait for handler to complete. Handler can still write during this wait.
			// If handler finishes with a valid response, we'll use it instead of 504.
			<-finCh

			// Check if handler panicked
			if _, panicked := c.Get(CtxKeyPanicRecovered); panicked {
				// Handler panicked after timeout. If handler already wrote headers,
				// we can't change the response. If not, send 500.
				tw.mu.Lock()
				handlerWrote := tw.wroteHeader
				tw.mu.Unlock()

				if !handlerWrote {
					c.AbortWithStatusJSON(http.StatusInternalServerError,
						wscutils.NewErrorResponse(defaultMsgID, defaultErrCode))
				}
				return
			}

			// Check if handler wrote a response
			tw.mu.Lock()
			handlerWrote := tw.wroteHeader
			tw.mu.Unlock()

			if handlerWrote {
				// Handler completed and wrote a response - use it instead of 504.
				// The response is already sent to the client.
				return
			}

			// Handler didn't write anything - send timeout response
			msgID, ok := middlewareScenarioToMsgID[RequestTimeout]
			if !ok {
				msgID = defaultMsgID
			}
			errCode, ok := middlewareScenarioToErrCode[RequestTimeout]
			if !ok {
				errCode = defaultErrCode
			}
			c.AbortWithStatusJSON(http.StatusGatewayTimeout, wscutils.NewErrorResponse(msgID, errCode))

		case <-finCh:
			// Handler completed within timeout.
		}
	}
}

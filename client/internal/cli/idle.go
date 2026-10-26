package cli

import (
	"io"
	"sync/atomic"
	"time"
)

// defaultIdleTimeout is how long a stream may go without a single byte before
// the connection counts as dead. The relay heartbeats every fifteen seconds, so
// a healthy stream resets the timer long before this.
const defaultIdleTimeout = 5 * time.Minute

// idleTimeoutSetting is empty in a normal build. The test harness injects a
// shorter duration through -ldflags, so the five-minute rule can be exercised
// without making every test wait five minutes.
var idleTimeoutSetting = ""

func idleTimeout() time.Duration {
	if idleTimeoutSetting == "" {
		return defaultIdleTimeout
	}
	timeout, err := time.ParseDuration(idleTimeoutSetting)
	if err != nil {
		return defaultIdleTimeout
	}
	return timeout
}

// idleReader treats a read gap longer than the timeout as death. The relay
// heartbeats every fifteen seconds, so a healthy stream resets the timer long
// before five minutes; the timer is a backstop for a connection that dies
// inside a heartbeat interval, or a relay that stops writing without closing
// the socket.
type idleReader struct {
	r       io.Reader
	timeout time.Duration
	timer   *time.Timer
	stale   atomic.Bool
}

func newIdleReader(r io.Reader, timeout time.Duration) *idleReader {
	reader := &idleReader{r: r, timeout: timeout}
	reader.timer = time.AfterFunc(timeout, func() {
		reader.stale.Store(true)
		if closer, ok := r.(io.Closer); ok {
			_ = closer.Close()
		}
	})
	return reader
}

func (r *idleReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.stale.Store(false)
		r.timer.Reset(r.timeout)
	}
	return n, err
}

func (r *idleReader) expired() bool {
	return r.stale.Load()
}

func (r *idleReader) stop() {
	r.timer.Stop()
}

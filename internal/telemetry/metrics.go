package telemetry

import "sync/atomic"

// Metrics tracks operational statistics.
type Metrics struct {
	ActiveConnections int64 `json:"active_connections"`
	TotalConnections  int64 `json:"total_connections"`
	TotalBytesTx      int64 `json:"total_bytes_tx"`
	TotalBytesRx      int64 `json:"total_bytes_rx"`
	TotalErrors       int64 `json:"total_errors"`
	RejectedConns     int64 `json:"rejected_connections"`
}

var globalMetrics Metrics

// GetMetrics returns a snapshot of the current metrics.
func GetMetrics() Metrics {
	return Metrics{
		ActiveConnections: atomic.LoadInt64(&globalMetrics.ActiveConnections),
		TotalConnections:  atomic.LoadInt64(&globalMetrics.TotalConnections),
		TotalBytesTx:      atomic.LoadInt64(&globalMetrics.TotalBytesTx),
		TotalBytesRx:      atomic.LoadInt64(&globalMetrics.TotalBytesRx),
		TotalErrors:       atomic.LoadInt64(&globalMetrics.TotalErrors),
		RejectedConns:     atomic.LoadInt64(&globalMetrics.RejectedConns),
	}
}

// AddActiveConnection increments the active connection count.
func AddActiveConnection(n int64) {
	atomic.AddInt64(&globalMetrics.ActiveConnections, n)
}

// GetActiveConnections returns the active connection count.
func GetActiveConnections() int64 {
	return atomic.LoadInt64(&globalMetrics.ActiveConnections)
}

// AddTotalConnection increments the total connection count.
func AddTotalConnection() {
	atomic.AddInt64(&globalMetrics.TotalConnections, 1)
}

// AddBytesTx adds to the total bytes transmitted.
func AddBytesTx(n int64) {
	atomic.AddInt64(&globalMetrics.TotalBytesTx, n)
}

// AddBytesRx adds to the total bytes received.
func AddBytesRx(n int64) {
	atomic.AddInt64(&globalMetrics.TotalBytesRx, n)
}

// AddError increments the total error count.
func AddError() {
	atomic.AddInt64(&globalMetrics.TotalErrors, 1)
}

// AddRejectedConn increments the rejected connection count.
func AddRejectedConn() {
	atomic.AddInt64(&globalMetrics.RejectedConns, 1)
}

// ResetMetrics is used for testing.
func ResetMetrics() {
	globalMetrics = Metrics{}
}

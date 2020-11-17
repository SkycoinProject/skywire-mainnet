package router

import (
	"sync"
	"sync/atomic"
	"time"
)

type networkStats struct {
	bandwidthSent     uint64
	bandwidthReceived uint64
	latency           uint32
	throughput        uint32

	bandwidthReceivedRecStartMu sync.Mutex
	bandwidthReceivedRecStart   time.Time
}

func newNetworkStats() *networkStats {
	return &networkStats{
		bandwidthReceivedRecStart: time.Now(),
	}
}

func (s *networkStats) SetLatency(latency time.Duration) {
	atomic.StoreUint32(&s.latency, uint32(latency.Milliseconds()))
}

func (s *networkStats) Latency() time.Duration {
	latencyMs := atomic.LoadUint32(&s.latency)

	return time.Duration(latencyMs)
}

func (s *networkStats) SetLocalThroughput(throughput uint32) {
	atomic.StoreUint32(&s.throughput, throughput)
}

func (s *networkStats) LocalThroughput() uint32 {
	return atomic.LoadUint32(&s.throughput)
}

func (s *networkStats) BandwidthSent() uint64 {
	return atomic.LoadUint64(&s.bandwidthSent)
}

func (s *networkStats) BandwidthReceived() uint64 {
	return atomic.LoadUint64(&s.bandwidthReceived)
}

func (s *networkStats) AddBandwidthSent(amount uint64) {
	atomic.AddUint64(&s.bandwidthSent, amount)
}

func (s *networkStats) AddBandwidthReceived(amount uint64) {
	atomic.AddUint64(&s.bandwidthReceived, amount)
}

func (s *networkStats) RemoteThroughput() int64 {
	s.bandwidthReceivedRecStartMu.Lock()
	timePassed := time.Since(s.bandwidthReceivedRecStart)
	s.bandwidthReceivedRecStart = time.Now()
	s.bandwidthReceivedRecStartMu.Unlock()

	bandwidth := atomic.LoadUint64(&s.bandwidthReceived)
	atomic.StoreUint64(&s.bandwidthReceived, 0)

	throughput := float64(bandwidth) / timePassed.Seconds()

	return int64(throughput)
}

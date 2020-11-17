package router

import (
	"net"
	"time"

	"github.com/skycoin/skywire/pkg/routing"
)

// NoiseRouteGroup is a route group wrapped with noise.
// Implements net.Conn.
type NoiseRouteGroup struct {
	rg *RouteGroup
	net.Conn
}

// LocalAddr returns local address.
func (nrg *NoiseRouteGroup) LocalAddr() net.Addr {
	return nrg.rg.LocalAddr()
}

// RemoteAddr returns remote address.
func (nrg *NoiseRouteGroup) RemoteAddr() net.Addr {
	return nrg.rg.RemoteAddr()
}

// IsAlive checks if connection is alive.
func (nrg *NoiseRouteGroup) IsAlive() bool {
	return nrg.rg.IsAlive()
}

// Latency returns latency till remote (ms).
func (nrg *NoiseRouteGroup) Latency() time.Duration {
	return nrg.rg.Latency()
}

// Throughput returns throughput till remote (bytes/s).
func (nrg *NoiseRouteGroup) Throughput() uint32 {
	return nrg.rg.Throughput()
}

// BandwidthSent returns amount of bandwidth sent (bytes).
func (nrg *NoiseRouteGroup) BandwidthSent() uint64 {
	return nrg.rg.BandwidthSent()
}

// BandwidthSent returns amount of bandwidth received (bytes).
func (nrg *NoiseRouteGroup) BandwidthReceived() uint64 {
	return nrg.rg.BandwidthReceived()
}

func (nrg *NoiseRouteGroup) isClosed() bool {
	return nrg.rg.isClosed()
}

func (nrg *NoiseRouteGroup) handlePacket(packet routing.Packet) error {
	return nrg.rg.handlePacket(packet)
}

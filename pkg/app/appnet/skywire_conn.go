package appnet

import (
	"net"
	"sync"
	"time"

	"github.com/skycoin/skywire/pkg/router"
)

// SkywireConn is a connection wrapper for skynet.
type SkywireConn struct {
	net.Conn
	nrg        *router.NoiseRouteGroup
	freePort   func()
	freePortMx sync.RWMutex
	once       sync.Once
}

// IsAlive checks whether connection is alive.
func (c *SkywireConn) IsAlive() bool {
	return c.nrg.IsAlive()
}

// Latency returns latency till remote (ms).
func (c *SkywireConn) Latency() time.Duration {
	return c.nrg.Latency()
}

// Throughput returns throughput till remote (bytes/s).
func (c *SkywireConn) Throughput() uint32 {
	return c.nrg.Throughput()
}

// BandwidthSent returns amount of bandwidth sent (bytes).
func (c *SkywireConn) BandwidthSent() uint64 {
	return c.nrg.BandwidthSent()
}

// BandwidthSent returns amount of bandwidth received (bytes).
func (c *SkywireConn) BandwidthReceived() uint64 {
	return c.nrg.BandwidthReceived()
}

// Close closes connection.
func (c *SkywireConn) Close() error {
	var err error

	c.once.Do(func() {
		defer func() {
			c.freePortMx.RLock()
			defer c.freePortMx.RUnlock()
			if c.freePort != nil {
				c.freePort()
			}
		}()

		err = c.Conn.Close()
	})

	return err
}

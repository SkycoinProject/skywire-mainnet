package vpn

import "net"

// UDPConnWriter implements `io.Writer` for UDP connection.
type UDPConnWriter struct {
	conn *net.UDPConn
	to   *net.UDPAddr
}

// NewUDPConnWriter constructs new `UDPConnWriter`.
func NewUDPConnWriter(conn *net.UDPConn, to *net.UDPAddr) *UDPConnWriter {
	return &UDPConnWriter{
		conn: conn,
		to:   to,
	}
}

// Write writes data `b` to the corresponding UDP address.
func (w *UDPConnWriter) Write(b []byte) (int, error) {
	return w.conn.WriteToUDP(b, w.to)
}

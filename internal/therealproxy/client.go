package therealproxy

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/hashicorp/yamux"
)

// Client implement multiplexing proxy client using yamux.
type Client struct {
	session  *yamux.Session
	listener net.Listener
}

// NewClient constructs a new Client.
func NewClient(conn net.Conn) (*Client, error) {
	session, err := yamux.Client(conn, nil)
	if err != nil {
		return nil, fmt.Errorf("yamux: %s", err)
	}

	return &Client{session: session}, nil
}

// ListenAndServe start tcp listener on addr and proxies incoming
// connection to a remote proxy server.
func (c *Client) ListenAndServe(addr string) error {
	var stream net.Conn
	var err error

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %s", err)
	}

	c.listener = l
	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accept: %s", err)
		}

		stream, err = c.session.Open()
		if err != nil {
			return fmt.Errorf("yamux: %s", err)
		}

		go func() {
			errCh := make(chan error, 2)
			go func() {
				_, err := io.Copy(stream, conn)
				errCh <- err
			}()

			go func() {
				_, err := io.Copy(conn, stream)
				errCh <- err
			}()

			for err := range errCh {
				conn.Close()
				stream.Close()

				if err != nil {
					log.Println("Copy error:", err)
				}
			}
		}()
	}
}

// Close implement io.Closer.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.listener.Close()
}

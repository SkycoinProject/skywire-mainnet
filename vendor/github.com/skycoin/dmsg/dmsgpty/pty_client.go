package dmsgpty

import (
	"fmt"
	"io"
	"net/rpc"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/dmsg/cipher"
)

// PtyClient represents the client end of a dmsgpty session.
type PtyClient struct {
	log  logrus.FieldLogger
	rpcC *rpc.Client
	done chan struct{}
	once sync.Once
}

// NewPtyClient creates a new pty client that interacts with a local pty.
func NewPtyClient(conn io.ReadWriteCloser) (*PtyClient, error) {
	if err := writeRequest(conn, PtyURI); err != nil {
		return nil, err
	}
	if err := readResponse(conn); err != nil {
		return nil, err
	}
	return &PtyClient{
		log:  logging.MustGetLogger("dmsgpty:pty-client"),
		rpcC: rpc.NewClient(conn),
		done: make(chan struct{}),
	}, nil
}

// NewProxyClient creates a new pty client that interacts with a remote pty hosted on the given dmsg pk and port.
// Interactions are proxied via the local dmsgpty.Host
func NewProxyClient(conn io.ReadWriteCloser, rPK cipher.PubKey, rPort uint16) (*PtyClient, error) {
	uri := fmt.Sprintf("%s?pk=%s&port=%d", PtyProxyURI, rPK, rPort)
	if err := writeRequest(conn, uri); err != nil {
		return nil, err
	}
	if err := readResponse(conn); err != nil {
		return nil, err
	}
	return &PtyClient{
		log:  logging.MustGetLogger("dmsgpty:proxy-client"),
		rpcC: rpc.NewClient(conn),
		done: make(chan struct{}),
	}, nil
}

// Close closes the pty and closes the connection to the remote.
func (sc *PtyClient) Close() error {
	if closed := sc.close(); !closed {
		return nil
	}
	// No need to wait for reply.
	_ = sc.Stop() //nolint:errcheck
	return sc.rpcC.Close()
}

func (sc *PtyClient) close() (closed bool) {
	sc.once.Do(func() {
		close(sc.done)
		closed = true
	})
	return closed
}

// Stop stops the pty.
func (sc *PtyClient) Stop() error {
	return sc.call("Stop", &empty, &empty)
}

// Read reads from the pty.
func (sc *PtyClient) Read(b []byte) (int, error) {
	reqN := len(b)
	var respB []byte
	err := sc.call("Read", &reqN, &respB)
	return copy(b, respB), processRPCError(err)
}

// Write writes to the pty.
func (sc *PtyClient) Write(b []byte) (int, error) {
	var n int
	err := sc.call("Write", &b, &n)
	return n, processRPCError(err)
}

func (*PtyClient) rpcMethod(m string) string {
	return PtyRPCName + "." + m
}

func (sc *PtyClient) call(method string, args, reply interface{}) error {
	call := sc.rpcC.Go(sc.rpcMethod(method), args, reply, nil)
	select {
	case <-sc.done:
		return io.ErrClosedPipe // TODO(evanlinjin): Is there a better error to use?
	case <-call.Done:
		return call.Error
	}
}

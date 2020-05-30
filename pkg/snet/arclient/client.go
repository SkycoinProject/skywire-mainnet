// Package arclient implements address resolver client
package arclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/libp2p/go-reuseport"
	"nhooyr.io/websocket"

	"github.com/SkycoinProject/skywire-mainnet/internal/httpauth"
)

var log = logging.MustGetLogger("arclient")

const (
	bindPath             = "/bind"
	resolvePath          = "/resolve/"
	resolveHolePunchPath = "/resolve_hole_punch/"
	wsPath               = "/ws"
	addrChSize           = 1024
)

var (
	// ErrNoEntry means that there exists no entry for this PK.
	ErrNoEntry = errors.New("no entry for this PK")
	// ErrNotConnected is returned when PK is not connected.
	ErrNotConnected = errors.New("this PK is not connected")
)

// Error is the object returned to the client when there's an error.
type Error struct {
	Error string `json:"error"`
}

// APIClient implements DMSG discovery API client.
type APIClient interface {
	io.Closer
	LocalAddr() string
	Bind(ctx context.Context, port string) error
	Resolve(ctx context.Context, pk cipher.PubKey) (string, error)
	ResolveHolePunch(ctx context.Context, pk cipher.PubKey) (string, error)
	WS(ctx context.Context, dialCh <-chan cipher.PubKey) (<-chan RemoteVisor, error)
}

// httpClient implements Client for uptime tracker API.
type httpClient struct {
	client    *httpauth.Client
	localAddr string
	pk        cipher.PubKey
	sk        cipher.SecKey
	wsConn    *websocket.Conn
}

// NewHTTP creates a new client setting a public key to the client to be used for auth.
// When keys are set, the client will sign request before submitting.
// The signature information is transmitted in the header using:
// * SW-Public: The specified public key
// * SW-Nonce:  The nonce for that public key
// * SW-Sig:    The signature of the payload + the nonce
func NewHTTP(remoteAddr string, pk cipher.PubKey, sk cipher.SecKey) (APIClient, error) {
	httpAuthClient, err := httpauth.NewClient(context.Background(), remoteAddr, pk, sk)
	if err != nil {
		return nil, fmt.Errorf("address resolver httpauth: %w", err)
	}

	client := &httpClient{
		client:    httpAuthClient,
		pk:        pk,
		sk:        sk,
		localAddr: "",
	}

	transport := &http.Transport{
		DialContext: func(_ context.Context, network, remoteAddr string) (conn net.Conn, err error) {
			conn, err = reuseport.Dial(network, client.localAddr, remoteAddr)
			if err == nil && client.localAddr == "" {
				client.localAddr = conn.LocalAddr().String()
			}

			return conn, err
		},
		DisableKeepAlives: false,
	}

	httpAuthClient.SetTransport(transport)

	return client, nil
}

func (c *httpClient) LocalAddr() string {
	return c.localAddr
}

// Get performs a new GET request.
func (c *httpClient) Get(ctx context.Context, path string) (*http.Response, error) {
	addr := c.client.Addr() + path

	req, err := http.NewRequest(http.MethodGet, addr, new(bytes.Buffer))
	if err != nil {
		return nil, err
	}

	return c.client.Do(req.WithContext(ctx))
}

// Post performs a POST request.
func (c *httpClient) Post(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(payload); err != nil {
		return nil, err
	}

	addr := c.client.Addr() + path

	req, err := http.NewRequest(http.MethodPost, addr, body)
	if err != nil {
		return nil, err
	}

	return c.client.Do(req.WithContext(ctx))
}

// Websocket performs a new websocket request.
func (c *httpClient) Websocket(ctx context.Context, path string) (*websocket.Conn, error) {
	header, err := c.client.Header()
	if err != nil {
		return nil, err
	}

	dialOpts := &websocket.DialOptions{
		HTTPClient: c.client.ReuseClient(),
		HTTPHeader: header,
	}

	addr, err := url.Parse(c.client.Addr())
	if err != nil {
		return nil, err
	}
	switch addr.Scheme {
	case "http":
		addr.Scheme = "ws"
	case "https":
		addr.Scheme = "wss"
	}

	addr.Path = path

	conn, resp, err := websocket.Dial(ctx, addr.String(), dialOpts)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		c.client.IncrementNonce()
	}

	return conn, nil
}

// BindRequest stores bind request values.
type BindRequest struct {
	Port string `json:"port"`
}

// Bind binds client PK to IP:port on address resolver.
func (c *httpClient) Bind(ctx context.Context, port string) error {
	req := BindRequest{
		Port: port,
	}

	resp, err := c.Post(ctx, bindPath, req)
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status: %d, error: %w", resp.StatusCode, extractError(resp.Body))
	}

	return nil
}

// ResolveResponse stores response response values.
type ResolveResponse struct {
	Addr string `json:"addr"`
}

func (c *httpClient) Resolve(ctx context.Context, pk cipher.PubKey) (string, error) {
	resp, err := c.Get(ctx, resolvePath+pk.String())
	if err != nil {
		return "", err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrNoEntry
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status: %d, error: %w", resp.StatusCode, extractError(resp.Body))
	}

	rawBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var resolveResp ResolveResponse

	if err := json.Unmarshal(rawBody, &resolveResp); err != nil {
		return "", err
	}

	return resolveResp.Addr, nil
}

func (c *httpClient) ResolveHolePunch(ctx context.Context, pk cipher.PubKey) (string, error) {
	resp, err := c.Get(ctx, resolveHolePunchPath+pk.String())
	if err != nil {
		return "", err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrNoEntry
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status: %d, error: %w", resp.StatusCode, extractError(resp.Body))
	}

	rawBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var resolveResp ResolveResponse

	if err := json.Unmarshal(rawBody, &resolveResp); err != nil {
		return "", err
	}

	return resolveResp.Addr, nil
}

// RemoteVisor contains public key and address of remote visor.
type RemoteVisor struct {
	PK   cipher.PubKey
	Addr string
}

func (c *httpClient) WS(ctx context.Context, dialCh <-chan cipher.PubKey) (<-chan RemoteVisor, error) {
	addrCh := make(chan RemoteVisor, addrChSize)

	if c.wsConn != nil {
		if err := c.wsConn.Close(websocket.StatusNormalClosure, "new connection created"); err != nil {
			log.WithError(err).Warnf("Failed to close WebSocket connection")
		}
	}

	conn, err := c.Websocket(ctx, wsPath)
	if err != nil {
		return nil, err
	}

	c.wsConn = conn

	go func(conn *websocket.Conn, addrCh chan<- RemoteVisor) {
		defer func() {
			close(addrCh)
		}()

		for {
			kind, rawMsg, err := conn.Read(context.TODO())
			if err != nil {
				log.Errorf("Failed to read WS message: %v", err)
				return
			}

			log.Infof("New WS message of type %v: %v", kind.String(), string(rawMsg))

			var remote RemoteVisor
			if err := json.Unmarshal(rawMsg, &remote); err != nil {
				log.Errorf("Failed to read unmarshal message: %v", err)
				continue
			}

			addrCh <- remote
		}
	}(conn, addrCh)

	go func(conn *websocket.Conn, dialCh <-chan cipher.PubKey) {
		for pk := range dialCh {
			if err := conn.Write(ctx, websocket.MessageText, []byte(pk.String())); err != nil {
				log.Errorf("Failed to write to %v: %v", pk, err)
				return
			}
		}
	}(conn, dialCh)

	return addrCh, nil
}

func (c *httpClient) Close() error {
	return c.wsConn.Close(websocket.StatusNormalClosure, "client closed")
}

// extractError returns the decoded error message from Body.
func extractError(r io.Reader) error {
	var apiError Error

	body, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, &apiError); err != nil {
		return errors.New(string(body))
	}

	return errors.New(apiError.Error)
}

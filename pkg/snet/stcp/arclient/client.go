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
	"runtime/debug"
	"strings"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/libp2p/go-reuseport"
	"nhooyr.io/websocket"

	"github.com/SkycoinProject/skywire-mainnet/internal/httpauth"
)

var log = logging.MustGetLogger("arclient")

const (
	bindPath    = "/bind"
	resolvePath = "/resolve/"
	dialPath    = "/dial/"
	listenPath  = "/listen"
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
	Bind(ctx context.Context, port string) error
	Resolve(ctx context.Context, pk cipher.PubKey) (string, error)
	Dial(ctx context.Context, pk cipher.PubKey) (string, error)
	Listen(ctx context.Context, dialCh <-chan cipher.PubKey) (<-chan string, error)
}

// httpClient implements Client for uptime tracker API.
type httpClient struct {
	client    *httpauth.Client
	localAddr string
	pk        cipher.PubKey
	sk        cipher.SecKey
}

type ClientOption func(c *httpClient)

// NewHTTP creates a new client setting a public key to the client to be used for auth.
// When keys are set, the client will sign request before submitting.
// The signature information is transmitted in the header using:
// * SW-Public: The specified public key
// * SW-Nonce:  The nonce for that public key
// * SW-Sig:    The signature of the payload + the nonce
func NewHTTP(remoteAddr string, pk cipher.PubKey, sk cipher.SecKey, opts ...ClientOption) (APIClient, error) {
	httpAuthClient, err := httpauth.NewClient(context.Background(), remoteAddr, pk, sk)
	if err != nil {
		return nil, fmt.Errorf("httpauth: %w", err)
	}

	client := &httpClient{client: httpAuthClient, pk: pk, sk: sk}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

func LocalAddr(localAddr string) ClientOption {
	return func(c *httpClient) {
		c.localAddr = localAddr

		transport := &http.Transport{
			DialContext: func(_ context.Context, network, addr string) (conn net.Conn, err error) {
				fmt.Printf("[LocalAddr] Dialing %v from %v via %v\nstack: %v\n", addr, localAddr, network, string(debug.Stack()))
				return reuseport.Dial(network, localAddr, addr)
			},
			DisableKeepAlives: false,
		}

		c.client.SetTransport(transport)
	}
}

// Get performs a new GET request.
func (c *httpClient) Get(ctx context.Context, path string) (*http.Response, error) {
	addr := c.client.Addr() + path
	fmt.Printf("[get request] addr: %v\n", addr)

	req, err := http.NewRequest(http.MethodGet, addr, new(bytes.Buffer))
	if err != nil {
		return nil, err
	}

	return c.client.DoWithReuse(req.WithContext(ctx))
}

// Post performs a POST request.
func (c *httpClient) Post(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(payload); err != nil {
		return nil, err
	}

	addr := c.client.Addr() + path
	fmt.Printf("[post request] addr: %v\n", addr)

	req, err := http.NewRequest(http.MethodPost, addr, body)
	if err != nil {
		return nil, err
	}

	return c.client.DoWithReuse(req.WithContext(ctx))
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

	addr.Host = strings.Replace(addr.Host, "9093", "9095", -1)

	addr.Path = path

	var conn *websocket.Conn
	for {
		fmt.Printf("[websocket] dialing %v\n", addr.String())

		wsConn, resp, err := websocket.Dial(ctx, addr.String(), dialOpts)
		if err != nil {
			return nil, err
		}

		fmt.Printf("[websocket] dialed, pinging %v\n", addr.String())

		if err := wsConn.Write(context.TODO(), websocket.MessageText, []byte("ping")); err != nil {
			return nil, fmt.Errorf("ping: %w", err)
		}

		fmt.Printf("[websocket] pinged %v\n", addr.String())

		repeat, err := c.client.CheckResponse(resp)
		if err != nil {
			return nil, err
		}

		if !repeat {
			conn = wsConn
			break
		}
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
		return fmt.Errorf("status: %d, error: %v", resp.StatusCode, extractError(resp.Body))
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
		return "", fmt.Errorf("status: %d, error: %v", resp.StatusCode, extractError(resp.Body))
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

func (c *httpClient) Dial(ctx context.Context, pk cipher.PubKey) (string, error) {
	resp, err := c.Post(ctx, dialPath+pk.String(), nil)
	if err != nil {
		return "", err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode == http.StatusUnprocessableEntity {
		return "", ErrNotConnected
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status: %d, error: %v", resp.StatusCode, extractError(resp.Body))
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

func (c *httpClient) Listen(ctx context.Context, dialCh <-chan cipher.PubKey) (<-chan string, error) {
	addrCh := make(chan string)

	conn, err := c.Websocket(ctx, listenPath)
	if err != nil {
		return nil, err
	}

	go func(conn *websocket.Conn, addrCh chan<- string) {
		defer func() {
			close(addrCh)
		}()

		for {
			typ, rawMsg, err := conn.Read(context.TODO())
			if err != nil {
				log.Errorf("Failed to read WS message: %v", err)
				return
			}

			msg := string(rawMsg)

			log.Infof("New WS message of type %v: %v", typ.String(), msg)
			addrCh <- msg
		}
	}(conn, addrCh)

	//go func(conn net.Conn, dialCh <-chan cipher.PubKey) {
	//	for dial := range dialCh {
	//		_, err := conn.Write([]byte(dial.String()))
	//		if err != nil {
	//			log.Errorf("Failed to write to %v: %v", conn.RemoteAddr(), err)
	//			return
	//		}
	//	}
	//}(conn, dialCh)

	return addrCh, nil
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

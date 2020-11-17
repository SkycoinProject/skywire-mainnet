package httpauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/skycoin/src/util/logging"
)

const (
	invalidNonceErrorMessage = "SW-Nonce does not match"
)

var log = logging.MustGetLogger("httpauth")

// NextNonceResponse represents a ServeHTTP response for json encoding
type NextNonceResponse struct {
	Edge      cipher.PubKey `json:"edge"`
	NextNonce Nonce         `json:"next_nonce"`
}

// HTTPResponse represents the http response struct
type HTTPResponse struct {
	Error *HTTPError  `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}

// HTTPError is included in an HTTPResponse
type HTTPError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Client implements Client for auth services.
// As Client needs to dial both with reusing address and without it, it uses two http clients: reuseClient and client.
type Client struct {
	// atomic requires 64-bit alignment for struct field access
	nonce       uint64
	mu          sync.Mutex
	reqMu       sync.Mutex
	client      *http.Client
	reuseClient *http.Client
	key         cipher.PubKey
	sec         cipher.SecKey
	addr        string // sanitized address of the client, which may differ from addr used in NewClient
}

// NewClient creates a new client setting a public key to the client to be used for Auth.
// When keys are set, the client will sign request before submitting.
// The signature information is transmitted in the header using:
// * SW-Public: The specified public key
// * SW-Nonce:  The nonce for that public key
// * SW-Sig:    The signature of the payload + the nonce
func NewClient(ctx context.Context, addr string, key cipher.PubKey, sec cipher.SecKey) (*Client, error) {
	c := &Client{
		client:      &http.Client{},
		reuseClient: &http.Client{},
		key:         key,
		sec:         sec,
		addr:        sanitizedAddr(addr),
	}

	// request server for a nonce
	nonce, err := c.Nonce(ctx, c.key)
	if err != nil {
		return nil, err
	}
	c.nonce = uint64(nonce)

	return c, nil
}

// Header returns headers for httpauth.
func (c *Client) Header() (http.Header, error) {
	nonce := c.getCurrentNonce()
	body := make([]byte, 0)
	sign, err := Sign(body, nonce, c.sec)
	if err != nil {
		return nil, err
	}

	header := make(http.Header)

	// use nonce, later, if no err from req update such nonce
	header.Set("SW-Nonce", strconv.FormatUint(uint64(nonce), 10))
	header.Set("SW-Sig", sign.Hex())
	header.Set("SW-Public", c.key.Hex())

	return header, nil
}

// Do performs a new authenticated Request and returns the response. Internally, if the request was
// successful nonce is incremented
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.do(c.client, req)
}

func (c *Client) do(client *http.Client, req *http.Request) (*http.Response, error) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	body := make([]byte, 0)
	if req.ContentLength != 0 {
		auxBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		if err := req.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close HTTP request body")
		}
		req.Body = ioutil.NopCloser(bytes.NewBuffer(auxBody))
		body = auxBody
	}

	resp, err := c.doRequest(client, req, body)
	if err != nil {
		return nil, err
	}

	isNonceValid, err := isNonceValid(resp)
	if err != nil {
		return nil, err
	}

	if !isNonceValid {
		nonce, err := c.Nonce(context.Background(), c.key)
		if err != nil {
			return nil, err
		}
		c.SetNonce(nonce)

		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close HTTP response body")
		}

		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))

		resp, err = c.doRequest(client, req, body)
		if err != nil {
			return nil, err
		}
	}

	if resp.StatusCode == http.StatusOK {
		c.IncrementNonce()
	}

	return resp, nil
}

// Nonce calls the remote API to retrieve the next expected nonce
func (c *Client) Nonce(ctx context.Context, key cipher.PubKey) (Nonce, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req, err := http.NewRequest(http.MethodGet, c.addr+"/security/nonces/"+key.Hex(), nil)
	if err != nil {
		return 0, err
	}

	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close HTTP response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("error getting current nonce: status: %d <- %v", resp.StatusCode, extractError(resp.Body))
	}

	var nr NextNonceResponse
	if err := json.NewDecoder(resp.Body).Decode(&nr); err != nil {
		return 0, err
	}

	return nr.NextNonce, nil
}

// ReuseClient returns HTTP client that reuses port for dialing.
func (c *Client) ReuseClient() *http.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.reuseClient
}

// SetTransport sets transport for HTTP client that reuses port for dialing.
func (c *Client) SetTransport(transport http.RoundTripper) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.reuseClient.Transport = transport
}

// SetNonce sets client current nonce to given nonce
func (c *Client) SetNonce(n Nonce) {
	atomic.StoreUint64(&c.nonce, uint64(n))
}

// Addr returns sanitized address of the client
func (c *Client) Addr() string {
	return c.addr
}

func (c *Client) doRequest(client *http.Client, req *http.Request, body []byte) (*http.Response, error) {
	nonce := c.getCurrentNonce()
	sign, err := Sign(body, nonce, c.sec)
	if err != nil {
		return nil, err
	}

	// use nonce, later, if no err from req update such nonce
	req.Header.Set("SW-Nonce", strconv.FormatUint(uint64(nonce), 10))
	req.Header.Set("SW-Sig", sign.Hex())
	req.Header.Set("SW-Public", c.key.Hex())

	c.mu.Lock()
	defer c.mu.Unlock()

	return client.Do(req)
}

func (c *Client) getCurrentNonce() Nonce {
	return Nonce(atomic.LoadUint64(&c.nonce))
}

// IncrementNonce increments client's current nonce.
func (c *Client) IncrementNonce() {
	atomic.AddUint64(&c.nonce, 1)
}

// isNonceValid checks if `res` contains an invalid nonce error.
// The error is occurred if status code equals to `http.StatusUnauthorized`
// and body contains `invalidNonceErrorMessage`.
func isNonceValid(res *http.Response) (bool, error) {
	var serverResponse HTTPResponse

	auxRespBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, err
	}
	if err := res.Body.Close(); err != nil {
		return false, err
	}
	res.Body = ioutil.NopCloser(bytes.NewBuffer(auxRespBody))

	if err := json.Unmarshal(auxRespBody, &serverResponse); err != nil || serverResponse.Error == nil {
		return true, nil
	}

	isAuthorized := serverResponse.Error.Code != http.StatusUnauthorized
	hasValidNonce := serverResponse.Error.Message != invalidNonceErrorMessage

	return isAuthorized && hasValidNonce, nil
}

func sanitizedAddr(addr string) string {
	if addr == "" {
		return "http://localhost"
	}

	u, err := url.Parse(addr)
	if err != nil {
		return "http://localhost"
	}

	if u.Scheme == "" {
		u.Scheme = "http"
	}

	u.Path = strings.TrimSuffix(u.Path, "/")
	return u.String()
}

// extractError returns the decoded error message from Body.
func extractError(r io.Reader) error {
	var serverError HTTPResponse

	body, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, &serverError); err != nil {
		return errors.New(string(body))
	}

	return errors.New(serverError.Error.Message)
}

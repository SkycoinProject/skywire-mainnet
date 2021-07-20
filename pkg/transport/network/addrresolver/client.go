// Package addrresolver implements address resolver client
package addrresolver

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
	"time"

	"github.com/AudriusButkevicius/pfilter"
	"github.com/skycoin/dmsg/cipher"
	dmsgnetutil "github.com/skycoin/dmsg/netutil"
	"github.com/skycoin/skycoin/src/util/logging"
	"github.com/xtaci/kcp-go"

	"github.com/skycoin/skywire/internal/httpauth"
	"github.com/skycoin/skywire/internal/netutil"
	"github.com/skycoin/skywire/internal/packetfilter"
)

const (
	// sudphPriority is used to set an order how connection filters apply.
	sudphPriority        = 1
	stcprBindPath        = "/bind/stcpr"
	addrChSize           = 1024
	udpKeepAliveInterval = 10 * time.Second
	udpKeepAliveMessage  = "keepalive"
	defaultUDPPort       = "30178"
)

var (
	// ErrNoEntry means that there exists no entry for this PK.
	ErrNoEntry = errors.New("no entry for this PK")
	// ErrNotReady is returned when address resolver is not ready.
	ErrNotReady = errors.New("address resolver is not ready")
)

// Error is the object returned to the client when there's an error.
type Error struct {
	Error string `json:"error"`
}

//go:generate mockery -name APIClient -case underscore -inpkg

// APIClient implements address resolver API client.
type APIClient interface {
	io.Closer
	BindSTCPR(ctx context.Context, port string) error
	BindSUDPH(filter *pfilter.PacketFilter, handshake Handshake) (<-chan RemoteVisor, error)
	Resolve(ctx context.Context, netType string, pk cipher.PubKey) (VisorData, error)
	Health(ctx context.Context) (int, error)
}

// VisorData stores visor data.
type VisorData struct {
	RemoteAddr string `json:"remote_addr"`
	IsLocal    bool   `json:"is_local,omitempty"`
	LocalAddresses
}

// httpClient implements APIClient for address resolver API.
type httpClient struct {
	log            *logging.Logger
	httpClient     *httpauth.Client
	pk             cipher.PubKey
	sk             cipher.SecKey
	remoteHTTPAddr string
	remoteUDPAddr  string
	sudphConn      net.PacketConn
	ready          chan struct{}
	closed         chan struct{}
}

// NewHTTP creates a new client setting a public key to the client to be used for auth.
// When keys are set, the client will sign request before submitting.
// The signature information is transmitted in the header using:
// * SW-Public: The specified public key.
// * SW-Nonce:  The nonce for that public key.
// * SW-Sig:    The signature of the payload + the nonce.
func NewHTTP(remoteAddr string, pk cipher.PubKey, sk cipher.SecKey, log *logging.Logger) (APIClient, error) {
	remoteURL, err := url.Parse(remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}

	remoteUDP := remoteURL.Host
	if _, _, err := net.SplitHostPort(remoteUDP); err != nil {
		remoteUDP = net.JoinHostPort(remoteUDP, defaultUDPPort)
	}

	client := &httpClient{
		log:            log,
		pk:             pk,
		sk:             sk,
		remoteHTTPAddr: remoteAddr,
		remoteUDPAddr:  remoteUDP,
		ready:          make(chan struct{}),
		closed:         make(chan struct{}),
	}

	client.log.Infof("Remote UDP server: %q", remoteUDP)

	go client.initHTTPClient()

	return client, nil
}

func (c *httpClient) initHTTPClient() {
	httpAuthClient, err := httpauth.NewClient(context.Background(), c.remoteHTTPAddr, c.pk, c.sk)
	if err != nil {
		c.log.WithError(err).
			Warnf("Failed to connect to address resolver. STCPR/SUDPH services are temporarily unavailable. Retrying...")

		retryLog := logging.MustGetLogger("snet.arclient.retrier")
		retry := dmsgnetutil.NewRetrier(retryLog, 1*time.Second, 10*time.Second, 0, 1)

		err := retry.Do(context.Background(), func() error {
			httpAuthClient, err = httpauth.NewClient(context.Background(), c.remoteHTTPAddr, c.pk, c.sk)
			return err
		})

		if err != nil {
			// This should not happen as retrier is set to try indefinitely.
			// If address resolver cannot be contacted indefinitely, 'c.ready' will be blocked indefinitely.
			c.log.WithError(err).Fatal("Permanently failed to connect to address resolver.")
		}
	}

	c.log.Infof("Connected to address resolver. STCPR/SUDPH services are available.")

	c.httpClient = httpAuthClient
	close(c.ready)
}

// Get performs a new GET request.
func (c *httpClient) Get(ctx context.Context, path string) (*http.Response, error) {
	<-c.ready

	addr := c.httpClient.Addr() + path

	req, err := http.NewRequest(http.MethodGet, addr, new(bytes.Buffer))
	if err != nil {
		return nil, err
	}

	return c.httpClient.Do(req.WithContext(ctx))
}

// Post performs a POST request.
func (c *httpClient) Post(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	<-c.ready

	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(payload); err != nil {
		return nil, err
	}

	addr := c.httpClient.Addr() + path

	req, err := http.NewRequest(http.MethodPost, addr, body)
	if err != nil {
		return nil, err
	}

	return c.httpClient.Do(req.WithContext(ctx))
}

// BindRequest stores bind request values.
type BindRequest struct {
	Port string `json:"port"`
}

// LocalAddresses contains outbound port and all network addresses of visor.
type LocalAddresses struct {
	Port      string   `json:"port"`
	Addresses []string `json:"addresses"`
}

// BindSTCPR binds client PK to IP:port on address resolver.
func (c *httpClient) BindSTCPR(ctx context.Context, port string) error {
	if !c.isReady() {
		c.log.Infof("BindSTCPR: Address resolver is not ready yet, waiting...")
		<-c.ready
		c.log.Infof("BindSTCPR: Address resolver became ready, binding")
	}

	addresses, err := netutil.LocalAddresses()
	if err != nil {
		return err
	}

	localAddresses := LocalAddresses{
		Addresses: addresses,
		Port:      port,
	}
	c.log.Infof("BindSTCPR: Address resolver binding with: %v", addresses)
	resp, err := c.Post(ctx, stcprBindPath, localAddresses)
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.WithError(err).Warn("Failed to close response body")
		}
	}()

	c.log.Infof("BindSTCPR: Address resolver response: %v", resp)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status: %d, error: %w", resp.StatusCode, extractError(resp.Body))
	}

	return nil
}

// Handshake type is used to decouple client from handshake and network packages
type Handshake func(net.Conn) (net.Conn, error)

func (c *httpClient) BindSUDPH(filter *pfilter.PacketFilter, hs Handshake) (<-chan RemoteVisor, error) {
	if !c.isReady() {
		c.log.Infof("BindSUDPR: Address resolver is not ready yet, waiting...")
		<-c.ready
		c.log.Infof("BindSUDPR: Address resolver became ready, binding")
	}

	rAddr, err := net.ResolveUDPAddr("udp", c.remoteUDPAddr)
	if err != nil {
		return nil, err
	}

	c.sudphConn = filter.NewConn(sudphPriority, packetfilter.NewAddressFilter(rAddr))

	_, localPort, err := net.SplitHostPort(c.sudphConn.LocalAddr().String())
	if err != nil {
		return nil, err
	}

	c.log.Infof("SUDPH Local port: %v", localPort)
	kcpConn, err := kcp.NewConn(c.remoteUDPAddr, nil, 0, 0, c.sudphConn)
	if err != nil {
		return nil, err
	}
	arConn, err := hs(kcpConn)
	if err != nil {
		return nil, err
	}

	addresses, err := netutil.LocalAddresses()
	if err != nil {
		return nil, err
	}

	localAddresses := LocalAddresses{
		Addresses: addresses,
		Port:      localPort,
	}

	laData, err := json.Marshal(localAddresses)
	if err != nil {
		return nil, err
	}

	if _, err := arConn.Write(laData); err != nil {
		return nil, err
	}

	addrCh := c.readSUDPHMessages(arConn)

	go func() {
		if err := c.keepAliveLoop(arConn); err != nil {
			c.log.WithError(err).Errorf("Failed to send keep alive UDP packet to address-resolver")
		}
	}()

	return addrCh, nil
}

func (c *httpClient) Resolve(ctx context.Context, tType string, pk cipher.PubKey) (VisorData, error) {
	if !c.isReady() {
		return VisorData{}, ErrNotReady
	}

	path := fmt.Sprintf("/resolve/%s/%s", tType, pk.String())

	resp, err := c.Get(ctx, path)
	if err != nil {
		return VisorData{}, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.WithError(err).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return VisorData{}, ErrNoEntry
	}

	if resp.StatusCode != http.StatusOK {
		return VisorData{}, fmt.Errorf("status: %d, error: %w", resp.StatusCode, extractError(resp.Body))
	}

	rawBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return VisorData{}, err
	}

	var resolveResp VisorData

	if err := json.Unmarshal(rawBody, &resolveResp); err != nil {
		return VisorData{}, err
	}

	return resolveResp, nil
}

func (c *httpClient) Health(ctx context.Context) (int, error) {
	if !c.isReady() {
		return http.StatusNotFound, nil
	}

	resp, err := c.Get(ctx, "/health")
	if err != nil {
		return 0, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.WithError(err).Warn("Failed to close response body")
		}
	}()

	return resp.StatusCode, nil
}

func (c *httpClient) isReady() bool {
	select {
	case <-c.ready:
		return true
	default:
		return false
	}
}

// RemoteVisor contains public key and address of remote visor.
type RemoteVisor struct {
	PK   cipher.PubKey
	Addr string
}

func (c *httpClient) readSUDPHMessages(reader io.Reader) <-chan RemoteVisor {
	addrCh := make(chan RemoteVisor, addrChSize)

	go func(addrCh chan<- RemoteVisor) {
		defer func() {
			close(addrCh)
		}()

		buf := make([]byte, 4096)

		for {
			select {
			case <-c.closed:
				return
			default:
				n, err := reader.Read(buf)
				if err != nil {
					c.log.Errorf("Failed to read SUDPH message: %v", err)
					return
				}

				c.log.Infof("New SUDPH message: %v", string(buf[:n]))

				var remote RemoteVisor
				if err := json.Unmarshal(buf[:n], &remote); err != nil {
					c.log.Errorf("Failed to read unmarshal message: %v", err)
					continue
				}

				addrCh <- remote
			}
		}
	}(addrCh)

	return addrCh
}

func (c *httpClient) Close() error {
	select {
	case <-c.closed:
		return nil // already closed
	default: // close
	}

	defer func() {
		c.sudphConn = nil
	}()

	if c.sudphConn != nil {
		if err := c.sudphConn.Close(); err != nil {
			c.log.WithError(err).Errorf("Failed to close SUDPH")
		}
		close(c.closed)
	}

	return nil
}

// Keep NAT mapping alive.
func (c *httpClient) keepAliveLoop(w io.Writer) error {
	for {
		select {
		case <-c.closed:
			return nil
		default:
			if _, err := w.Write([]byte(udpKeepAliveMessage)); err != nil {
				return err
			}

			time.Sleep(udpKeepAliveInterval)
		}
	}
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

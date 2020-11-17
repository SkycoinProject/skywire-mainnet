// Package tpdclient implements transport discovery client
package tpdclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/httputil"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/internal/httpauth"
	"github.com/skycoin/skywire/pkg/transport"
)

var log = logging.MustGetLogger("transport-discovery")

// JSONError is the object returned to the client when there's an error.
type JSONError struct {
	Error string `json:"error"`
}

// apiClient implements Client for discovery API.
type apiClient struct {
	client *httpauth.Client
	key    cipher.PubKey
	sec    cipher.SecKey
}

// NewHTTP creates a new client setting a public key to the client to be used for auth.
// When keys are set, the client will sign request before submitting.
// The signature information is transmitted in the header using:
// * SW-Public: The specified public key
// * SW-Nonce:  The nonce for that public key
// * SW-Sig:    The signature of the payload + the nonce
func NewHTTP(ctx context.Context, addr string, key cipher.PubKey, sec cipher.SecKey) (transport.DiscoveryClient, error) {
	client, err := httpauth.NewClient(ctx, addr, key, sec)
	if err != nil {
		return nil, fmt.Errorf("transport discovery httpauth: %w", err)
	}

	return &apiClient{client: client, key: key, sec: sec}, nil
}

// Post performs a POST request.
func (c *apiClient) Post(ctx context.Context, path string, payload interface{}) (*http.Response, error) {
	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(payload); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.client.Addr()+path, body)
	if err != nil {
		return nil, err
	}

	return c.client.Do(req.WithContext(ctx))
}

// Get performs a new GET request.
func (c *apiClient) Get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.client.Addr()+path, new(bytes.Buffer))
	if err != nil {
		return nil, err
	}

	return c.client.Do(req.WithContext(ctx))
}

// Delete performs a new DELETE request.
func (c *apiClient) Delete(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, c.client.Addr()+path, new(bytes.Buffer))
	if err != nil {
		return nil, err
	}

	return c.client.Do(req.WithContext(ctx))
}

// RegisterTransports registers new Transports.
func (c *apiClient) RegisterTransports(ctx context.Context, entries ...*transport.SignedEntry) error {
	if len(entries) == 0 {
		return nil
	}

	resp, err := c.Post(ctx, "/transports/", entries)
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close HTTP response body")
		}
	}()

	return httputil.ErrorFromResp(resp)
}

// GetTransportByID returns Transport for corresponding ID.
func (c *apiClient) GetTransportByID(ctx context.Context, id uuid.UUID) (*transport.EntryWithStatus, error) {
	resp, err := c.Get(ctx, fmt.Sprintf("/transports/id:%s", id.String()))
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close HTTP response body")
		}
	}()

	if err := httputil.ErrorFromResp(resp); err != nil {
		return nil, err
	}

	entry := &transport.EntryWithStatus{}
	if err := json.NewDecoder(resp.Body).Decode(entry); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}

	return entry, nil
}

// GetTransportsByEdge returns all Transports registered for the edge.
func (c *apiClient) GetTransportsByEdge(ctx context.Context, pk cipher.PubKey) ([]*transport.EntryWithStatus, error) {
	resp, err := c.Get(ctx, fmt.Sprintf("/transports/edge:%s", pk))
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close HTTP response body")
		}
	}()

	if err := httputil.ErrorFromResp(resp); err != nil {
		return nil, err
	}

	var entries []*transport.EntryWithStatus
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}

	return entries, nil
}

// DeleteTransport deletes given transport by it's ID. A visor can only delete transports if he is one of it's edges.
func (c *apiClient) DeleteTransport(ctx context.Context, id uuid.UUID) error {
	resp, err := c.Delete(ctx, fmt.Sprintf("/transports/id:%s", id.String()))
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close HTTP response body")
		}
	}()

	return httputil.ErrorFromResp(resp)
}

// UpdateStatuses updates statuses of transports in discovery.
func (c *apiClient) UpdateStatuses(ctx context.Context, statuses ...*transport.Status) ([]*transport.EntryWithStatus, error) {
	if len(statuses) == 0 {
		return nil, nil
	}

	resp, err := c.Post(ctx, "/statuses", statuses)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close HTTP response body")
		}
	}()

	if err := httputil.ErrorFromResp(resp); err != nil {
		return nil, err
	}

	var entries []*transport.EntryWithStatus
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}

	return entries, nil
}

func (c *apiClient) Health(ctx context.Context) (int, error) {
	resp, err := c.Get(ctx, "/health")
	if err != nil {
		return 0, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Failed to close HTTP response body")
		}
	}()

	return resp.StatusCode, nil
}

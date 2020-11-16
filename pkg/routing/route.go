// Package routing defines routing related entities and management
// operations.
package routing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/skycoin/dmsg/cipher"
)

// Route is a succession of transport entries that denotes a path from source visor to destination visor
type Route struct {
	Desc      RouteDescriptor `json:"desc"`
	Hops      []Hop           `json:"path"`
	KeepAlive time.Duration   `json:"keep_alive"`
}

func (r Route) String() string {
	res := fmt.Sprintf("[KeepAlive: %s] %s\n", r.KeepAlive, r.Desc.String())
	for _, hop := range r.Hops {
		res += fmt.Sprintf("\t%s\n", hop)
	}

	return res
}

// Errors associated with BidirectionalRoute/BidirectionalRouteList.
var (
	ErrBiRouteHasNoForwardHops = errors.New("bidirectional route does not have forward hops")
	ErrBiRouteHasNoReverseHops = errors.New("bidirectional route does not have reverse hops")
	ErrBiRouteHasInvalidDesc   = errors.New("bidirectional route has an invalid route description")
)

// BidirectionalRoute is a Route with both forward and reverse Paths.
type BidirectionalRoute struct {
	Desc      RouteDescriptor
	KeepAlive time.Duration
	Forward   []Hop
	Reverse   []Hop
}

// ForwardAndReverse generate forward and reverse routes for bidirectional route.
func (br *BidirectionalRoute) ForwardAndReverse() (forward, reverse Route) {
	forwardRoute := Route{
		Desc:      br.Desc,
		Hops:      br.Forward,
		KeepAlive: br.KeepAlive,
	}

	reverseRoute := Route{
		Desc:      br.Desc.Invert(),
		Hops:      br.Reverse,
		KeepAlive: br.KeepAlive,
	}

	return forwardRoute, reverseRoute
}

// Check checks whether the bidirectional route is valid.
func (br *BidirectionalRoute) Check() error {
	if len(br.Forward) == 0 {
		return ErrBiRouteHasNoForwardHops
	}

	if len(br.Reverse) == 0 {
		return ErrBiRouteHasNoReverseHops
	}

	if srcPK := br.Desc.SrcPK(); br.Forward[0].From != srcPK || br.Reverse[len(br.Reverse)-1].To != srcPK {
		return ErrBiRouteHasInvalidDesc
	}

	if dstPK := br.Desc.DstPK(); br.Reverse[0].From != dstPK || br.Forward[len(br.Forward)-1].To != dstPK {
		return ErrBiRouteHasInvalidDesc
	}

	return nil
}

// String implements fmt.Stringer
func (br *BidirectionalRoute) String() string {
	m := map[string]interface{}{
		"descriptor": br.Desc.String(),
		"keep_alive": br.KeepAlive.String(),
		"fwd_hops":   br.Forward,
		"rev_hops":   br.Reverse,
	}

	j, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		panic(err) // should never happen
	}

	return string(j)
}

// BidirectionalRouteList is a list of Route's with both forward and reverse Paths.
type BidirectionalRouteList struct {
	Desc      RouteDescriptor
	KeepAlive time.Duration
	Forward   [][]Hop
	Reverse   [][]Hop
}

// ForwardAndReverse generate forward and reverse routes for bidirectional route.
func (br *BidirectionalRouteList) ForwardAndReverse() (forward, reverse []Route) {
	forwardRoutes := make([]Route, 0)
	reverseRoutes := make([]Route, 0)

	for _, route := range br.Forward {
		forwardRoutes = append(forwardRoutes, Route{
			Desc:      br.Desc,
			Hops:      route,
			KeepAlive: br.KeepAlive,
		})
	}

	for _, route := range br.Reverse {
		reverseRoutes = append(reverseRoutes, Route{
			Desc:      br.Desc.Invert(),
			Hops:      route,
			KeepAlive: br.KeepAlive,
		})
	}

	return forwardRoutes, reverseRoutes
}

// Check checks whether the bidirectional route is valid.
func (br *BidirectionalRouteList) Check() error {
	if len(br.Forward) == 0 {
		return ErrBiRouteHasNoForwardHops
	}

	if len(br.Reverse) == 0 {
		return ErrBiRouteHasNoReverseHops
	}

	for i := range br.Forward {
		if srcPK := br.Desc.SrcPK(); br.Forward[i][0].From != srcPK || br.Reverse[i][len(br.Reverse[i])-1].To != srcPK {
			return ErrBiRouteHasInvalidDesc
		}

		if dstPK := br.Desc.DstPK(); br.Reverse[i][0].From != dstPK || br.Forward[i][len(br.Forward[i])-1].To != dstPK {
			return ErrBiRouteHasInvalidDesc
		}
	}

	return nil
}

// String implements fmt.Stringer
func (br *BidirectionalRouteList) String() string {
	m := map[string]interface{}{
		"descriptor": br.Desc.String(),
		"keep_alive": br.KeepAlive.String(),
		"fwd_hops":   br.Forward,
		"rev_hops":   br.Reverse,
	}

	j, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		panic(err) // should never happen
	}

	return string(j)
}

// EdgeRules represents edge forward and reverse rules. Edge rules are forward and consume rules.
type EdgeRules struct {
	Desc    RouteDescriptor
	Forward Rule
	Reverse Rule
}

// String implements fmt.Stringer
func (er EdgeRules) String() string {
	m := map[string]interface{}{
		"descriptor": er.Desc.String(),
		"routing_rules": []string{
			er.Forward.String(),
			er.Reverse.String(),
		},
	}

	j, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		panic(err)
	}

	return string(j)
}

// EdgeRulesList represents a list of edge forward and reverse rules. Edge rules are forward and consume rules.
type EdgeRulesList struct {
	Desc    RouteDescriptor
	Forward []Rule
	Reverse []Rule
}

// String implements fmt.Stringer
func (er EdgeRulesList) String() string {
	m := map[string]interface{}{
		"descriptor":    er.Desc.String(),
		"forward_rules": er.Forward,
		"reverse_rules": er.Reverse,
	}

	j, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		panic(err)
	}

	return string(j)
}

// Hop defines a route hop between 2 nodes.
type Hop struct {
	TpID uuid.UUID
	From cipher.PubKey
	To   cipher.PubKey
}

// String implements fmt.Stringer
func (h Hop) String() string {
	return fmt.Sprintf("%s -> %s @ %s", h.From, h.To, h.TpID)
}

// PathEdges are the edge nodes of a path
type PathEdges [2]cipher.PubKey

// MarshalText implements encoding.TextMarshaler
func (p PathEdges) MarshalText() ([]byte, error) {
	b1, err := p[0].MarshalText()
	if err != nil {
		return nil, err
	}

	b2, err := p[1].MarshalText()
	if err != nil {
		return nil, err
	}

	res := bytes.NewBuffer(b1)
	res.WriteString(":") // nolint
	res.Write(b2)        // nolint

	return res.Bytes(), nil
}

// UnmarshalText implements json.Unmarshaler
func (p *PathEdges) UnmarshalText(b []byte) error {
	err := p[0].UnmarshalText(b[:66])
	if err != nil {
		return err
	}

	err = p[1].UnmarshalText(b[67:])
	if err != nil {
		return err
	}

	return nil
}

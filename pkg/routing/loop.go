package routing

import (
	"fmt"
	"time"

	"github.com/skycoin/dmsg/cipher"
)

// Loop defines a loop over a pair of routes.
type Loop struct {
	Local   Addr
	Remote  Addr
	Forward Route
	Reverse Route
	Expiry  time.Time
}

// Initiator returns initiator of the Loop.
func (l *Loop) Initiator() cipher.PubKey {
	if len(l.Forward) == 0 {
		panic("empty forward route")
	}

	return l.Forward[0].From
}

// Responder returns responder of the Loop.
func (l *Loop) Responder() cipher.PubKey {
	if len(l.Reverse) == 0 {
		panic("empty reverse route")
	}

	return l.Reverse[0].From
}

func (l *Loop) String() string {
	return fmt.Sprintf("lport: %d. rport: %d. routes: %s/%s. expire at %s",
		l.Local.Port, l.Remote.Port, l.Forward, l.Reverse, l.Expiry)
}

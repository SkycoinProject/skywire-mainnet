module github.com/SkycoinProject/skywire-mainnet

go 1.12

require (
	github.com/SkycoinProject/dmsg v0.0.0-20191015061558-709ec3a1f4f5
	github.com/SkycoinProject/skycoin v0.26.0
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20190910110746-680d30ca3117 // indirect
	github.com/armon/go-socks5 v0.0.0-20160902184237-e75332964ef5
	github.com/creack/pty v1.1.7
	github.com/go-chi/chi v4.0.2+incompatible
	github.com/google/uuid v1.1.1
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/securecookie v1.1.1
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d
	github.com/kr/pty v1.1.8 // indirect
	github.com/mattn/go-isatty v0.0.9 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/profile v1.3.0
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/common v0.6.0
	github.com/sirupsen/logrus v1.4.2
	github.com/skycoin/dmsg v0.0.0-20190805065636-70f4c32a994f // indirect
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.4.0
	go.etcd.io/bbolt v1.3.3
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
	golang.org/x/net v0.0.0-20191014212845-da9a3fd4c582
)

// Uncomment for tests with alternate branches of 'dmsg'
//replace github.com/SkycoinProject/dmsg => ../dmsg

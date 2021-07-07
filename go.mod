module github.com/skycoin/skywire

go 1.16

require (
	github.com/AudriusButkevicius/pfilter v0.0.0-20210515103320-4b4b86609d51
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/VictoriaMetrics/metrics v1.17.2
	github.com/armon/go-socks5 v0.0.0-20160902184237-e75332964ef5
	github.com/creack/pty v1.1.11 // indirect
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/uuid v1.1.1
	github.com/gorilla/securecookie v1.1.1
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/klauspost/reedsolomon v1.9.9 // indirect
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mholt/archiver/v3 v3.3.0
	github.com/mmcloughlin/avo v0.0.0-20200523190732-4439b6b2c061 // indirect
	github.com/pkg/profile v1.5.0
	github.com/schollz/progressbar/v2 v2.15.0
	github.com/shirou/gopsutil v2.20.5+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/skycoin/dmsg v0.0.0-20210616115957-702a819a9b3d
	github.com/skycoin/skycoin v0.27.1
	github.com/skycoin/yamux v0.0.0-20200803175205-571ceb89da9f
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	github.com/spf13/cobra v1.0.0
	github.com/stretchr/testify v1.6.1
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/templexxx/cpufeat v0.0.0-20180724012125-cef66df7f161 // indirect
	github.com/templexxx/xor v0.0.0-20191217153810-f85b25db303b // indirect
	github.com/tjfoc/gmsm v1.3.2 // indirect
	github.com/toqueteos/webbrowser v1.2.0
	github.com/xtaci/kcp-go v5.4.20+incompatible
	github.com/xtaci/lossyconn v0.0.0-20200209145036-adba10fffc37 // indirect
	go.etcd.io/bbolt v1.3.5
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a // indirect
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4
	golang.org/x/sys v0.0.0-20210608053332-aa57babbf139
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56 // indirect
	golang.org/x/tools v0.1.2 // indirect
	golang.zx2c4.com/wireguard v0.0.20200320
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c
	nhooyr.io/websocket v1.8.2
)

// Uncomment for tests with alternate branches of 'dmsg'
//replace github.com/skycoin/dmsg => ../dmsg

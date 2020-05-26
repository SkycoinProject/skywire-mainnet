package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/sirupsen/logrus"

	"github.com/SkycoinProject/skywire-mainnet/internal/vpn"
	"github.com/SkycoinProject/skywire-mainnet/pkg/app"
	"github.com/SkycoinProject/skywire-mainnet/pkg/app/appnet"
	"github.com/SkycoinProject/skywire-mainnet/pkg/routing"
	"github.com/SkycoinProject/skywire-mainnet/pkg/skyenv"
)

const (
	netType = appnet.TypeSkynet
	vpnPort = routing.Port(skyenv.VPNServerPort)
)

var (
	log = logrus.New()
)

var (
	localPKStr = flag.String("pk", "", "Local PubKey")
	localSKStr = flag.String("sk", "", "Local SecKey")
	passcode   = flag.String("passcode", "", "Passcode to authenticate connecting users")
)

func main() {
	flag.Parse()

	localPK := cipher.PubKey{}
	if *localPKStr != "" {
		if err := localPK.UnmarshalText([]byte(*localPKStr)); err != nil {
			log.WithError(err).Fatalln("Invalid local PK")
		}
	}

	localSK := cipher.SecKey{}
	if *localSKStr != "" {
		if err := localSK.UnmarshalText([]byte(*localSKStr)); err != nil {
			log.WithError(err).Fatalln("Invalid local SK")
		}
	}

	var noiseCreds vpn.NoiseCredentials
	if localPK.Null() && !localSK.Null() {
		var err error
		noiseCreds, err = vpn.NewNoiseCredentialsFromSK(localSK)
		if err != nil {
			log.WithError(err).Fatalln("error creating noise credentials")
		}
	} else {
		noiseCreds = vpn.NewNoiseCredentials(localSK, localPK)
	}

	appClient := app.NewClient(nil)
	defer appClient.Close()

	osSigs := make(chan os.Signal, 2)

	sigs := []os.Signal{syscall.SIGTERM, syscall.SIGINT}
	for _, sig := range sigs {
		signal.Notify(osSigs, sig)
	}

	l, err := appClient.Listen(netType, vpnPort)
	if err != nil {
		log.WithError(err).Errorf("Error listening network %v on port %d", netType, vpnPort)
		return
	}

	log.Infof("Got app listener, bound to %d", vpnPort)

	srvCfg := vpn.ServerConfig{
		Passcode:    *passcode,
		Credentials: noiseCreds,
	}
	srv, err := vpn.NewServer(srvCfg, log)
	if err != nil {
		log.WithError(err).Fatalln("Error creating VPN server")
	}
	defer func() {
		if err := srv.Close(); err != nil {
			log.WithError(err).Errorln("Error closing server")
		}
	}()
	go func() {
		if err := srv.Serve(l); err != nil {
			log.WithError(err).Errorln("Error serving")
		}
	}()

	<-osSigs
}

package visor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SkycoinProject/skywire-mainnet/pkg/util/pathutil"

	"github.com/SkycoinProject/skywire-mainnet/pkg/app/appcommon"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/SkycoinProject/skywire-mainnet/internal/testhelpers"

	"github.com/SkycoinProject/skywire-mainnet/pkg/app/appserver"

	"github.com/SkycoinProject/skywire-mainnet/pkg/router"

	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/skywire-mainnet/pkg/snet"
	"github.com/SkycoinProject/skywire-mainnet/pkg/transport"
	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/skycoin/src/util/logging"
)

var masterLogger *logging.MasterLogger

func TestMain(m *testing.M) {
	masterLogger = logging.NewMasterLogger()
	loggingLevel, ok := os.LookupEnv("TEST_LOGGING_LEVEL")
	if ok {
		lvl, err := logging.LevelFromString(loggingLevel)
		if err != nil {
			log.Fatal(err)
		}
		masterLogger.SetLevel(lvl)
	} else {
		masterLogger.Out = ioutil.Discard
	}

	os.Exit(m.Run())
}

// TODO(nkryuchkov): fix and uncomment
//func TestNewNode(t *testing.T) {
//	pk, sk := cipher.GenerateKeyPair()
//	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		require.NoError(t, json.NewEncoder(w).Encode(&httpauth.NextNonceResponse{Edge: pk, NextNonce: 1}))
//	}))
//	defer srv.Close()
//
//	conf := Config{Version: "1.0", LocalPath: "local", AppsPath: "apps"}
//	conf.Node.StaticPubKey = pk
//	conf.Node.StaticSecKey = sk
//	conf.Dmsg.Discovery = "http://skywire.skycoin.com:8001"
//	conf.Dmsg.ServerCount = 10
//	conf.Transport.Discovery = srv.URL
//	conf.Apps = []AppConfig{
//		{App: "foo", Version: "1.1", Port: 1},
//		{App: "bar", AutoStart: true, Port: 2},
//	}
//
//	defer func() {
//		require.NoError(t, os.RemoveAll("local"))
//	}()
//
//	node, err := NewNode(&conf, masterLogger)
//	require.NoError(t, err)
//
//	assert.NotNil(t, node.router)
//	assert.NotNil(t, node.appsConf)
//	assert.NotNil(t, node.appsPath)
//	assert.NotNil(t, node.localPath)
//	assert.NotNil(t, node.startedApps)
//}

func TestNodeStartClose(t *testing.T) {
	r := &router.MockRouter{}
	r.On("Serve", mock.Anything /* context */).Return(testhelpers.NoErr)
	r.On("Close").Return(testhelpers.NoErr)

	apps := make(map[string]AppConfig)
	appCfg := []AppConfig{
		{
			App:       "skychat",
			Version:   "1.0",
			AutoStart: true,
			Port:      1,
		},
		{
			App:       "foo",
			Version:   "1.0",
			AutoStart: false,
		},
	}

	for _, app := range appCfg {
		apps[app.App] = app
	}

	defer func() {
		require.NoError(t, os.RemoveAll("skychat"))
	}()

	nodeCfg := Config{}

	node := &Node{
		conf:     &nodeCfg,
		router:   r,
		appsConf: apps,
		logger:   logging.MustGetLogger("test"),
	}

	pm := &appserver.MockProcManager{}
	appCfg1 := appcommon.Config{
		Name:         apps["skychat"].App,
		Version:      apps["skychat"].Version,
		SockFilePath: nodeCfg.AppServerSockFile,
		VisorPK:      nodeCfg.Node.StaticPubKey.Hex(),
		WorkDir:      filepath.Join("", apps["skychat"].App, fmt.Sprintf("v%s", apps["skychat"].Version)),
	}
	appArgs1 := append([]string{filepath.Join(node.dir(), apps["skychat"].App)}, apps["skychat"].Args...)
	appPID1 := appcommon.ProcID(10)
	pm.On("Run", mock.Anything, appCfg1, appArgs1, mock.Anything, mock.Anything).
		Return(appPID1, testhelpers.NoErr)
	pm.On("Wait", apps["skychat"].App).Return(testhelpers.NoErr)

	pm.On("StopAll").Return()

	node.procManager = pm

	dmsgC := dmsg.NewClient(cipher.PubKey{}, cipher.SecKey{}, disc.NewMock(), nil)
	go dmsgC.Serve()

	netConf := snet.Config{
		PubKey:          cipher.PubKey{},
		SecKey:          cipher.SecKey{},
		TpNetworks:      nil,
		DmsgDiscAddr:    "",
		DmsgMinSessions: 0,
	}

	network := snet.NewRaw(netConf, dmsgC, nil)
	tmConf := &transport.ManagerConfig{
		PubKey:          cipher.PubKey{},
		DiscoveryClient: transport.NewDiscoveryMock(),
	}

	tm, err := transport.NewManager(network, tmConf)
	node.tm = tm
	require.NoError(t, err)

	errCh := make(chan error)
	go func() {
		errCh <- node.Start()
	}()

	require.NoError(t, <-errCh)
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, node.Close())
}

func TestNodeSpawnApp(t *testing.T) {
	pk, _ := cipher.GenerateKeyPair()
	r := &router.MockRouter{}
	r.On("Serve", mock.Anything /* context */).Return(testhelpers.NoErr)
	r.On("Close").Return(testhelpers.NoErr)

	defer func() {
		require.NoError(t, os.RemoveAll("skychat"))
	}()

	app := AppConfig{
		App:       "skychat",
		Version:   "1.0",
		AutoStart: false,
		Port:      10,
		Args:      []string{"foo"},
	}

	apps := make(map[string]AppConfig)
	apps["skychat"] = app

	nodeCfg := Config{}
	nodeCfg.Node.StaticPubKey = pk

	node := &Node{
		router:   r,
		appsConf: apps,
		logger:   logging.MustGetLogger("test"),
		conf:     &nodeCfg,
	}
	pathutil.EnsureDir(node.dir())
	defer func() {
		require.NoError(t, os.RemoveAll(node.dir()))
	}()

	pm := &appserver.MockProcManager{}
	appCfg := appcommon.Config{
		Name:         app.App,
		Version:      app.Version,
		SockFilePath: nodeCfg.AppServerSockFile,
		VisorPK:      nodeCfg.Node.StaticPubKey.Hex(),
		WorkDir:      filepath.Join("", app.App, fmt.Sprintf("v%s", app.Version)),
	}
	appArgs := append([]string{filepath.Join(node.dir(), app.App)}, app.Args...)
	pm.On("Wait", app.App).Return(testhelpers.NoErr)

	appPID := appcommon.ProcID(10)
	pm.On("Run", mock.Anything, appCfg, appArgs, mock.Anything, mock.Anything).
		Return(appPID, testhelpers.NoErr)
	pm.On("Exists", app.App).Return(true)
	pm.On("Stop", app.App).Return(testhelpers.NoErr)

	node.procManager = pm

	require.NoError(t, node.StartApp(app.App))
	time.Sleep(100 * time.Millisecond)

	require.True(t, node.procManager.Exists(app.App))

	require.NoError(t, node.StopApp(app.App))
}

func TestNodeSpawnAppValidations(t *testing.T) {
	pk, _ := cipher.GenerateKeyPair()
	r := &router.MockRouter{}
	r.On("Serve", mock.Anything /* context */).Return(testhelpers.NoErr)
	r.On("Close").Return(testhelpers.NoErr)

	defer func() {
		require.NoError(t, os.RemoveAll("skychat"))
	}()

	c := &Config{}
	c.Node.StaticPubKey = pk

	node := &Node{
		router: r,
		logger: logging.MustGetLogger("test"),
		conf:   c,
	}
	pathutil.EnsureDir(node.dir())
	defer func() {
		require.NoError(t, os.RemoveAll(node.dir()))
	}()

	t.Run("fail - can't bind to reserved port", func(t *testing.T) {
		app := AppConfig{
			App:     "skychat",
			Version: "1.0",
			Port:    3,
		}
		wantErr := "can't bind to reserved port 3"

		pm := &appserver.MockProcManager{}
		appCfg := appcommon.Config{
			Name:         app.App,
			Version:      app.Version,
			SockFilePath: c.AppServerSockFile,
			VisorPK:      c.Node.StaticPubKey.Hex(),
			WorkDir:      filepath.Join("", app.App, fmt.Sprintf("v%s", app.Version)),
		}
		appArgs := append([]string{filepath.Join(node.dir(), app.App)}, app.Args...)

		appPID := appcommon.ProcID(10)
		pm.On("Run", mock.Anything, appCfg, appArgs, mock.Anything, mock.Anything).
			Return(appPID, testhelpers.NoErr)
		pm.On("Exists", app.App).Return(false)

		node.procManager = pm

		errCh := make(chan error)
		go func() {
			errCh <- node.SpawnApp(&app, nil)
		}()

		time.Sleep(100 * time.Millisecond)
		err := <-errCh
		require.Error(t, err)
		assert.Equal(t, wantErr, err.Error())
	})

	t.Run("fail - app already started", func(t *testing.T) {
		app := AppConfig{
			App:     "skychat",
			Version: "1.0",
			Port:    10,
		}
		wantErr := fmt.Sprintf("error running app skychat: %s", appserver.ErrAppAlreadyStarted)

		pm := &appserver.MockProcManager{}
		appCfg := appcommon.Config{
			Name:         app.App,
			Version:      app.Version,
			SockFilePath: c.AppServerSockFile,
			VisorPK:      c.Node.StaticPubKey.Hex(),
			WorkDir:      filepath.Join("", app.App, fmt.Sprintf("v%s", app.Version)),
		}
		appArgs := append([]string{filepath.Join(node.dir(), app.App)}, app.Args...)

		appPID := appcommon.ProcID(10)
		pm.On("Run", mock.Anything, appCfg, appArgs, mock.Anything, mock.Anything).
			Return(appPID, appserver.ErrAppAlreadyStarted)
		pm.On("Exists", app.App).Return(true)

		node.procManager = pm

		errCh := make(chan error)
		go func() {
			errCh <- node.SpawnApp(&app, nil)
		}()

		time.Sleep(100 * time.Millisecond)
		err := <-errCh
		require.Error(t, err)
		assert.Equal(t, wantErr, err.Error())
	})
}

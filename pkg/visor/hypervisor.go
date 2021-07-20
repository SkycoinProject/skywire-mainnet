package visor

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/httputil"
	"github.com/skycoin/skycoin/src/util/logging"
	"nhooyr.io/websocket"

	"github.com/skycoin/skywire/pkg/app/appcommon"
	"github.com/skycoin/skywire/pkg/app/launcher"
	"github.com/skycoin/skywire/pkg/routing"
	"github.com/skycoin/skywire/pkg/skyenv"
	"github.com/skycoin/skywire/pkg/util/updater"
	"github.com/skycoin/skywire/pkg/visor/dmsgtracker"
	"github.com/skycoin/skywire/pkg/visor/hypervisorconfig"
	"github.com/skycoin/skywire/pkg/visor/usermanager"
)

const (
	httpTimeout = 30 * time.Second
)

const (
	statusStop = iota
	statusStart
)

var (
	log = logging.MustGetLogger("hypervisor") // nolint: gochecknoglobals
)

// Conn represents a visor connection.
type Conn struct {
	Addr  dmsg.Addr
	SrvPK cipher.PubKey
	API   API
	PtyUI *dmsgPtyUI
}

// Hypervisor manages visors.
type Hypervisor struct {
	c            hypervisorconfig.Config
	visor        *Visor
	dmsgC        *dmsg.Client
	visors       map[cipher.PubKey]Conn // connected remote visors
	trackers     *dmsgtracker.Manager   // dmsg trackers
	users        *usermanager.UserManager
	mu           *sync.RWMutex
	visorMu      sync.Mutex
	visorChanMux map[cipher.PubKey]*chanMux
	selfConn     Conn
}

// New creates a new Hypervisor.
func New(config hypervisorconfig.Config, visor *Visor, dmsgC *dmsg.Client) (*Hypervisor, error) {
	config.Cookies.TLS = config.EnableTLS

	boltUserDB, err := usermanager.NewBoltUserStore(config.DBPath)
	if err != nil {
		return nil, err
	}

	singleUserDB := usermanager.NewSingleUserStore("admin", boltUserDB)

	selfConn := Conn{
		Addr:  dmsg.Addr{PK: config.PK, Port: config.DmsgPort},
		API:   visor,
		PtyUI: nil,
	}

	hv := &Hypervisor{
		c:            config,
		visor:        visor,
		dmsgC:        dmsgC,
		visors:       make(map[cipher.PubKey]Conn),
		trackers:     dmsgtracker.NewDmsgTrackerManager(nil, dmsgC, 0, 0),
		users:        usermanager.NewUserManager(singleUserDB, config.Cookies),
		mu:           new(sync.RWMutex),
		visorChanMux: make(map[cipher.PubKey]*chanMux),
		selfConn:     selfConn,
	}

	return hv, nil
}

// ServeRPC serves RPC of a Hypervisor.
func (hv *Hypervisor) ServeRPC(ctx context.Context, dmsgPort uint16) error {
	lis, err := hv.dmsgC.Listen(dmsgPort)
	if err != nil {
		return err
	}

	if hv.visor != nil {
		// Track hypervisor node.
		if _, err := hv.trackers.MustGet(ctx, hv.visor.conf.PK); err != nil {
			log.WithField("addr", hv.c.DmsgDiscovery).WithError(err).Warn("Failed to dial tracker stream.")
		}
	}

	// setup
	hv.mu.Lock()
	hv.selfConn.PtyUI = setupDmsgPtyUI(hv.dmsgC, hv.c.PK)
	hv.mu.Unlock()

	for {
		conn, err := lis.AcceptStream()
		if err != nil {
			return err
		}

		addr := conn.RawRemoteAddr()
		log := logging.MustGetLogger(fmt.Sprintf("rpc_client:%s", addr.PK))

		visorConn := &Conn{
			Addr:  addr,
			SrvPK: conn.ServerPK(),
			API:   NewRPCClient(log, conn, RPCPrefix, skyenv.DefaultRPCTimeout),
			PtyUI: setupDmsgPtyUI(hv.dmsgC, addr.PK),
		}

		if _, err := hv.trackers.MustGet(ctx, addr.PK); err != nil {
			log.WithField("addr", hv.c.DmsgDiscovery).WithError(err).Warn("Failed to dial tracker stream.")
		}

		log.Info("Accepted.")

		hv.mu.Lock()
		hv.visors[addr.PK] = *visorConn
		hv.mu.Unlock()
	}
}

// MockConfig configures how mock data is to be added.
type MockConfig struct {
	Visors            int
	MaxTpsPerVisor    int
	MaxRoutesPerVisor int
	EnableAuth        bool
}

type elementResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// AddMockData adds mock data to Hypervisor.
func (hv *Hypervisor) AddMockData(config MockConfig) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // nolint:gosec

	for i := 0; i < config.Visors; i++ {
		pk, client, err := NewMockRPCClient(r, config.MaxTpsPerVisor, config.MaxRoutesPerVisor)
		if err != nil {
			return err
		}

		hv.mu.Lock()
		hv.visors[pk] = Conn{
			Addr: dmsg.Addr{
				PK:   pk,
				Port: uint16(i),
			},
			API: client,
		}
		hv.mu.Unlock()
	}

	hv.c.EnableAuth = config.EnableAuth

	return nil
}

// HTTPHandler returns a http handler.
func (hv *Hypervisor) HTTPHandler() http.Handler {
	return hv.makeMux()
}

func (hv *Hypervisor) makeMux() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(httputil.SetLoggerMiddleware(log))

	r.Route("/", func(r chi.Router) {
		r.Route("/api", func(r chi.Router) {
			r.Use(middleware.Timeout(httpTimeout))

			r.Get("/ping", hv.getPong())

			if hv.c.EnableAuth {
				r.Group(func(r chi.Router) {
					r.Post("/create-account", hv.users.CreateAccount())
					r.Post("/login", hv.users.Login())
					r.Post("/logout", hv.users.Logout())
				})
			}

			r.Group(func(r chi.Router) {
				if hv.c.EnableAuth {
					r.Use(hv.users.Authorize)
				}

				r.Get("/user", hv.users.UserInfo())
				r.Post("/change-password", hv.users.ChangePassword())
				r.Get("/about", hv.getAbout())
				r.Get("/dmsg", hv.getDmsg())

				r.Get("/visors", hv.getVisors())
				r.Get("/visors-summary", hv.getAllVisorsSummary())
				r.Get("/visors/{pk}", hv.getVisor())
				r.Get("/visors/{pk}/summary", hv.getVisorSummary())
				r.Get("/visors/{pk}/health", hv.getHealth())
				r.Get("/visors/{pk}/uptime", hv.getUptime())
				r.Get("/visors/{pk}/apps", hv.getApps())
				r.Get("/visors/{pk}/apps/{app}", hv.getApp())
				r.Put("/visors/{pk}/apps/{app}", hv.putApp())
				r.Get("/visors/{pk}/apps/{app}/logs", hv.appLogsSince())
				r.Get("/visors/{pk}/apps/{app}/stats", hv.getAppStats())
				r.Get("/visors/{pk}/apps/{app}/connections", hv.appConnections())
				r.Get("/visors/{pk}/transport-types", hv.getTransportTypes())
				r.Get("/visors/{pk}/transports", hv.getTransports())
				r.Post("/visors/{pk}/transports", hv.postTransport())
				r.Get("/visors/{pk}/transports/{tid}", hv.getTransport())
				r.Delete("/visors/{pk}/transports/{tid}", hv.deleteTransport())
				r.Delete("/visors/{pk}/transports/", hv.deleteTransports())
				r.Get("/visors/{pk}/routes", hv.getRoutes())
				r.Post("/visors/{pk}/routes", hv.postRoute())
				r.Get("/visors/{pk}/routes/{rid}", hv.getRoute())
				r.Put("/visors/{pk}/routes/{rid}", hv.putRoute())
				r.Delete("/visors/{pk}/routes/{rid}", hv.deleteRoute())
				r.Delete("/visors/{pk}/routes/", hv.deleteRoutes())
				r.Get("/visors/{pk}/routegroups", hv.getRouteGroups())
				r.Post("/visors/{pk}/restart", hv.restart())
				r.Post("/visors/{pk}/exec", hv.exec())
				r.Post("/visors/{pk}/update", hv.updateVisor())
				r.Get("/visors/{pk}/update/ws", hv.updateVisorWS())
				r.Get("/visors/{pk}/update/ws/running", hv.isVisorWSUpdateRunning())
				r.Get("/visors/{pk}/update/available", hv.visorUpdateAvailable())
				r.Get("/visors/{pk}/update/available/{channel}", hv.visorUpdateAvailable())
				r.Get("/visors/{pk}/runtime-logs", hv.getRuntimeLogs())
				r.Post("/visors/{pk}/min-hops", hv.postMinHops())
			})
		})

		// we don't enable `dmsgpty` endpoints for Windows
		if runtime.GOOS != "windows" {
			r.Route("/pty", func(r chi.Router) {
				if hv.c.EnableAuth {
					r.Use(hv.users.Authorize)
				}

				r.Get("/{pk}", hv.getPty())
			})
		}

		r.Handle("/*", http.FileServer(http.FS(hv.c.UIAssets)))
	})

	return r
}

func (hv *Hypervisor) log(r *http.Request) logrus.FieldLogger {
	return httputil.GetLogger(r)
}

func (hv *Hypervisor) getPong() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`"PONG!"`)); err != nil {
			hv.log(r).WithError(err).Warn("getPong: Failed to send PONG!")
		}
	}
}

// About provides info about the hypervisor.
type About struct {
	PubKey cipher.PubKey   `json:"public_key"` // The hypervisor's public key.
	Build  *buildinfo.Info `json:"build"`
}

func (hv *Hypervisor) getAbout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httputil.WriteJSON(w, r, http.StatusOK, About{
			PubKey: hv.c.PK,
			Build:  buildinfo.Get(),
		})
	}
}

func (hv *Hypervisor) getDmsg() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := hv.getDmsgSummary()
		httputil.WriteJSON(w, r, http.StatusOK, out)
	}
}

func (hv *Hypervisor) getDmsgSummary() []dmsgtracker.DmsgClientSummary {
	hv.mu.RLock()
	defer hv.mu.RUnlock()

	pks := make([]cipher.PubKey, 0, len(hv.visors)+1)
	if hv.visor != nil {
		// Add hypervisor node.
		pks = append(pks, hv.visor.conf.PK)
	}

	for pk := range hv.visors {
		pks = append(pks, pk)
	}

	return hv.trackers.GetBulk(pks)
}

// Health represents a visor's health report attached to hypervisor to visor request status
type Health struct {
	Status int `json:"status"`
	*HealthInfo
}

// provides summary of health information for every visor
func (hv *Hypervisor) getHealth() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		vh := &Health{}

		type healthRes struct {
			h   *HealthInfo
			err error
		}

		resCh := make(chan healthRes)
		tCh := time.After(HealthTimeout)

		go func() {
			hi, err := ctx.API.Health()
			resCh <- healthRes{hi, err}
		}()

		select {
		case res := <-resCh:
			if res.err != nil {
				vh.Status = http.StatusInternalServerError
			} else {
				vh.HealthInfo = res.h
				vh.Status = http.StatusOK
			}

			httputil.WriteJSON(w, r, http.StatusOK, vh)
		case <-tCh:
			httputil.WriteJSON(w, r, http.StatusRequestTimeout, &Health{Status: http.StatusRequestTimeout})
		}
	})
}

// getUptime gets given visor's uptime
func (hv *Hypervisor) getUptime() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		u, err := ctx.API.Uptime()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, u)
	})
}

// provides overview of all visors.
func (hv *Hypervisor) getVisors() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hv.mu.RLock()
		wg := new(sync.WaitGroup)
		wg.Add(len(hv.visors))

		i := 0
		if hv.visor != nil {
			i++
		}

		overviews := make([]Overview, len(hv.visors)+i)

		if hv.visor != nil {
			overview, err := hv.visor.Overview()
			if err != nil {
				log.WithError(err).Warn("Failed to obtain overview of this visor.")
				overview = &Overview{PubKey: hv.visor.conf.PK}
			}

			overviews[0] = *overview
		}

		for pk, c := range hv.visors {
			go func(pk cipher.PubKey, c Conn, i int) {
				log := hv.log(r).
					WithField("visor_addr", c.Addr).
					WithField("func", "getVisors")

				log.Debug("Requesting overview via RPC.")

				overview, err := c.API.Overview()
				if err != nil {
					log.WithError(err).
						Warn("Failed to obtain overview via RPC.")
					overview = &Overview{PubKey: pk}
				} else {
					log.Debug("Obtained overview via RPC.")
				}
				overviews[i] = *overview
				wg.Done()
			}(pk, c, i)
			i++
		}

		wg.Wait()
		hv.mu.RUnlock()

		httputil.WriteJSON(w, r, http.StatusOK, overviews)
	}
}

// provides overview of single visor.
func (hv *Hypervisor) getVisor() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		overview, err := ctx.API.Overview()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, overview)
	})
}

// provides extra summary of single visor.
func (hv *Hypervisor) getVisorSummary() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		summary, err := ctx.API.Summary()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		dmsgStats := make(map[string]dmsgtracker.DmsgClientSummary)
		dSummary := hv.getDmsgSummary()
		for _, stat := range dSummary {
			dmsgStats[stat.PK.String()] = stat
		}

		if stat, ok := dmsgStats[summary.Overview.PubKey.String()]; ok {
			summary.DmsgStats = &stat
		} else {
			summary.DmsgStats = &dmsgtracker.DmsgClientSummary{}
		}
		httputil.WriteJSON(w, r, http.StatusOK, summary)
	})
}

func makeSummaryResp(online, hyper bool, sum *Summary) Summary {
	sum.Online = online
	sum.IsHypervisor = hyper
	return *sum
}

func (hv *Hypervisor) getAllVisorsSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hv.mu.RLock()
		wg := new(sync.WaitGroup)
		wg.Add(len(hv.visors))

		i := 1

		dmsgStats := make(map[string]dmsgtracker.DmsgClientSummary)
		wg.Add(1)
		go func() {
			summary := hv.getDmsgSummary()
			for _, stat := range summary {
				dmsgStats[stat.PK.String()] = stat
			}
			wg.Done()
		}()

		summaries := make([]Summary, len(hv.visors)+i)

		summary, err := hv.visor.Summary()
		if err != nil {
			log.WithError(err).Warn("Failed to obtain summary of this visor.")
			summary = &Summary{
				Overview: &Overview{
					PubKey: hv.visor.conf.PK,
				},
				Health: &HealthInfo{},
			}
		}

		summaries[0] = makeSummaryResp(err == nil, true, summary)

		for pk, c := range hv.visors {
			go func(pk cipher.PubKey, c Conn, i int) {
				log := hv.log(r).
					WithField("visor_addr", c.Addr).
					WithField("func", "getVisors")

				log.Debug("Requesting summary via RPC.")

				summary, err := c.API.Summary()
				if err != nil {
					log.WithError(err).
						Warn("Failed to obtain summary via RPC.", pk)

					summary = &Summary{
						Overview: &Overview{
							PubKey: pk,
						},
						Health: &HealthInfo{},
					}
				} else {
					log.Debug("Obtained summary via RPC.")
				}
				resp := makeSummaryResp(err == nil, false, summary)
				summaries[i] = resp
				wg.Done()
			}(pk, c, i)
			i++
		}

		wg.Wait()
		for i := 0; i < len(summaries); i++ {
			if stat, ok := dmsgStats[summaries[i].Overview.PubKey.String()]; ok {
				summaries[i].DmsgStats = &stat
			} else {
				summaries[i].DmsgStats = &dmsgtracker.DmsgClientSummary{}
			}
		}

		hv.mu.RUnlock()

		httputil.WriteJSON(w, r, http.StatusOK, summaries)
	}
}

// returns app summaries of a given node of pk
func (hv *Hypervisor) getApps() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		apps, err := ctx.API.Apps()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, apps)
	})
}

// returns an app summary of a given visor's pk and app name
func (hv *Hypervisor) getApp() http.HandlerFunc {
	return hv.withCtx(hv.appCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		httputil.WriteJSON(w, r, http.StatusOK, ctx.App)
	})
}

func (hv *Hypervisor) getAppStats() http.HandlerFunc {
	return hv.withCtx(hv.appCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		stats, err := ctx.API.GetAppStats(ctx.App.Name)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, &stats)
	})
}

// TODO: simplify
// nolint: funlen,gocognit,godox
func (hv *Hypervisor) putApp() http.HandlerFunc {
	return hv.withCtx(hv.appCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		type req struct {
			AutoStart  *bool          `json:"autostart,omitempty"`
			Killswitch *bool          `json:"killswitch,omitempty"`
			Secure     *bool          `json:"secure,omitempty"`
			Status     *int           `json:"status,omitempty"`
			Passcode   *string        `json:"passcode,omitempty"`
			PK         *cipher.PubKey `json:"pk,omitempty"`
		}

		shouldRestartApp := func(r req) bool {
			// we restart the app if one of these fields was changed
			return r.Killswitch != nil || r.Secure != nil || r.Passcode != nil ||
				r.PK != nil
		}

		var reqBody req
		if err := httputil.ReadJSON(r, &reqBody); err != nil {
			if err != io.EOF {
				hv.log(r).Warnf("putApp request: %v", err)
			}

			httputil.WriteJSON(w, r, http.StatusBadRequest, usermanager.ErrMalformedRequest)

			return
		}

		if reqBody.AutoStart != nil {
			if *reqBody.AutoStart != ctx.App.AutoStart {
				if err := ctx.API.SetAutoStart(ctx.App.Name, *reqBody.AutoStart); err != nil {
					httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
					return
				}
			}
		}

		if reqBody.Passcode != nil {
			if err := ctx.API.SetAppPassword(ctx.App.Name, *reqBody.Passcode); err != nil {
				httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
				return
			}
		}

		if reqBody.PK != nil {
			if err := ctx.API.SetAppPK(ctx.App.Name, *reqBody.PK); err != nil {
				httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
				return
			}
		}

		if reqBody.Killswitch != nil {
			if err := ctx.API.SetAppKillswitch(ctx.App.Name, *reqBody.Killswitch); err != nil {
				httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
				return
			}
		}

		if reqBody.Secure != nil {
			if err := ctx.API.SetAppSecure(ctx.App.Name, *reqBody.Secure); err != nil {
				httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
				return
			}
		}

		if shouldRestartApp(reqBody) {
			if err := ctx.API.RestartApp(ctx.App.Name); err != nil {
				httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
				return
			}
		}

		if reqBody.Status != nil {
			switch *reqBody.Status {
			case statusStop:
				if err := ctx.API.StopApp(ctx.App.Name); err != nil {
					httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
					return
				}
			case statusStart:
				if err := ctx.API.StartApp(ctx.App.Name); err != nil {
					httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
					return
				}
			default:
				errMsg := fmt.Errorf("value of 'status' field is %d when expecting 0 or 1", *reqBody.Status)
				httputil.WriteJSON(w, r, http.StatusBadRequest, errMsg)
				return
			}
		}

		httputil.WriteJSON(w, r, http.StatusOK, ctx.App)
	})
}

// LogsRes parses logs as json, along with the last obtained timestamp for use on subsequent requests
type LogsRes struct {
	LastLogTimestamp string   `json:"last_log_timestamp"`
	Logs             []string `json:"logs"`
}

func (hv *Hypervisor) appLogsSince() http.HandlerFunc {
	return hv.withCtx(hv.appCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		since := r.URL.Query().Get("since")
		since = strings.Replace(since, " ", "+", 1) // we need to put '+' again that was replaced in the query string

		// if time is not parsable or empty default to return all logs
		t, err := time.Parse(time.RFC3339Nano, since)
		if err != nil {
			t = time.Unix(0, 0)
		}

		logs, err := ctx.API.LogsSince(t, ctx.App.Name)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		if len(logs) == 0 {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, fmt.Errorf("no new available logs"))
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, &LogsRes{
			LastLogTimestamp: appcommon.TimestampFromLog(logs[len(logs)-1]),
			Logs:             logs,
		})
	})
}

func (hv *Hypervisor) appConnections() http.HandlerFunc {
	return hv.withCtx(hv.appCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		cSummary, err := ctx.API.GetAppConnectionsSummary(ctx.App.Name)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, &cSummary)
	})
}

func (hv *Hypervisor) getTransportTypes() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		types, err := ctx.API.TransportTypes()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, types)
	})
}

func (hv *Hypervisor) getTransports() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		qTypes := strSliceFromQuery(r, "type", nil)

		qPKs, err := pkSliceFromQuery(r, "pk", nil)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusBadRequest, err)
			return
		}

		qLogs, err := httputil.BoolFromQuery(r, "logs", true)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusBadRequest, err)
			return
		}

		transports, err := ctx.API.Transports(qTypes, qPKs, qLogs)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}
		httputil.WriteJSON(w, r, http.StatusOK, transports)
	})
}

func (hv *Hypervisor) postTransport() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		var reqBody struct {
			TpType string        `json:"transport_type"`
			Remote cipher.PubKey `json:"remote_pk"`
			Public bool          `json:"public"`
		}

		if err := httputil.ReadJSON(r, &reqBody); err != nil {
			if err != io.EOF {
				hv.log(r).Warnf("postTransport request: %v", err)
			}

			httputil.WriteJSON(w, r, http.StatusBadRequest, usermanager.ErrMalformedRequest)

			return
		}

		const timeout = 30 * time.Second
		tSummary, err := ctx.API.AddTransport(reqBody.Remote, reqBody.TpType, reqBody.Public, timeout)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, tSummary)
	})
}

func (hv *Hypervisor) getTransport() http.HandlerFunc {
	return hv.withCtx(hv.tpCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		httputil.WriteJSON(w, r, http.StatusOK, ctx.Tp)
	})
}

func (hv *Hypervisor) deleteTransport() http.HandlerFunc {
	return hv.withCtx(hv.tpCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		if err := ctx.API.RemoveTransport(ctx.Tp.ID); err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, true)
	})
}

func (hv *Hypervisor) deleteTransports() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		var transports []string
		response := make(map[string]elementResponse)
		err := json.NewDecoder(r.Body).Decode(&transports)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusBadRequest, err)
			return
		}
		for _, transport := range transports {
			transportBoxed, err := uuid.Parse(transport)
			if err != nil {
				response[transport] = elementResponse{
					Success: false,
					Error:   err.Error(),
				}
				continue
			}
			_, err = ctx.API.Transport(transportBoxed)
			if err != nil {
				if err.Error() == ErrNotFound.Error() {
					errMsg := fmt.Errorf("transport of ID %s is not found", transportBoxed)
					response[transport] = elementResponse{
						Success: false,
						Error:   errMsg.Error(),
					}
					continue
				}
			}

			if err := ctx.API.RemoveTransport(transportBoxed); err != nil {
				response[transport] = elementResponse{
					Success: false,
					Error:   err.Error(),
				}
				continue
			}
			response[transport] = elementResponse{
				Success: true,
			}
		}
		httputil.WriteJSON(w, r, http.StatusOK, response)
	})
}

type routingRuleResp struct {
	Key     routing.RouteID      `json:"key"`
	Rule    string               `json:"rule"`
	Summary *routing.RuleSummary `json:"rule_summary,omitempty"`
}

func makeRoutingRuleResp(key routing.RouteID, rule routing.Rule, summary bool) routingRuleResp {
	resp := routingRuleResp{
		Key:  key,
		Rule: hex.EncodeToString(rule),
	}

	if summary {
		resp.Summary = rule.Summary()
	}

	return resp
}

func (hv *Hypervisor) getRoutes() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		qSummary, err := httputil.BoolFromQuery(r, "summary", false)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusBadRequest, err)
			return
		}

		rules, err := ctx.API.RoutingRules()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		resp := make([]routingRuleResp, len(rules))
		for i, rule := range rules {
			resp[i] = makeRoutingRuleResp(rule.KeyRouteID(), rule, qSummary)
		}

		httputil.WriteJSON(w, r, http.StatusOK, resp)
	})
}

func (hv *Hypervisor) postRoute() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		var summary routing.RuleSummary
		if err := httputil.ReadJSON(r, &summary); err != nil {
			if err != io.EOF {
				hv.log(r).Warnf("postRoute request: %v", err)
			}

			httputil.WriteJSON(w, r, http.StatusBadRequest, usermanager.ErrMalformedRequest)

			return
		}

		rule, err := summary.ToRule()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusBadRequest, err)
			return
		}

		if err := ctx.API.SaveRoutingRule(rule); err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, makeRoutingRuleResp(rule.KeyRouteID(), rule, true))
	})
}

func (hv *Hypervisor) getRoute() http.HandlerFunc {
	return hv.withCtx(hv.routeCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		qSummary, err := httputil.BoolFromQuery(r, "summary", true)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusBadRequest, err)
			return
		}

		rule, err := ctx.API.RoutingRule(ctx.RtKey)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusNotFound, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, makeRoutingRuleResp(ctx.RtKey, rule, qSummary))
	})
}

func (hv *Hypervisor) putRoute() http.HandlerFunc {
	return hv.withCtx(hv.routeCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		var rSummary routing.RuleSummary
		if err := httputil.ReadJSON(r, &rSummary); err != nil {
			if err != io.EOF {
				hv.log(r).Warnf("putRoute request: %v", err)
			}

			httputil.WriteJSON(w, r, http.StatusBadRequest, usermanager.ErrMalformedRequest)

			return
		}

		rule, err := rSummary.ToRule()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusBadRequest, err)
			return
		}

		if err := ctx.API.SaveRoutingRule(rule); err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, makeRoutingRuleResp(ctx.RtKey, rule, true))
	})
}

func (hv *Hypervisor) deleteRoute() http.HandlerFunc {
	return hv.withCtx(hv.routeCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		if err := ctx.API.RemoveRoutingRule(ctx.RtKey); err != nil {
			httputil.WriteJSON(w, r, http.StatusNotFound, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, true)
	})
}

func (hv *Hypervisor) deleteRoutes() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		var rids []string
		response := make(map[string]elementResponse)
		err := json.NewDecoder(r.Body).Decode(&rids)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusNotFound, err)
			return
		}
		rules, err := ctx.API.RoutingRules()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusNotFound, err)
			return
		}
		for _, rid := range rids {
			ridUint64, err := strconv.ParseUint(rid, 10, 32)
			if err != nil {
				response[rid] = elementResponse{
					Success: false,
					Error:   err.Error(),
				}
				continue
			}
			routeID := routing.RouteID(ridUint64)
			contains := false
			for _, rule := range rules {
				if rule.KeyRouteID() == routeID {
					contains = true
				}
			}
			if !contains {
				errMsg := fmt.Errorf("route of ID %s is not found", rid)
				response[rid] = elementResponse{
					Success: false,
					Error:   errMsg.Error(),
				}
				continue
			}

			if err := ctx.API.RemoveRoutingRule(routeID); err != nil {
				response[rid] = elementResponse{
					Success: false,
					Error:   err.Error(),
				}
				continue
			}
			response[rid] = elementResponse{
				Success: true,
			}
		}
		httputil.WriteJSON(w, r, http.StatusOK, response)
	})
}

type routeGroupResp struct {
	routing.RuleConsumeFields
	FwdRule routing.RuleForwardFields `json:"resp"`
}

func makeRouteGroupResp(info RouteGroupInfo) routeGroupResp {
	if len(info.FwdRule) == 0 || len(info.ConsumeRule) == 0 {
		return routeGroupResp{}
	}

	return routeGroupResp{
		RuleConsumeFields: *info.ConsumeRule.Summary().ConsumeFields,
		FwdRule:           *info.FwdRule.Summary().ForwardFields,
	}
}

func (hv *Hypervisor) getRouteGroups() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		routegroups, err := ctx.API.RouteGroups()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		resp := make([]routeGroupResp, len(routegroups))
		for i, l := range routegroups {
			resp[i] = makeRouteGroupResp(l)
		}

		httputil.WriteJSON(w, r, http.StatusOK, resp)
	})
}

// NOTE: Reply comes with a delay, because of check if new executable is started successfully.
func (hv *Hypervisor) restart() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		if err := ctx.API.Restart(); err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		httputil.WriteJSON(w, r, http.StatusOK, true)
	})
}

// executes a command and returns its output
func (hv *Hypervisor) exec() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		var reqBody struct {
			Command string `json:"command"`
		}

		if err := httputil.ReadJSON(r, &reqBody); err != nil {
			if err != io.EOF {
				hv.log(r).Warnf("exec request: %v", err)
			}

			httputil.WriteJSON(w, r, http.StatusBadRequest, usermanager.ErrMalformedRequest)

			return
		}

		out, err := ctx.API.Exec(reqBody.Command)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		output := struct {
			Output string `json:"output"`
		}{strings.TrimSpace(string(out))}

		httputil.WriteJSON(w, r, http.StatusOK, output)
	})
}

func (hv *Hypervisor) updateVisor() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		var updateConfig updater.UpdateConfig

		if err := httputil.ReadJSON(r, &updateConfig); err != nil {
			hv.log(r).Warnf("update visor request: %v", err)
			httputil.WriteJSON(w, r, http.StatusBadRequest, usermanager.ErrMalformedRequest)

			return
		}

		if updateConfig.Channel == "" {
			updateConfig.Channel = updater.ChannelStable
		}

		updated, err := ctx.API.Update(updateConfig)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		output := struct {
			Updated bool `json:"updated"`
		}{updated}

		httputil.WriteJSON(w, r, http.StatusOK, output)
	})
}

func (hv *Hypervisor) updateVisorWS() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			hv.log(r).WithError(err).Warnf("Failed to upgrade to websocket.")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		defer func() {
			if err := ws.Close(websocket.StatusNormalClosure, "response sent"); err != nil {
				hv.log(r).WithError(err).Warnf("Failed to close WebSocket connection")
			}
		}()

		_, raw, err := ws.Read(context.Background())
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		var updateConfig updater.UpdateConfig
		if err := json.Unmarshal(raw, &updateConfig); err != nil {
			hv.log(r).Warnf("update visor request %v: %v", string(raw), err)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		if updateConfig.Channel == "" {
			updateConfig.Channel = updater.ChannelStable
		}

		consumer := make(chan StatusMessage, 512)
		hv.visorMu.Lock()
		if mux := hv.visorChanMux[ctx.Addr.PK]; mux == nil {
			ch := ctx.API.UpdateWithStatus(updateConfig)
			hv.visorChanMux[ctx.Addr.PK] = newChanMux(ch, []chan<- StatusMessage{consumer})
		} else {
			hv.visorChanMux[ctx.Addr.PK].addConsumer(consumer)
		}
		hv.visorMu.Unlock()

		defer func() {
			hv.visorMu.Lock()
			delete(hv.visorChanMux, ctx.Addr.PK)
			hv.visorMu.Unlock()
		}()

		for status := range consumer {
			if status.IsError {
				if err := ws.Close(websocket.StatusAbnormalClosure, status.Text); err != nil {
					hv.log(r).WithError(err).Warnf("failed to close WebSocket (abnormal)")
					return
				}
			}

			output := struct {
				Status string `json:"status"`
			}{status.Text}

			rawOutput, err := json.Marshal(output)
			if err != nil {
				hv.log(r).WithError(err).Errorf("Failed to marshal JSON: %#v", output)
				return
			}

			if err := ws.Write(context.Background(), websocket.MessageText, rawOutput); err != nil {
				hv.log(r).WithError(err).Warnf("Failed to write WebSocket response")
			}
		}

		if err := ws.Close(websocket.StatusNormalClosure, "finished"); err != nil {
			hv.log(r).WithError(err).Warnf("failed to close WebSocket (normal)")
		}
	})
}

func (hv *Hypervisor) isVisorWSUpdateRunning() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		running := false
		hv.visorMu.Lock()
		running = hv.visorChanMux != nil && hv.visorChanMux[ctx.Addr.PK] != nil
		hv.visorMu.Unlock()

		resp := struct {
			Running bool `json:"running"`
		}{
			running,
		}

		httputil.WriteJSON(w, r, http.StatusOK, resp)
	})
}

func (hv *Hypervisor) visorUpdateAvailable() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		channel := updater.Channel(chi.URLParam(r, "channel"))
		if channel == "" {
			channel = updater.ChannelStable
		}

		version, err := ctx.API.UpdateAvailable(channel)
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		overview, err := ctx.API.Overview()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}

		output := struct {
			Available        bool   `json:"available"`
			CurrentVersion   string `json:"current_version"`
			AvailableVersion string `json:"available_version,omitempty"`
			ReleaseURL       string `json:"release_url,omitempty"`
		}{
			Available:      version != nil,
			CurrentVersion: overview.BuildInfo.Version,
		}

		if version != nil {
			output.AvailableVersion = version.String()
			output.ReleaseURL = version.ReleaseURL()
		}

		httputil.WriteJSON(w, r, http.StatusOK, output)
	})
}

func (hv *Hypervisor) getRuntimeLogs() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		logs, err := ctx.API.RuntimeLogs()
		if err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(logs))
		if err != nil {
			hv.visor.log.Errorf("Cannot write response: %s", err)
		}
	})
}

func (hv *Hypervisor) postMinHops() http.HandlerFunc {
	return hv.withCtx(hv.visorCtx, func(w http.ResponseWriter, r *http.Request, ctx *httpCtx) {
		var reqBody struct {
			MinHops uint16 `json:"min_hops"`
		}

		if err := httputil.ReadJSON(r, &reqBody); err != nil {
			if err != io.EOF {
				hv.log(r).Warnf("postMinHops request: %v", err)
			}
			httputil.WriteJSON(w, r, http.StatusBadRequest, usermanager.ErrMalformedRequest)
			return
		}

		if err := ctx.API.SetMinHops(reqBody.MinHops); err != nil {
			httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
			return
		}
		httputil.WriteJSON(w, r, http.StatusOK, struct{}{})
	})
}

/*
	<<< Helper functions >>>
*/

func (hv *Hypervisor) visorConn(pk cipher.PubKey) (Conn, bool) {
	hv.mu.RLock()
	conn, ok := hv.visors[pk]
	hv.mu.RUnlock()

	return conn, ok
}

type httpCtx struct {
	// Hypervisor
	Conn

	// App
	App *launcher.AppState

	// Transport
	Tp *TransportSummary

	// Route
	RtKey routing.RouteID
}

type (
	valuesFunc  func(w http.ResponseWriter, r *http.Request) (*httpCtx, bool)
	handlerFunc func(w http.ResponseWriter, r *http.Request, ctx *httpCtx)
)

func (hv *Hypervisor) withCtx(vFunc valuesFunc, hFunc handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if rv, ok := vFunc(w, r); ok {
			hFunc(w, r, rv)
		}
	}
}

func (hv *Hypervisor) visorCtx(w http.ResponseWriter, r *http.Request) (*httpCtx, bool) {
	pk, err := pkFromParam(r, "pk")
	if err != nil {
		httputil.WriteJSON(w, r, http.StatusBadRequest, err)
		return nil, false
	}

	if pk != hv.c.PK {
		v, ok := hv.visorConn(pk)

		if !ok {
			httputil.WriteJSON(w, r, http.StatusNotFound, fmt.Errorf("visor of pk '%s' not found", pk))
			return nil, false
		}

		return &httpCtx{
			Conn: v,
		}, true
	}
	hv.mu.Lock()
	conn := hv.selfConn
	hv.mu.Unlock()

	return &httpCtx{
		Conn: conn,
	}, true
}

func (hv *Hypervisor) appCtx(w http.ResponseWriter, r *http.Request) (*httpCtx, bool) {
	ctx, ok := hv.visorCtx(w, r)
	if !ok {
		return nil, false
	}

	appName := chi.URLParam(r, "app")

	apps, err := ctx.API.Apps()
	if err != nil {
		httputil.WriteJSON(w, r, http.StatusInternalServerError, err)
		return nil, false
	}

	for _, a := range apps {
		if a.Name == appName {
			ctx.App = a
			return ctx, true
		}
	}

	errMsg := fmt.Errorf("can not find app of name %s from visor %s", appName, ctx.Addr.PK)
	httputil.WriteJSON(w, r, http.StatusNotFound, errMsg)

	return nil, false
}

func (hv *Hypervisor) tpCtx(w http.ResponseWriter, r *http.Request) (*httpCtx, bool) {
	ctx, ok := hv.visorCtx(w, r)
	if !ok {
		return nil, false
	}

	tid, err := uuidFromParam(r, "tid")
	if err != nil {
		httputil.WriteJSON(w, r, http.StatusBadRequest, err)
		return nil, false
	}

	tp, err := ctx.API.Transport(tid)
	if err != nil {
		if err.Error() == ErrNotFound.Error() {
			errMsg := fmt.Errorf("transport of ID %s is not found", tid)
			httputil.WriteJSON(w, r, http.StatusNotFound, errMsg)

			return nil, false
		}

		httputil.WriteJSON(w, r, http.StatusInternalServerError, err)

		return nil, false
	}

	ctx.Tp = tp

	return ctx, true
}

func (hv *Hypervisor) routeCtx(w http.ResponseWriter, r *http.Request) (*httpCtx, bool) {
	ctx, ok := hv.visorCtx(w, r)
	if !ok {
		return nil, false
	}

	rid, err := ridFromParam(r, "rid")
	if err != nil {
		httputil.WriteJSON(w, r, http.StatusBadRequest, err)
		return nil, false
	}

	ctx.RtKey = rid

	return ctx, true
}

func pkFromParam(r *http.Request, key string) (cipher.PubKey, error) {
	pk := cipher.PubKey{}
	err := pk.UnmarshalText([]byte(chi.URLParam(r, key)))

	return pk, err
}

func uuidFromParam(r *http.Request, key string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, key))
}

func ridFromParam(r *http.Request, key string) (routing.RouteID, error) {
	rid, err := strconv.ParseUint(chi.URLParam(r, key), 10, 32)
	if err != nil {
		return 0, errors.New("invalid route ID provided")
	}

	return routing.RouteID(rid), nil
}

func strSliceFromQuery(r *http.Request, key string, defaultVal []string) []string {
	slice, ok := r.URL.Query()[key]
	if !ok {
		return defaultVal
	}

	return slice
}

func pkSliceFromQuery(r *http.Request, key string, defaultVal []cipher.PubKey) ([]cipher.PubKey, error) {
	qPKs, ok := r.URL.Query()[key]
	if !ok {
		return defaultVal, nil
	}

	pks := make([]cipher.PubKey, len(qPKs))

	for i, qPK := range qPKs {
		pk := cipher.PubKey{}
		if err := pk.UnmarshalText([]byte(qPK)); err != nil {
			return nil, err
		}

		pks[i] = pk
	}

	return pks, nil
}

func (hv *Hypervisor) serveDmsg(ctx context.Context, log *logging.Logger) {
	go func() {
		<-hv.dmsgC.Ready()
		if err := hv.ServeRPC(ctx, hv.c.DmsgPort); err != nil {
			log.WithError(err).Fatal("Failed to serve RPC client over dmsg.")
		}
	}()
	log.WithField("addr", dmsg.Addr{PK: hv.c.PK, Port: hv.c.DmsgPort}).
		Info("Serving RPC client over dmsg.")
}

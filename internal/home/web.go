package home

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"net/netip"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/AdguardTeam/AdGuardHome/internal/agh"
	"github.com/AdguardTeam/AdGuardHome/internal/aghhttp"
	"github.com/AdguardTeam/AdGuardHome/internal/aghnet"
	"github.com/AdguardTeam/AdGuardHome/internal/updater"
	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/logutil/slogutil"
	"github.com/AdguardTeam/golibs/netutil"
	"github.com/AdguardTeam/golibs/netutil/httputil"
	"github.com/AdguardTeam/golibs/osutil"
	"github.com/AdguardTeam/golibs/osutil/executil"
	"github.com/AdguardTeam/golibs/service"
	"github.com/NYTimes/gziphandler"
)

// TODO(a.garipov): Make configurable.
const (
	// readTimeout is the maximum duration for reading the entire request,
	// including the body.
	readTimeout = 60 * time.Second
	// readHdrTimeout is the amount of time allowed to read request headers.
	readHdrTimeout = 60 * time.Second
	// writeTimeout is the maximum duration before timing out writes of the
	// response.
	writeTimeout = 5 * time.Minute
)

// webAPIConfig is a configuration structure for webAPI.
type webAPIConfig struct {
	// CommandConstructor is used to run external commands.  It must not be nil.
	CommandConstructor executil.CommandConstructor

	// updater is used for updating AdGuard home.  If disableUpdate is set to
	// false, it must not be nil.
	updater *updater.Updater

	// logger is a slog logger used in webAPI. It must not be nil.
	logger *slog.Logger

	// baseLogger is used to create loggers for other entities.  It must not be
	// nil.
	baseLogger *slog.Logger

	// confModifier is used to update the global configuration.
	confModifier agh.ConfigModifier

	// httpReg registers HTTP handlers.  It must not be nil.
	httpReg aghhttp.Registrar

	// auth stores web user information and handles authentication.  It must not
	// be nil.
	auth *auth

	// mux is the default *http.ServeMux, the same as [globalContext.mux].  It
	// must not be nil.
	mux *http.ServeMux

	// hostsContainer is used for DNS initialization on updates.
	hostsContainer *aghnet.HostsContainer

	// clientFS is used to initialize file server.  It must not be nil.
	clientFS fs.FS

	// BindAddr is the binding address with port for plain HTTP web interface.
	BindAddr netip.AddrPort

	// workDir is the base working directory.
	workDir string

	// confPath is the configuration file path.
	confPath string

	// pidFilePath is a path to a PID file.
	pidFilePath string

	// ReadTimeout is an option to pass to http.Server for setting an
	// appropriate field.
	ReadTimeout time.Duration

	// ReadHeaderTimeout is an option to pass to http.Server for setting an
	// appropriate field.
	ReadHeaderTimeout time.Duration

	// WriteTimeout is an option to pass to http.Server for setting an
	// appropriate field.
	WriteTimeout time.Duration

	// defaultWebPort is the suggested default HTTP port for the install wizard.
	defaultWebPort uint16

	// firstRun, if true, tells AdGuard Home to register install handlers.
	firstRun bool

	// disableUpdate, if true, tells AdGuard Home to not check for updates.
	disableUpdate bool
}

// webAPI is the web UI and API server.
type webAPI struct {
	conf *webAPIConfig

	// confModifier is used to update the global configuration.
	confModifier agh.ConfigModifier

	// cmdCons is used to run external commands.
	cmdCons executil.CommandConstructor

	// httpReg registers HTTP handlers.
	httpReg aghhttp.Registrar

	// TODO(a.garipov): Refactor all these servers.
	httpServer *http.Server

	// logger is a slog logger used in webAPI. It must not be nil.
	logger *slog.Logger

	// baseLogger is used to create loggers for other entities.  It must not be
	// nil.
	baseLogger *slog.Logger

	// auth stores web user information and handles authentication.
	auth *auth

	// hostsContainer is used for DNS initialization on updates.
	hostsContainer *aghnet.HostsContainer

	// closed is true when the web API server is shutting down.
	closed atomic.Bool

	// pidFilePath is used for cleanup.
	pidFilePath string

	// startTime is the start time of the web API server in Unix milliseconds.
	startTime time.Time
}

// newWebAPI creates a new instance of the web UI and API server.  conf must be
// valid.
//
// TODO(a.garipov):  Return a proper error.
func newWebAPI(ctx context.Context, conf *webAPIConfig) (w *webAPI) {
	conf.logger.InfoContext(ctx, "initializing")

	w = &webAPI{
		conf:           conf,
		confModifier:   conf.confModifier,
		cmdCons:        conf.CommandConstructor,
		httpReg:        conf.httpReg,
		logger:         conf.logger,
		baseLogger:     conf.baseLogger,
		auth:           conf.auth,
		hostsContainer: conf.hostsContainer,
		pidFilePath:    conf.pidFilePath,
		startTime:      time.Now(),
	}

	clientFS := http.FileServer(http.FS(conf.clientFS))

	mux := conf.mux
	// if not configured, redirect / to /install.html, otherwise redirect /install.html to /
	mux.Handle("/", httputil.Wrap(
		clientFS,
		httputil.MiddlewareFunc(w.postInstallHandler),
		httputil.MiddlewareFunc(gziphandler.GzipHandler),
	))

	// add handlers for /install paths, we only need them when we're not configured yet
	if conf.firstRun {
		conf.logger.InfoContext(
			ctx,
			"This is the first launch of AdGuard Home, redirecting everything to /install.html",
		)

		mux.Handle("/install.html", w.preInstallHandler(clientFS))
		w.registerInstallHandlers()
	} else {
		w.registerControlHandlers()
	}

	return w
}

// loggerKeyServer is the key used by [webAPI] to identify servers.
const loggerKeyServer = "server"

// start starts serving HTTP requests.
func (web *webAPI) start(ctx context.Context) {
	defer slogutil.RecoverAndExit(ctx, web.logger, osutil.ExitCodeFailure)

	web.logger.InfoContext(ctx, "AdGuard Home is available at the following addresses:")

	// This loop is used as an ability to change listening host and/or port.
	for !web.closed.Load() {
		printHTTPAddresses(ctx, web.logger)
		errs := make(chan error, 2)

		logger := web.baseLogger.With(loggerKeyServer, "plain")
		hdlr := web.wrapMux(logger)

		// Enable unencrypted HTTP/2, e.g. for proxies.
		protocols := &http.Protocols{}
		protocols.SetUnencryptedHTTP2(true)
		protocols.SetHTTP1(true)

		// Create a new instance, because the Web is not usable after Shutdown.
		web.httpServer = &http.Server{
			Addr:              web.conf.BindAddr.String(),
			Handler:           hdlr,
			ReadTimeout:       web.conf.ReadTimeout,
			ReadHeaderTimeout: web.conf.ReadHeaderTimeout,
			WriteTimeout:      web.conf.WriteTimeout,
			ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
			Protocols:         protocols,
		}
		go func() {
			defer slogutil.RecoverAndLog(ctx, logger)

			logger.InfoContext(ctx, "starting plain server", "addr", web.httpServer.Addr)

			errs <- web.httpServer.ListenAndServe()
		}()

		err := <-errs
		if !errors.Is(err, http.ErrServerClosed) {
			cleanupAlways(ctx, logger, web.pidFilePath)

			panic(err)
		}

		// We use ErrServerClosed as a sign that we need to rebind on a new
		// address, so go back to the start of the loop.
	}
}

// wrapMux wraps mux with common middlewares.  l must not be nil.
func (web *webAPI) wrapMux(l *slog.Logger) (h http.Handler) {
	h = httputil.Wrap(web.conf.mux, httputil.MiddlewareFunc(limitRequestBody))

	// TODO(a.garipov):  Remove other logs like this in other code.
	logMw := httputil.NewLogMiddleware(l, slog.LevelDebug)
	h = logMw.Wrap(h)

	h = web.auth.middleware().Wrap(h)

	return h
}

// close gracefully shuts down the HTTP servers.
func (web *webAPI) close(ctx context.Context) {
	web.logger.InfoContext(ctx, "stopping http server")

	web.closed.Store(true)

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	shutdownSrv(ctx, web.logger, web.httpServer)

	if web.auth != nil {
		web.auth.close(ctx)
	}

	web.logger.InfoContext(ctx, "stopped http server")
}

// startPprof launches the debug and profiling server on the provided port.
// baseLogger must not be nil.
func startPprof(baseLogger *slog.Logger, port uint16) {
	addr := netip.AddrPortFrom(netutil.IPv4Localhost(), port)

	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	mux := http.NewServeMux()
	httputil.RoutePprof(mux)

	ctx := context.Background()
	logger := baseLogger.With(slogutil.KeyPrefix, "pprof")

	go func() {
		defer slogutil.RecoverAndLog(ctx, logger)

		logger.InfoContext(ctx, "listening", "addr", addr)
		err := http.ListenAndServe(addr.String(), mux)
		if !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "shutting down", slogutil.KeyError, err)
		}
	}()
}

// type check
var _ service.Shutdowner = (*webAPI)(nil)

// Shutdown implements the [service.Shutdowner] interface.  It gracefully shuts
// down the web API server.  It always returns nil error.
func (web *webAPI) Shutdown(ctx context.Context) (err error) {
	web.close(ctx)

	return nil
}

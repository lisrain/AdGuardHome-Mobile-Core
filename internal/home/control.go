package home

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/AdguardTeam/AdGuardHome/internal/aghhttp"
	"github.com/AdguardTeam/AdGuardHome/internal/aghnet"
	"github.com/AdguardTeam/AdGuardHome/internal/version"
	"github.com/AdguardTeam/golibs/httphdr"
	"github.com/AdguardTeam/golibs/netutil"
	"github.com/NYTimes/gziphandler"
)

// appendDNSAddrs is a convenient helper for appending a formatted form of DNS
// addresses to a slice of strings.
func appendDNSAddrs(dst []string, addrs ...netip.Addr) (res []string) {
	for _, addr := range addrs {
		hostport := addr.String()
		if p := config.DNS.Port; p != defaultPortDNS {
			hostport = netutil.JoinHostPort(hostport, p)
		}

		dst = append(dst, hostport)
	}

	return dst
}

// appendDNSAddrsWithIfaces formats and appends all DNS addresses from src to
// dst.  It also adds the IP addresses of all network interfaces if src contains
// an unspecified IP address.
func appendDNSAddrsWithIfaces(dst []string, src []netip.Addr) (res []string, err error) {
	ifacesAdded := false
	for _, h := range src {
		if !h.IsUnspecified() {
			dst = appendDNSAddrs(dst, h)

			continue
		} else if ifacesAdded {
			continue
		}

		// Add addresses of all network interfaces for addresses like
		// "0.0.0.0" and "::".
		var ifaces []*aghnet.NetInterface
		ifaces, err = aghnet.GetValidNetInterfacesForWeb()
		if err != nil {
			return nil, fmt.Errorf("cannot get network interfaces: %w", err)
		}

		for _, iface := range ifaces {
			dst = appendDNSAddrs(dst, iface.Addresses...)
		}

		ifacesAdded = true
	}

	return dst, nil
}

// collectDNSAddresses returns the list of DNS addresses the server is listening
// on, including the addresses on all interfaces in cases of unspecified IPs.
func collectDNSAddresses() (addrs []string, err error) {
	if hosts := config.DNS.BindHosts; len(hosts) == 0 {
		addrs = appendDNSAddrs(addrs, netutil.IPv4Localhost())
	} else {
		addrs, err = appendDNSAddrsWithIfaces(addrs, hosts)
		if err != nil {
			return nil, fmt.Errorf("collecting dns addresses: %w", err)
		}
	}

	return addrs, nil
}

// statusResponse is a response for /control/status endpoint.
type statusResponse struct {
	Version  string   `json:"version"`
	Language string   `json:"language"`
	DNSAddrs []string `json:"dns_addresses"`
	DNSPort  uint16   `json:"dns_port"`
	HTTPPort uint16   `json:"http_port"`

	// ProtectionDisabledDuration is the duration of the protection pause in
	// milliseconds.
	ProtectionDisabledDuration int64 `json:"protection_disabled_duration"`

	// StartTime is the start time of the web API server in Unix milliseconds.
	StartTime aghhttp.JSONTime `json:"start_time"`

	ProtectionEnabled bool `json:"protection_enabled"`
	// TODO(e.burkov): Inspect if front-end doesn't requires this field as
	// openapi.yaml declares.
	IsDHCPAvailable bool `json:"dhcp_available"`
	IsRunning       bool `json:"running"`
}

func (web *webAPI) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := web.logger

	dnsAddrs, err := collectDNSAddresses()
	if err != nil {
		// Don't add a lot of formatting, since the error is already
		// wrapped by collectDNSAddresses.
		aghhttp.ErrorAndLog(ctx, l, r, w, http.StatusInternalServerError, "%s", err)

		return
	}

	var (
		protDisabledUntil *time.Time
		protEnabled       bool
	)
	if globalContext.dnsServer != nil {
		protEnabled, protDisabledUntil = globalContext.dnsServer.UpdatedProtectionStatus(ctx)
	}

	var resp statusResponse
	func() {
		config.RLock()
		defer config.RUnlock()

		var protectionDisabledDuration int64
		if protDisabledUntil != nil {
			// Make sure that we don't send negative numbers to the frontend,
			// since enough time might have passed to make the difference less
			// than zero.
			protectionDisabledDuration = max(0, time.Until(*protDisabledUntil).Milliseconds())
		}

		resp = statusResponse{
			Version:                    version.Version(),
			Language:                   config.Language,
			DNSAddrs:                   dnsAddrs,
			DNSPort:                    config.DNS.Port,
			HTTPPort:                   config.HTTPConfig.Address.Port(),
			ProtectionDisabledDuration: protectionDisabledDuration,
			StartTime:                  aghhttp.JSONTime(web.startTime),
			ProtectionEnabled:          protEnabled,
			IsRunning:                  isRunning(),
		}
	}()

	aghhttp.WriteJSONResponseOK(ctx, l, w, r, resp)
}

// registerControlHandlers sets up HTTP handlers for various control endpoints.
func (web *webAPI) registerControlHandlers() {
	mux := web.conf.mux

	mux.Handle(
		"/control/version.json",
		web.postInstallHandler(http.HandlerFunc(web.handleVersionJSON)),
	)
	web.httpReg.Register(http.MethodPost, "/control/update", web.handleUpdate)

	web.httpReg.Register(http.MethodGet, "/control/status", web.handleStatus)
	web.httpReg.Register(http.MethodGet, "/control/profile", web.handleGetProfile)
	web.httpReg.Register(http.MethodPut, "/control/profile/update", web.handlePutProfile)

	web.registerAuthHandlers()
}

// webMw provides middleware for route handlers.  The set method must be called
// to initialize the middleware.
type webMw struct {
	// postInstallMw is middleware that verifies that AdGuard Home is not
	// running for the first time.
	postInstallMw func(h http.Handler) (wrapped http.Handler)

	// ensureMw is like postInstallMw, but also applies gzip and enforces the
	// HTTP method.
	ensureMw aghhttp.WrapFunc
}

// set sets the middleware functions used to build handler chains.
func (mw *webMw) set(web *webAPI) {
	mw.postInstallMw = web.postInstallHandler

	mw.ensureMw = func(method string, h http.HandlerFunc) (wrapped http.Handler) {
		return web.postInstallHandler(gziphandler.GzipHandler(web.ensure(method, h)))
	}
}

// wrap returns a wrapped HTTP handler for the given route.
//
// TODO(s.chzhen):  Implement [httputil.Middleware].
func (mw *webMw) wrap(method string, h http.HandlerFunc) (wrapped http.Handler) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw.ensureMw(method, h).ServeHTTP(w, r)
	})
}

// ensure returns a wrapped handler that verifies the request method.  It also
// performs additional method and header checks.
func (web *webAPI) ensure(
	method string,
	handler func(http.ResponseWriter, *http.Request),
) (wrapped http.HandlerFunc) {
	return func(w http.ResponseWriter, r *http.Request) {
		m := r.Method
		if m != method {
			aghhttp.ErrorAndLog(
				r.Context(),
				web.logger,
				r,
				w,
				http.StatusMethodNotAllowed,
				"only method %s is allowed",
				method,
			)

			return
		}

		if modifiesData(m) {
			if !web.ensureContentType(w, r) {
				return
			}

			globalContext.controlLock.Lock()
			defer globalContext.controlLock.Unlock()
		}

		handler(w, r)
	}
}

// modifiesData returns true if m is an HTTP method that can modify data.
func modifiesData(m string) (ok bool) {
	return m == http.MethodPost || m == http.MethodPut || m == http.MethodDelete
}

// ensureContentType makes sure that the content type of a data-modifying
// request is set correctly.  If it is not, ensureContentType writes a response
// to w, and ok is false.
func (web *webAPI) ensureContentType(w http.ResponseWriter, r *http.Request) (ok bool) {
	const statusUnsup = http.StatusUnsupportedMediaType

	ctx := r.Context()

	cType := r.Header.Get(httphdr.ContentType)
	if r.ContentLength == 0 {
		if cType == "" {
			return true
		}

		// Assume that browsers always send a content type when submitting HTML
		// forms and require no content type for requests with no body to make
		// sure that the request comes from JavaScript.
		aghhttp.ErrorAndLog(
			ctx,
			web.logger,
			r,
			w,
			statusUnsup,
			"empty body with content-type %q not allowed",
			cType,
		)

		return false

	}

	const wantCType = aghhttp.HdrValApplicationJSON
	if cType == wantCType {
		return true
	}

	aghhttp.ErrorAndLog(
		ctx,
		web.logger,
		r,
		w,
		statusUnsup,
		"only content-type %s is allowed",
		wantCType,
	)

	return false
}

// preInstallHandler lets the handler run only if firstRun is true; it does not
// perform redirects.
func (web *webAPI) preInstallHandler(handler http.Handler) (wrapped http.Handler) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !web.conf.firstRun {
			// If it's not first run, do not allow access to install-only routes
			// (for example, /install.html once configuration is complete).
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)

			return
		}

		handler.ServeHTTP(w, r)
	})
}

// postInstallHandler lets the handler to run only if firstRun is false.
// Otherwise, it redirects to /install.html.
func (web *webAPI) postInstallHandler(handler http.Handler) (wrapped http.Handler) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if web.conf.firstRun &&
			!strings.HasPrefix(path, "/install.") &&
			!strings.HasPrefix(path, "/assets/") {
			http.Redirect(w, r, "install.html", http.StatusFound)

			return
		}

		handler.ServeHTTP(w, r)
	})
}

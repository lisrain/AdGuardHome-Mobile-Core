package configmgr

import (
	"net/netip"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/timeutil"
	"github.com/AdguardTeam/golibs/validate"
)

// HTTPConfig is the on-disk web API configuration.
type HTTPConfig struct {
	// Pprof defines the profiling HTTP handler.  It must not be nil.
	Pprof *HTTPPprofConfig `yaml:"pprof"`

	// Address is the addresses on which to serve web API.
	Address netip.AddrPort `yaml:"address"`

	// SessionTTL for a web session.
	SessionTTL timeutil.Duration `yaml:"session_ttl"`
}

// HTTPPprofConfig is the block with pprof HTTP configuration.
type HTTPPprofConfig struct {
	// Port for the profiling handler.
	Port uint16 `yaml:"port"`

	// Enabled defines if the profiling handler is enabled.
	Enabled bool `yaml:"enabled"`
}

// type check
var _ validate.Interface = (*HTTPConfig)(nil)

// Validate implements the [validate.Interface] interface for *HTTPConfig.
func (c *HTTPConfig) Validate() (err error) {
	if c == nil {
		return errors.ErrNoValue
	}

	var errs []error
	errs = validate.Append(errs, "pprof", c.Pprof)

	return errors.Join(errs...)
}

// type check
var _ validate.Interface = (*HTTPPprofConfig)(nil)

// Validate implements the [validate.Interface] interface for *HTTPPprofConfig.
func (c *HTTPPprofConfig) Validate() (err error) {
	if c == nil {
		return errors.ErrNoValue
	}

	// TODO(d.kolyshev):  Validate.

	return nil
}

package home

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AdguardTeam/AdGuardHome/internal/agh"
	"github.com/AdguardTeam/AdGuardHome/internal/aghalg"
	"github.com/AdguardTeam/AdGuardHome/internal/aghos"
	"github.com/AdguardTeam/AdGuardHome/internal/configmgr"
	"github.com/AdguardTeam/AdGuardHome/internal/configmigrate"
	"github.com/AdguardTeam/AdGuardHome/internal/dnsforward"
	"github.com/AdguardTeam/AdGuardHome/internal/filtering"
	"github.com/AdguardTeam/AdGuardHome/internal/filtering/rulelist"
	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/logutil/slogutil"
	"github.com/AdguardTeam/golibs/netutil"
	"github.com/AdguardTeam/golibs/timeutil"
	"github.com/google/renameio/v2/maybe"
	yaml "go.yaml.in/yaml/v4"
)

const (
	// dataDir is the name of a directory under the working one to store some
	// persistent data.
	dataDir = "data"

	// userFilterDataDir is the name of the directory used to store users'
	// FS-based rule lists.
	userFilterDataDir = "userfilters"

	// defaultFastestTimeout is the default timeout for proxy fastest upstream
	// mode.
	defaultFastestTimeout = 1 * time.Second
)

// osConfig contains OS-related configuration.
type osConfig struct {
	// Group is the name of the group which AdGuard Home must switch to on
	// startup.  Empty string means no switching.
	Group string `yaml:"group"`
	// User is the name of the user which AdGuard Home must switch to on
	// startup.  Empty string means no switching.
	User string `yaml:"user"`
	// RlimitNoFile is the maximum number of opened fd's per process.  Zero
	// means use the default value.
	RlimitNoFile uint64 `yaml:"rlimit_nofile"`
}

type clientsConfig struct {
	// Sources defines the set of sources to fetch the runtime clients from.
	Sources *clientSourcesConfig `yaml:"runtime_sources"`
	// Persistent are the configured clients.
	Persistent []*clientObject `yaml:"persistent"`
}

// clientSourceConfig is used to configure where the runtime clients will be
// obtained from.
type clientSourcesConfig struct {
	WHOIS     bool `yaml:"whois"`
	ARP       bool `yaml:"arp"`
	RDNS      bool `yaml:"rdns"`
	DHCP      bool `yaml:"dhcp"`
	HostsFile bool `yaml:"hosts"`
}

// configuration is loaded from YAML.
//
// Field ordering is important, YAML fields better not to be reordered, if it's
// not absolutely necessary.
//
// TODO(d.kolyshev):  Use [configmgr.Config].
type configuration struct {
	// Raw file data to avoid re-reading of configuration file
	// It's reset after config is parsed
	fileData []byte

	// HTTPConfig is the block with http conf.
	HTTPConfig configmgr.HTTPConfig `yaml:"http"`

	// Users are the clients capable for accessing the web interface.
	Users []webUser `yaml:"users"`
	// AuthAttempts is the maximum number of failed login attempts a user
	// can do before being blocked.
	AuthAttempts uint `yaml:"auth_attempts"`
	// AuthBlockMin is the duration, in minutes, of the block of new login
	// attempts after AuthAttempts unsuccessful login attempts.
	AuthBlockMin uint `yaml:"block_auth_min"`
	// ProxyURL is the address of proxy server for the internal HTTP client.
	ProxyURL string `yaml:"http_proxy"`
	// Language is a two-letter ISO 639-1 language code.
	Language string `yaml:"language"`
	// Theme is a UI theme for current user.
	Theme Theme `yaml:"theme"`

	// TODO(a.garipov): Make DNS and the fields below pointers and validate
	// and/or reset on explicit nulling.

	// DNS is a block with DNS configuration params.
	DNS configmgr.DNSConfig `yaml:"dns"`

	// QueryLog is a block with query log configuration params.
	QueryLog configmgr.QueryLogConfig `yaml:"querylog"`

	// Stats is a block with statistics configuration settings.
	Stats configmgr.StatsConfig `yaml:"statistics"`

	// Filters reflects the filters from [filtering.Config].  It's cloned to the
	// config used in the filtering module at the startup.  Afterwards it's
	// cloned from the filtering module back here.
	//
	// TODO(e.burkov):  Move all the filtering configuration fields into the
	// only configuration subsection covering the changes with a single
	// migration.
	Filters          []filtering.FilterYAML `yaml:"filters"`
	WhitelistFilters []filtering.FilterYAML `yaml:"whitelist_filters"`
	UserRules        []string               `yaml:"user_rules"`

	// DHCP is a block with DHCP configuration params.
	DHCP *configmgr.DHCPConfig `yaml:"dhcp"`

	// Filtering is a block with DNS filtering configuration settings.
	Filtering *filtering.Config `yaml:"filtering"`

	// Clients contains the YAML representations of the persistent clients.
	// This field is only used for reading and writing persistent client data.
	// Keep this field sorted to ensure consistent ordering.
	Clients *clientsConfig `yaml:"clients"`

	// Log is a block with log configuration settings.
	Log *configmgr.LogConfig `yaml:"log"`

	OSConfig *osConfig `yaml:"os"`

	sync.RWMutex `yaml:"-"`

	// SchemaVersion is the version of the configuration schema.  See
	// [configmigrate.LastSchemaVersion].
	SchemaVersion uint `yaml:"schema_version"`

	// UnsafeUseCustomUpdateIndexURL is the URL to the custom update index.
	//
	// NOTE: It's only exists for testing purposes and should not be used in
	// release.
	UnsafeUseCustomUpdateIndexURL bool `yaml:"unsafe_use_custom_update_index_url,omitempty"`
}

// Default block host constants.
const (
	defaultSafeBrowsingBlockHost = "standard-block.dns.adguard.com"
	defaultParentalBlockHost     = "family-block.dns.adguard.com"
)

// defaultLocalDomainName is the default domain name used for DHCP hosts.
const defaultLocalDomainName = "lan"

const (
	// defaultDHCPLeaseTTL is the default time-to-live for DHCP leases, in
	// seconds.
	defaultDHCPLeaseTTL = uint32(24 * 60 * 60)

	// defaultDHCPTimeoutICMP is the default timeout for waiting for ICMP
	// responses during DHCP address checks, in milliseconds.
	defaultDHCPTimeoutICMP = uint32(1000)
)

// config is the global configuration structure.
//
// TODO(a.garipov, e.burkov): This global is awful and must be removed.
var config = &configuration{
	AuthAttempts: 5,
	AuthBlockMin: 15,
	HTTPConfig: configmgr.HTTPConfig{
		Pprof: &configmgr.HTTPPprofConfig{
			Port:    6060,
			Enabled: false,
		},
		Address:    netip.AddrPortFrom(netip.IPv4Unspecified(), 3000),
		SessionTTL: timeutil.Duration(30 * timeutil.Day),
	},
	DNS: configmgr.DNSConfig{
		EDNSClientSubnet: &configmgr.EDNSClientSubnet{
			CustomIP: netip.Addr{},
		},
		PendingRequests: &configmgr.PendingRequests{
			Enabled: true,
		},
		UpstreamMode: proxy.UpstreamModeLoadBalance,
		BindHosts:    []netip.Addr{netip.IPv4Unspecified()},
		TrustedProxies: []netutil.Prefix{{
			Prefix: netip.MustParsePrefix("127.0.0.0/8"),
		}, {
			Prefix: netip.MustParsePrefix("::1/128"),
		}},
		CacheOptimisticAnswerTTL: timeutil.Duration(30 * time.Second),
		CacheOptimisticMaxAge:    timeutil.Duration(12 * time.Hour),
		FastestTimeout:           timeutil.Duration(defaultFastestTimeout),
		UpstreamTimeout:          timeutil.Duration(dnsforward.DefaultTimeout),

		// set default maximum concurrent queries to 300
		// we introduced a default limit due to this:
		// https://github.com/AdguardTeam/AdGuardHome/issues/2015#issuecomment-674041912
		// was later increased to 300 due to https://github.com/AdguardTeam/AdGuardHome/issues/2257
		MaxGoroutines: 300,

		RatelimitSubnetLenIPv4: 24,
		RatelimitSubnetLenIPv6: 56,
		CacheSize:              4 * 1024 * 1024,
		Ratelimit:              20,
		Port:                   defaultPortDNS,
		CacheEnabled:           true,
		EnableDNSSEC:           true,
		HostsFileEnabled:       true,
		RefuseAny:              true,
		UsePrivateRDNS:         true,
	},
	QueryLog: configmgr.QueryLogConfig{
		DirPath:        "",
		Ignored:        []string{},
		Interval:       timeutil.Duration(90 * timeutil.Day),
		MemSize:        1000,
		Enabled:        true,
		FileEnabled:    true,
		IgnoredEnabled: false,
	},
	Stats: configmgr.StatsConfig{
		DirPath:        "",
		Ignored:        []string{},
		Interval:       timeutil.Duration(1 * timeutil.Day),
		Enabled:        true,
		IgnoredEnabled: false,
	},
	// NOTE: Keep these parameters in sync with the one put into
	// client/src/helpers/filters/filters.ts by scripts/vetted-filters.
	//
	// TODO(a.garipov): Think of a way to make scripts/vetted-filters update
	// these as well if necessary.
	Filters: []filtering.FilterYAML{{
		ID:      1,
		Enabled: true,
		URL:     "https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt",
		Name:    "AdGuard DNS filter",
	}, {
		ID:      2,
		Enabled: false,
		URL:     "https://adguardteam.github.io/HostlistsRegistry/assets/filter_2.txt",
		Name:    "AdAway Default Blocklist",
	}},
	Filtering: &filtering.Config{
		ProtectionEnabled:  true,
		BlockingMode:       filtering.BlockingModeDefault,
		BlockedResponseTTL: 10, // in seconds

		FilteringEnabled:           true,
		FiltersUpdateIntervalHours: 24,

		RewritesEnabled: true,

		ParentalEnabled:     false,
		SafeBrowsingEnabled: false,

		MaxHTTPSize:           rulelist.DefaultMaxRuleListSize,
		SafeBrowsingCacheSize: 1 * 1024 * 1024,
		ParentalCacheSize:     1 * 1024 * 1024,
		CacheTime:             30,

		ParentalBlockHost:     defaultParentalBlockHost,
		SafeBrowsingBlockHost: defaultSafeBrowsingBlockHost,
	},
	DHCP: &configmgr.DHCPConfig{
		Conf4: &configmgr.DHCPv4Config{
			ICMPTimeout:   defaultDHCPTimeoutICMP,
			LeaseDuration: defaultDHCPLeaseTTL,
		},
		Conf6: &configmgr.DHCPv6Config{
			LeaseDuration: defaultDHCPLeaseTTL,
		},
		InterfaceName:   "",
		LocalDomainName: defaultLocalDomainName,
		Enabled:         false,
	},
	Clients: &clientsConfig{
		Sources: &clientSourcesConfig{
			WHOIS:     true,
			ARP:       true,
			RDNS:      true,
			DHCP:      true,
			HostsFile: true,
		},
	},
	OSConfig:      &osConfig{},
	SchemaVersion: configmigrate.LastSchemaVersion,
	Theme:         ThemeAuto,
}

// configFilePath returns the absolute, symlink-resolved path to the current
// configuration file.  l must not be nil.
//
// TODO(s.chzhen):  Fix the bug where the wrong file may be resolved:
// [filepath.EvalSymlinks] resolves a relative path against the current working
// directory, not workDir.  Make the path absolute relative to workDir before
// calling EvalSymlinks.
func configFilePath(
	ctx context.Context,
	l *slog.Logger,
	workDir string,
	confPath string,
) (resolved string) {
	resolved, err := filepath.EvalSymlinks(confPath)
	if err != nil {
		l.DebugContext(
			ctx,
			"symlink resolve failed; using original path",
			"path", confPath,
			slogutil.KeyError, err,
		)

		resolved = confPath
	}

	if !filepath.IsAbs(confPath) {
		resolved = filepath.Join(workDir, confPath)
	}

	return resolved
}

// validateBindHosts returns error if any of binding hosts from configuration is
// not a valid IP address.
func validateBindHosts(
	ctx context.Context,
	l *slog.Logger,
	conf *configuration,
	fileData []byte,
) (err error) {
	if !conf.HTTPConfig.Address.IsValid() {
		return errors.Error("http.address is not a valid ip address")
	}

	for i, addr := range conf.DNS.BindHosts {
		if !addr.IsValid() {
			logIPHint(ctx, l, fileData)

			return fmt.Errorf("dns.bind_hosts at index %d is not a valid ip address", i)
		}
	}

	return nil
}

// parseConfig loads configuration from the YAML file, upgrading it if
// necessary.  l must not be nil.
func parseConfig(
	ctx context.Context,
	l *slog.Logger,
	workDir string,
	confPath string,
	legacyNoEtcHosts bool,
) (err error) {
	config.fileData, err = readConfigFile(ctx, l, workDir, confPath)
	if err != nil {
		// Don't wrap the error, because it's informative enough as is.
		return err
	}

	migrator := configmigrate.New(&configmigrate.Config{
		Logger:           l.With(slogutil.KeyPrefix, "config_migrator"),
		WorkingDir:       workDir,
		DataDir:          filepath.Join(workDir, dataDir),
		LegacyNoEtcHosts: legacyNoEtcHosts,
	})

	var upgraded bool
	config.fileData, upgraded, err = migrator.Migrate(
		ctx,
		config.fileData,
		configmigrate.LastSchemaVersion,
	)
	if err != nil {
		// Don't wrap the error, because it's informative enough as is.
		return err
	} else if upgraded {
		confPath = configFilePath(ctx, l, workDir, confPath)
		l.DebugContext(ctx, "writing config file after config upgrade", "path", confPath)

		err = maybe.WriteFile(confPath, config.fileData, aghos.DefaultPermFile)
		if err != nil {
			return fmt.Errorf("writing new config: %w", err)
		}
	}

	err = yaml.Unmarshal(config.fileData, &config)
	if err != nil {
		// Don't wrap the error since it's informative enough as is.
		return err
	}

	err = validateConfig(ctx, l, config.fileData)
	if err != nil {
		return err
	}

	if config.DNS.UpstreamTimeout == 0 {
		config.DNS.UpstreamTimeout = timeutil.Duration(dnsforward.DefaultTimeout)
	}

	return nil
}

// logIPHint logs an informational message when the config contains an unquoted
// IP address with a trailing colon.  It's a best-effort check for a YAML
// parsing behavior where a list item is decoded as {key: null}.  l must not be
// nil.
func logIPHint(ctx context.Context, l *slog.Logger, data []byte) {
	var conf struct {
		DNS struct {
			BindHosts []any `yaml:"bind_hosts"`
		} `yaml:"dns"`
	}

	err := yaml.Unmarshal(data, &conf)
	if err != nil {
		// This should not happen since this is already the validation process.
		l.DebugContext(
			ctx,
			"failed to unmarshal config while logging ip hint",
			slogutil.KeyError, err,
		)

		return
	}

	for _, h := range conf.DNS.BindHosts {
		m, ok := h.(map[string]any)
		if !ok {
			continue
		}

		if !hasNilValue(m) {
			continue
		}

		l.WarnContext(ctx, "quote addresses that end with a colon in 'dns.bind_hosts'")

		return
	}
}

// hasNilValue returns true if m contains a nil value.
func hasNilValue(m map[string]any) (ok bool) {
	for _, v := range m {
		if v == nil {
			return true
		}
	}

	return false
}

// validateConfig returns error if the configuration is invalid.  l must not be
// nil.
func validateConfig(ctx context.Context, l *slog.Logger, fileData []byte) (err error) {
	err = validateBindHosts(ctx, l, config, fileData)
	if err != nil {
		// Don't wrap the error since it's informative enough as is.
		return err
	}

	err = validateConfigPorts(config)
	if err != nil {
		// Don't wrap the error since it's informative enough as is.
		return err
	}

	if !filtering.ValidateUpdateIvl(config.Filtering.FiltersUpdateIntervalHours) {
		config.Filtering.FiltersUpdateIntervalHours = 24
	}

	if err = config.DHCP.Validate(); err != nil {
		return fmt.Errorf("validating dhcp config: %w", err)
	}

	if err = config.HTTPConfig.Validate(); err != nil {
		return fmt.Errorf("validating http config: %w", err)
	}

	if len(config.Users) == 0 {
		l.WarnContext(ctx, "no users in the configuration file; authentication is disabled")
	}

	validateLanguage(ctx, l)

	return nil
}

// validateLanguage validates the specified language and clears it with a
// warning if it is unsupported.  l must not be nil.
func validateLanguage(ctx context.Context, l *slog.Logger) {
	if config.Language != "" && !allowedLanguages.Has(config.Language) {
		l.WarnContext(ctx, "unsupported language", "lang", config.Language)

		// Clear the language so the frontend can use the client's browser
		// language.
		config.Language = ""
	}
}

// validateConfigPorts returns an error in case the given conf has conflict
// ports.  conf must not be nil.
func validateConfigPorts(conf *configuration) (err error) {
	tcpPorts := aghalg.UniqChecker[tcpPort]{}
	addPorts(tcpPorts, tcpPort(conf.HTTPConfig.Address.Port()))

	udpPorts := aghalg.UniqChecker[udpPort]{}
	addPorts(udpPorts, udpPort(conf.DNS.Port))

	var errs []error
	if err = tcpPorts.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("validating tcp ports: %w", err))
	}

	if err = udpPorts.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("validating udp ports: %w", err))
	}

	return errors.Join(errs...)
}

// udpPort is the port number for UDP protocol.
type udpPort uint16

// tcpPort is the port number for TCP protocol.
type tcpPort uint16

// addPorts is a helper for ports validation that skips zero ports.
func addPorts[T tcpPort | udpPort](uc aghalg.UniqChecker[T], ports ...T) {
	for _, p := range ports {
		if p != 0 {
			uc.Add(p)
		}
	}
}

// readConfigFile reads configuration file contents.  l must not be nil.
func readConfigFile(
	ctx context.Context,
	l *slog.Logger,
	workDir string,
	confPath string,
) (fileData []byte, err error) {
	if len(config.fileData) > 0 {
		return config.fileData, nil
	}

	confPath = configFilePath(ctx, l, workDir, confPath)
	l.DebugContext(ctx, "reading config file", "path", confPath)

	// Do not wrap the error because it's informative enough as is.
	return os.ReadFile(confPath)
}

// write saves configuration to the YAML file and also saves the user filter
// contents to a file.  l must not be nil.
func (c *configuration) write(
	ctx context.Context,
	l *slog.Logger,
	auth *auth,
	workDir string,
	confPath string,
) (err error) {
	c.Lock()
	defer c.Unlock()

	if auth != nil {
		config.Users = auth.usersList(ctx)
	}

	if globalContext.stats != nil {
		globalContext.stats.WriteDiskConfig(&config.Stats)
	}

	if globalContext.queryLog != nil {
		globalContext.queryLog.WriteDiskConfig(&config.QueryLog, &config.DNS)
	}

	if globalContext.filters != nil {
		globalContext.filters.WriteDiskConfig(config.Filtering)
		config.Filters = config.Filtering.Filters
		config.WhitelistFilters = config.Filtering.WhitelistFilters
		config.UserRules = config.Filtering.UserRules
	}

	if s := globalContext.dnsServer; s != nil {
		s.WriteDiskConfig(&config.DNS)

		addrProcConf := s.AddrProcConfig()
		config.Clients.Sources.RDNS = addrProcConf.UseRDNS
		config.Clients.Sources.WHOIS = addrProcConf.UseWHOIS
	}

	config.Clients.Persistent = globalContext.clients.forConfig()

	confPath = configFilePath(ctx, l, workDir, confPath)
	l.DebugContext(ctx, "writing config file", "path", confPath)

	buf := &bytes.Buffer{}
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)

	err = enc.Encode(config)
	if err != nil {
		return fmt.Errorf("generating config file: %w", err)
	}

	err = maybe.WriteFile(confPath, buf.Bytes(), aghos.DefaultPermFile)
	if err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// defaultConfigModifier is a default [agh.ConfigModifier] implementation.
type defaultConfigModifier struct {
	auth     *auth
	config   *configuration
	logger   *slog.Logger
	workDir  string
	confPath string
}

// newDefaultConfigModifier returns the new properly initialized
// *defaultConfigModifier.  All arguments must not be nil.
//
// TODO(s.chzhen):  Consider using configuration struct.
func newDefaultConfigModifier(
	conf *configuration,
	l *slog.Logger,
	workDir string,
	confPath string,
) (cm *defaultConfigModifier) {
	return &defaultConfigModifier{
		config:   conf,
		logger:   l,
		workDir:  workDir,
		confPath: confPath,
	}
}

// type check
var _ agh.ConfigModifier = (*defaultConfigModifier)(nil)

// Apply implements the [agh.ConfigModifier] interface for
// *defaultConfigModifier.
func (cm *defaultConfigModifier) Apply(ctx context.Context) {
	err := cm.config.write(ctx, cm.logger, cm.auth, cm.workDir, cm.confPath)
	if err != nil {
		cm.logger.ErrorContext(ctx, "writing config", slogutil.KeyError, err)
	}
}

// setAuth sets the auth parameters used by Apply.
func (cm *defaultConfigModifier) setAuth(a *auth) {
	cm.auth = a
}

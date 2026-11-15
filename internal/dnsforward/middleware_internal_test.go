package dnsforward

import (
	"net"
	"testing"
	"time"

	"github.com/AdguardTeam/AdGuardHome/internal/aghtest"
	"github.com/AdguardTeam/AdGuardHome/internal/filtering"
	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/AdguardTeam/golibs/testutil"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Common constants for tests.
const (
	blockedHost = "blockedhost.org"
	testFQDN    = "example.org."

	dnsClientTimeout = 200 * time.Millisecond
)

func TestServer_middlewareUDP(t *testing.T) {
	t.Parallel()

	const (
		clientIPv4 = "127.0.0.1"
		clientIPv6 = "::1"
	)

	clientIPs := []string{clientIPv4, clientIPv6}

	testCases := []struct {
		name              string
		host              string
		allowedClients    []string
		disallowedClients []string
		blockedHosts      []string
		wantTimeout       bool
	}{{
		name:              "allow_all",
		host:              testFQDN,
		allowedClients:    []string{},
		disallowedClients: []string{},
		blockedHosts:      []string{},
		wantTimeout:       false,
	}, {
		name:              "allowed_client_allowed",
		host:              testFQDN,
		allowedClients:    clientIPs,
		disallowedClients: []string{},
		blockedHosts:      []string{},
		wantTimeout:       false,
	}, {
		name:              "allowed_client_rejected",
		host:              testFQDN,
		allowedClients:    []string{"1:2:3::4"},
		disallowedClients: []string{},
		blockedHosts:      []string{},
		wantTimeout:       true,
	}, {
		name:              "disallowed_client_allowed",
		host:              testFQDN,
		allowedClients:    []string{},
		disallowedClients: []string{"1:2:3::4"},
		blockedHosts:      []string{},
		wantTimeout:       false,
	}, {
		name:              "disallowed_client_rejected",
		host:              testFQDN,
		allowedClients:    []string{},
		disallowedClients: clientIPs,
		blockedHosts:      []string{},
		wantTimeout:       true,
	}, {
		name:              "blocked_hosts_allowed",
		host:              testFQDN,
		allowedClients:    []string{},
		disallowedClients: []string{},
		blockedHosts:      []string{blockedHost},
		wantTimeout:       false,
	}, {
		name:              "blocked_hosts_rejected",
		host:              dns.Fqdn(blockedHost),
		allowedClients:    []string{},
		disallowedClients: []string{},
		blockedHosts:      []string{blockedHost},
		wantTimeout:       true,
	}}

	localAns := newTestDNSAnswer(testFQDN, net.IP{1, 2, 3, 4})
	localUpsAddr := newTestUpstream(t, localAns)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := createTestServer(
				t,
				&filtering.Config{
					BlockingMode: filtering.BlockingModeDefault,
				},
				ServerConfig{
					UDPListenAddrs:    []*net.UDPAddr{{}},
					TCPListenAddrs:    []*net.TCPAddr{{}},
					AllowedClients:    tc.allowedClients,
					DisallowedClients: tc.disallowedClients,
					BlockedHosts:      tc.blockedHosts,
					UpstreamDNS:       []string{localUpsAddr},
					UpstreamMode:      proxy.UpstreamModeLoadBalance,
					EDNSClientSubnet:  &EDNSClientSubnet{Enabled: false},
					ClientsContainer:  EmptyClientsContainer{},
				},
			)

			startDeferStop(t, s)

			client := &dns.Client{
				Net:     "udp",
				Timeout: dnsClientTimeout,
			}

			req := createTestMessage(tc.host)
			addr := s.dnsProxy.Addr(proxy.ProtoUDP).String()

			reply, _, err := client.Exchange(req, addr)
			if tc.wantTimeout {
				assertTimeoutError(t, err, reply)
			} else {
				require.NoError(t, err)

				assertSuccessResponse(t, reply, localAns)
			}
		})
	}
}

// newTestDNSAnswer creates a standard A record answer for testing.
func newTestDNSAnswer(fqdn string, ip net.IP) (ans []dns.RR) {
	return []dns.RR{&dns.A{
		Hdr: dns.RR_Header{
			Name:     fqdn,
			Rrtype:   dns.TypeA,
			Class:    dns.ClassINET,
			Ttl:      3600,
			Rdlength: 4,
		},
		A: ip,
	}}
}

// newTestUpstream creates a test upstream handler and returns its address.
func newTestUpstream(tb testing.TB, answer []dns.RR) (addr string) {
	tb.Helper()

	pt := testutil.NewPanicT(tb)

	handler := dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		resp := (&dns.Msg{}).SetReply(req)
		resp.Answer = answer

		require.NoError(pt, w.WriteMsg(resp))
	})

	return aghtest.StartLocalhostUpstream(tb, handler).String()
}

// assertSuccessResponse checks that the response is successful with expected
// answer.
func assertSuccessResponse(tb testing.TB, reply *dns.Msg, expectedAns []dns.RR) {
	tb.Helper()

	require.NotNil(tb, reply)

	assert.Equal(tb, dns.RcodeSuccess, reply.Rcode)
	assert.Equal(tb, expectedAns, reply.Answer)
}

// assertRejectedResponse checks that the response has the expected error code
// and no answer.
func assertRejectedResponse(tb testing.TB, reply *dns.Msg, wantRCode int) {
	tb.Helper()

	require.NotNil(tb, reply)

	assert.Equal(tb, wantRCode, reply.Rcode)
	assert.Empty(tb, reply.Answer)
}

// assertTimeoutError checks that the error is a timeout error and reply is nil.
func assertTimeoutError(tb testing.TB, err error, reply *dns.Msg) {
	tb.Helper()

	wantErr := &net.OpError{}
	require.ErrorAs(tb, err, &wantErr)

	assert.True(tb, wantErr.Timeout())
	assert.Nil(tb, reply)
}

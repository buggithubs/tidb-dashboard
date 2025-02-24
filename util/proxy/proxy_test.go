// Copyright 2021 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"

	"github.com/pingcap/tidb-dashboard/util/testutil"
)

var configForTest = Config{
	UpstreamProbeInterval: time.Millisecond * 200,
}

const probeWait = time.Millisecond * 500 // UpstreamProbeInterval*2.5

func sendGetToProxy(proxy *Proxy) (*resty.Response, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d", proxy.Port())
	return resty.New().SetTimeout(time.Millisecond * 500).R().Get(url)
}

func TestNoUpstream(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
}

func TestAddUpstream(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	server := testutil.NewHTTPServer("foo")
	defer server.Close()

	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server)})
	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	// Incoming connection will not be established until a probe interval.
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())

	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "foo", resp.String())
}

func TestAddMultipleUpstream(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	servers := testutil.NewMultiServer(5, "foo#%d")
	defer servers.CloseAll()

	assert.False(t, p.HasActiveUpstream())
	p.SetUpstreams(servers.GetEndpoints())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())

	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Contains(t, resp.String(), "foo#")
	assert.Equal(t, servers.LastResp(), resp.String())
}

func TestRemoveAllUpstreams(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	server := testutil.NewHTTPServer("foo")
	defer server.Close()

	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server)})
	assert.False(t, p.HasActiveUpstream())
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "foo", resp.String())

	p.SetUpstreams([]string{})
	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
}

func TestRemoveOneUpstream(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	server1 := testutil.NewHTTPServer("foo")
	defer server1.Close()

	server2 := testutil.NewHTTPServer("bar")
	defer server2.Close()

	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server1)})
	assert.False(t, p.HasActiveUpstream())
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "foo", resp.String())

	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server1), testutil.GetHTTPServerHost(server2)})
	assert.True(t, p.HasActiveUpstream())
	time.Sleep(probeWait)
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "foo", resp.String())
	assert.True(t, p.HasActiveUpstream())

	// The active upstream is removed, another upstream should be used.
	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server2)})
	assert.True(t, p.HasActiveUpstream())
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "bar", resp.String())

	// Add upstream back
	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server1), testutil.GetHTTPServerHost(server2)})
	assert.True(t, p.HasActiveUpstream())
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "bar", resp.String())

	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "bar", resp.String())
}

func TestPickLastActiveUpstream(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	server1 := testutil.NewHTTPServer("foo")
	defer server1.Close()

	server2 := testutil.NewHTTPServer("bar")
	defer server2.Close()

	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server1)})
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "foo", resp.String())

	// Even if SetUpstreams is called, the active upstream should be unchanged.
	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server1), testutil.GetHTTPServerHost(server2)})
	assert.True(t, p.HasActiveUpstream())
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "foo", resp.String())

	time.Sleep(probeWait)
	for i := 0; i < 5; i++ {
		// Let's try multiple times! We should always get "foo".
		assert.True(t, p.HasActiveUpstream())
		resp, err = sendGetToProxy(p)
		assert.Nil(t, err)
		assert.Equal(t, "foo", resp.String())
	}
}

func TestAllUpstreamDown(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	servers := testutil.NewMultiServer(3, "foo#%d")
	defer servers.CloseAll()

	p.SetUpstreams(servers.GetEndpoints())
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Contains(t, resp.String(), "foo#")
	assert.Equal(t, servers.LastResp(), resp.String())

	servers.CloseAll()
	assert.True(t, p.HasActiveUpstream())

	time.Sleep(probeWait)
	// Since we only set inactive when new connection is established (lazily), HasActiveUpstream is still true here.
	assert.True(t, p.HasActiveUpstream())

	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())
}

func TestActiveUpstreamDown(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	servers := testutil.NewMultiServer(5, "foo#%d")
	defer servers.CloseAll()

	p.SetUpstreams(servers.GetEndpoints())
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Contains(t, resp.String(), "foo#")
	assert.Equal(t, servers.LastResp(), resp.String())
	assert.Equal(t, fmt.Sprintf("foo#%d", servers.LastId()), resp.String())

	// Close the last accessed server
	servers.Servers[servers.LastId()].Close()

	// The connection is still succeeded, but forwarded to another upstream.
	resp2, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Contains(t, resp2.String(), "foo#")
	assert.Equal(t, servers.LastResp(), resp2.String())
	assert.NotEqual(t, resp.String(), resp2.String()) // Check upstream has changed

	time.Sleep(probeWait)
	resp3, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, resp3.String(), resp2.String()) // Unchanged
}

func TestNonActiveUpstreamDown(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	servers := testutil.NewMultiServer(5, "foo#%d")
	defer servers.CloseAll()

	p.SetUpstreams(servers.GetEndpoints())
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("foo#%d", servers.LastId()), resp.String())

	// Close other non active servers
	for i := 0; i < 5; i++ {
		if i != servers.LastId() {
			servers.Servers[i].Close()
		}
	}

	resp2, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, resp.String(), resp2.String()) // Unchanged

	time.Sleep(probeWait)
	resp3, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, resp.String(), resp3.String()) // Unchanged
}

func TestBrokenServer(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "foo")
	}))
	defer server.Close()

	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server)})
	assert.False(t, p.HasActiveUpstream())

	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())

	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.True(t, os.IsTimeout(err))
	assert.True(t, p.HasActiveUpstream())

	// In this case, proxy will not switch the upstream by design. Let's check it is still
	// connecting the original "broken" upstream.
	server2 := testutil.NewHTTPServer("foo")
	defer server2.Close()

	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server), testutil.GetHTTPServerHost(server2)})
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())

	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.True(t, os.IsTimeout(err))

	// Let's remove the first upstream! We should get success response immediately without waiting probe.
	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server2)})
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "foo", resp.String())
}

func TestUpstreamBack(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	server := testutil.NewHTTPServer("foo")
	defer server.Close()
	host := testutil.GetHTTPServerHost(server)

	p.SetUpstreams([]string{host})
	assert.False(t, p.HasActiveUpstream())
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "foo", resp.String())

	// Close the upstream server
	server.Close()
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())

	// Start the upstream server again at the original listen address
	server2 := testutil.NewHTTPServerAtHost("bar", host)
	defer server2.Close()
	// We will still get failure here, even if the upstream is back. It will recover at next probe round.
	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())

	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "bar", resp.String())
}

func TestUpstreamSwitchComplex(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	server := testutil.NewHTTPServer("foo")
	defer server.Close()

	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server)})
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "foo", resp.String())

	server2 := testutil.NewHTTPServer("bar")
	defer server2.Close()

	// Let's close the current upstream
	server.Close()
	assert.True(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())

	// Wait one round probe, nothing is changed
	time.Sleep(probeWait)
	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)

	// Add a new alive upstream
	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server), testutil.GetHTTPServerHost(server2)})
	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)

	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "bar", resp.String())

	// Bring down the new upstream again!
	server2.Close()

	assert.True(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)

	server3 := testutil.NewHTTPServer("box")
	defer server3.Close()
	host3 := testutil.GetHTTPServerHost(server3)

	// Add a new alive upstream
	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server), testutil.GetHTTPServerHost(server2), host3})
	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)

	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "box", resp.String())

	server3.Close()

	server4 := testutil.NewHTTPServer("car")
	host4 := testutil.GetHTTPServerHost(server4)
	server4.Close()

	// Add a bad upstream
	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server), testutil.GetHTTPServerHost(server2), host3, host4})
	assert.True(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())

	time.Sleep(probeWait)
	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)

	// Bring back server3
	server3New := testutil.NewHTTPServerAtHost("newBox", host3)
	defer server3New.Close()

	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)

	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "newBox", resp.String())

	// Remove server3
	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server), testutil.GetHTTPServerHost(server2), host4})
	assert.True(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())
	time.Sleep(probeWait)
	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)

	// Remove server4
	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server), testutil.GetHTTPServerHost(server2)})
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())

	// Start server4 again, nothing should be changed (keep failure).
	server4New := testutil.NewHTTPServerAtHost("newCar", host4)
	defer server4New.Close()

	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())
	time.Sleep(probeWait)
	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)

	// Add server3 back to the upstream
	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server2), host3})
	assert.False(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "newBox", resp.String())
	assert.True(t, p.HasActiveUpstream())

	// Change upstream to host4
	p.SetUpstreams([]string{host4})
	assert.True(t, p.HasActiveUpstream())
	_, err = sendGetToProxy(p) // At this time, active upstream is host3, and host4 is not recognized as alive, so it should fail
	assert.NotNil(t, err)
	assert.False(t, p.HasActiveUpstream())

	time.Sleep(probeWait)
	resp, err = sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "newCar", resp.String())
	assert.True(t, p.HasActiveUpstream())
}

func TestClose(t *testing.T) {
	p, err := New(configForTest)
	assert.Nil(t, err)
	defer p.Close()

	server := testutil.NewHTTPServer("foo")
	defer server.Close()

	p.SetUpstreams([]string{testutil.GetHTTPServerHost(server)})
	time.Sleep(probeWait)
	assert.True(t, p.HasActiveUpstream())
	resp, err := sendGetToProxy(p)
	assert.Nil(t, err)
	assert.Equal(t, "foo", resp.String())

	p.Close()
	assert.True(t, p.HasActiveUpstream()) // TODO: Should we fix this behaviour?
	_, err = sendGetToProxy(p)
	assert.NotNil(t, err)

	p.Close() // Close again should be fine!
}

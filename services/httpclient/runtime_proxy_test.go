package httpclient

import (
	"github.com/Yuelioi/yueling-go/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyChangeWithoutReplacingClient(t *testing.T) {
	old := config.Current()
	defer config.Publish(old)
	proxy := func(value string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(value)) }))
	}
	first := proxy("first")
	defer first.Close()
	second := proxy("second")
	defer second.Close()
	client := &Client{Client: &http.Client{Transport: &runtimeProxyTransport{}}}
	for _, v := range []struct{ address, want string }{{first.URL, "first"}, {second.URL, "second"}} {
		next := config.Current()
		next.Tools.Proxy = v.address
		config.Publish(next)
		body, err := client.GetBytes("http://example.invalid/resource")
		if err != nil || string(body) != v.want {
			t.Fatalf("proxy change ignored: %s %v", body, err)
		}
	}
}

package tools

import (
	"errors"
	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/services/httpclient"
	"io"
	"net/http"
	"strings"
	"testing"
)

type weatherFixtureTransport func(*http.Request) (*http.Response, error)

func (f weatherFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestWeatherFailureDoesNotExposeProviderDiagnostic(t *testing.T) {
	old, client := config.Current(), httpclient.Direct
	t.Cleanup(func() { config.Publish(old); httpclient.Direct = client })
	c := old
	c.Tools.QWeatherHost = "fixture.invalid"
	c.Tools.QWeatherKey = "fixture-key"
	config.Publish(c)
	httpclient.Direct = &httpclient.Client{Client: &http.Client{Transport: weatherFixtureTransport(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "lookup") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":"200","location":[{"id":"1","name":"杭州"}]}`)), Header: make(http.Header)}, nil
		}
		return nil, errors.New("private-provider-diagnostic")
	})}}
	content, err := testTools.weatherHandler(&ai.ToolContext{Params: map[string]any{"city": "杭州"}})
	if err == nil || strings.Contains(content, "private-provider") || strings.Contains(content, "fixture.invalid") {
		t.Fatalf("failure not isolated: %q error=%t", content, err != nil)
	}
	for _, name := range []string{"get_weather", "get_chat_history", "resolve_user_by_name", "get_group_members"} {
		tool, _ := testEngine.Registry.GetTool(name)
		if !tool.ReadOnly {
			t.Errorf("%s query marked as possible write", name)
		}
	}
}

func TestWeatherLocationOutageIsNotAnUnknownCity(t *testing.T) {
	old, client := config.Current(), httpclient.Direct
	t.Cleanup(func() { config.Publish(old); httpclient.Direct = client })
	c := old
	c.Tools.QWeatherHost = "fixture.invalid"
	c.Tools.QWeatherKey = "fixture"
	config.Publish(c)
	httpclient.Direct = &httpclient.Client{Client: &http.Client{Transport: weatherFixtureTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("private-location-outage") })}}
	content, err := testTools.weatherHandler(&ai.ToolContext{Params: map[string]any{"city": "杭州"}})
	if err == nil || strings.Contains(content, "找不到城市") || strings.Contains(content, "private-") {
		t.Fatalf("outage misreported: %q error=%t", content, err != nil)
	}
}

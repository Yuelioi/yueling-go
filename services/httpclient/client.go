package httpclient

import (
	"net/http"
	"net/url"
	"time"

	"github.com/Yuelioi/yueling-go/config"
)

var (
	Direct = &http.Client{Timeout: 10 * time.Second}
	Proxy  *http.Client
)

func init() {
	Proxy = Direct
}

func InitProxy() {
	addr := config.C.Tools.Proxy
	if addr == "" {
		return
	}
	u, err := url.Parse(addr)
	if err != nil {
		return
	}
	Proxy = &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(u)},
		Timeout:   15 * time.Second,
	}
}

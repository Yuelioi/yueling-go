package tools

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/Yuelioi/yueling-go/ai"
	"github.com/Yuelioi/yueling-go/config"
	"github.com/Yuelioi/yueling-go/db"
	"github.com/Yuelioi/yueling-go/services/httpclient"
)

func init() {
	ai.Register(ai.ToolMeta{
		Name:        "get_weather",
		Description: "查询指定城市的实时天气和未来3天预报",
		Tags:        []string{"天气", "weather", "查询"},
		Triggers:    []string{"天气", "weather", "气温", "下雨", "温度", "预报"},
		Slots:       []string{"city"},
		Params: []ai.Param{
			{Name: "city", Type: "string", Description: "城市名称，如【北京】【上海】【成都】；若用户已设置位置可留空", Required: false},
		},
		Handler: weatherHandler,
	})
}

func weatherHandler(ctx *ai.ToolContext) (string, error) {
	city := ctx.String("city")
	if city == "" {
		if loc, ok := db.GetUserProfile(ctx.UserID(), "位置"); ok && loc != "" {
			city = loc
		}
	}
	if city == "" {
		return "请告诉我查哪个城市的天气，或先用「我在 城市名」设置你的位置。", nil
	}

	cfg := config.C.Tools
	if cfg.QWeatherKey == "" || cfg.QWeatherHost == "" {
		return "天气功能未配置", nil
	}

	locID, locName, err := lookupLocation(city, cfg.QWeatherHost, cfg.QWeatherKey)
	if err != nil {
		return fmt.Sprintf("找不到城市【%s】，请确认城市名", city), nil
	}

	now, err := fetchWeatherNow(locID, cfg.QWeatherHost, cfg.QWeatherKey)
	if err != nil {
		return "天气查询失败: " + err.Error(), nil
	}

	forecast, _ := fetchWeather3d(locID, cfg.QWeatherHost, cfg.QWeatherKey)

	result := fmt.Sprintf("%s 实时天气\n温度 %s°C（体感 %s°C）| %s\n%s %s级 | 湿度 %s%% | 能见度 %skm",
		locName, now.Temp, now.FeelsLike, now.Text,
		now.WindDir, now.WindScale, now.Humidity, now.Vis,
	)
	if forecast != "" {
		result += "\n\n" + forecast
	}
	return result, nil
}

func qwGet(host, path, key string) ([]byte, error) {
	u := fmt.Sprintf("https://%s%s", host, path)
	return httpclient.Direct.GetBytes(u, "X-QW-Api-Key", key)
}

// ---- GeoAPI ----

func lookupLocation(city, host, key string) (id, name string, err error) {
	path := fmt.Sprintf("/geo/v2/city/lookup?location=%s", url.QueryEscape(city))
	body, err := qwGet(host, path, key)
	if err != nil {
		return "", "", err
	}
	var gr struct {
		Code     string `json:"code"`
		Location []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Adm1 string `json:"adm1"`
		} `json:"location"`
	}
	if err := json.Unmarshal(body, &gr); err != nil || gr.Code != "200" || len(gr.Location) == 0 {
		return "", "", fmt.Errorf("city not found")
	}
	loc := gr.Location[0]
	return loc.ID, loc.Name + " " + loc.Adm1, nil
}

// ---- Weather Now ----

type nowData struct {
	Temp      string `json:"temp"`
	FeelsLike string `json:"feelsLike"`
	Text      string `json:"text"`
	WindDir   string `json:"windDir"`
	WindScale string `json:"windScale"`
	Humidity  string `json:"humidity"`
	Vis       string `json:"vis"`
}

func fetchWeatherNow(locID, host, key string) (*nowData, error) {
	body, err := qwGet(host, "/v7/weather/now?location="+locID, key)
	if err != nil {
		return nil, err
	}
	var r struct {
		Code string  `json:"code"`
		Now  nowData `json:"now"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Code != "200" {
		return nil, fmt.Errorf("api error: %s", r.Code)
	}
	return &r.Now, nil
}

// ---- 3-day forecast ----

func fetchWeather3d(locID, host, key string) (string, error) {
	body, err := qwGet(host, "/v7/weather/3d?location="+locID, key)
	if err != nil {
		return "", err
	}
	var r struct {
		Code  string `json:"code"`
		Daily []struct {
			FxDate  string `json:"fxDate"`
			TempMax string `json:"tempMax"`
			TempMin string `json:"tempMin"`
			TextDay string `json:"textDay"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Code != "200" {
		return "", fmt.Errorf("forecast error")
	}
	result := "未来3天："
	for _, d := range r.Daily {
		result += fmt.Sprintf("\n  %s %s %s~%s°C", d.FxDate, d.TextDay, d.TempMin, d.TempMax)
	}
	return result, nil
}

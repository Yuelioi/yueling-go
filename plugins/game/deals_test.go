package game

import (
	"strings"
	"testing"
	"time"
)

func TestParseEpicFreeGamesFiltersPermanentFreeAndDLC(t *testing.T) {
	payload := []byte(`{
  "data":{"Catalog":{"searchStore":{"elements":[
    {"title":"本周游戏","offerType":"BASE_GAME","productSlug":"weekly-game","price":{"totalPrice":{"originalPrice":6800}},"promotions":{"promotionalOffers":[{"promotionalOffers":[{"startDate":"2026-08-13T15:00:00Z","endDate":"2026-08-20T15:00:00Z","discountSetting":{"discountPercentage":0}}]}]}},
    {"title":"下周游戏","offerType":"BASE_GAME","price":{"totalPrice":{"originalPrice":9800}},"offerMappings":[{"pageSlug":"next-game"}],"promotions":{"upcomingPromotionalOffers":[{"promotionalOffers":[{"startDate":"2026-08-20T15:00:00Z","endDate":"2026-08-27T15:00:00Z","discountSetting":{"discountPercentage":0}}]}]}},
    {"title":"永久免费","offerType":"BASE_GAME","price":{"totalPrice":{"originalPrice":0}},"promotions":{"promotionalOffers":[{"promotionalOffers":[{"startDate":"2026-08-13T15:00:00Z","endDate":"2026-08-20T15:00:00Z","discountSetting":{"discountPercentage":0}}]}]}},
    {"title":"免费DLC","offerType":"ADD_ON","price":{"totalPrice":{"originalPrice":2000}},"promotions":{"promotionalOffers":[{"promotionalOffers":[{"startDate":"2026-08-13T15:00:00Z","endDate":"2026-08-20T15:00:00Z","discountSetting":{"discountPercentage":0}}]}]}}
  ]}}}
}`)
	now := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	current, upcoming, err := parseEpicFreeGames(payload, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(current) != 1 || current[0].Title != "本周游戏" || !strings.Contains(current[0].URL, "/p/weekly-game") {
		t.Fatalf("current = %+v", current)
	}
	if len(upcoming) != 1 || upcoming[0].Title != "下周游戏" || !strings.Contains(upcoming[0].URL, "/p/next-game") {
		t.Fatalf("upcoming = %+v", upcoming)
	}
}

func TestParseSteamSearchAndDetails(t *testing.T) {
	appid, err := parseSteamSearch([]byte(`{"total":1,"items":[{"id":1245620,"name":"ELDEN RING"}]}`))
	if err != nil || appid != 1245620 {
		t.Fatalf("appid=%d err=%v", appid, err)
	}
	details := []byte(`{"1245620":{"success":true,"data":{"name":"艾尔登法环","header_image":"https://cdn.example/cover.jpg","is_free":false,"release_date":{"coming_soon":false,"date":"2022 年 2 月 25 日"},"price_overview":{"currency":"CNY","initial":29800,"final":17880,"discount_percent":40}}}}`)
	game, err := parseSteamDetails(details, appid)
	if err != nil {
		t.Fatal(err)
	}
	if game.Name != "艾尔登法环" || !game.HasPrice || game.CurrentPrice != 17880 || game.DiscountPercent != 40 {
		t.Fatalf("game = %+v", game)
	}
}

func TestApplySteamHistoryUsesLowestInfoAndFormatsResult(t *testing.T) {
	game := steamGame{AppID: 1245620, Name: "艾尔登法环", InitialPrice: 29800, CurrentPrice: 17880, DiscountPercent: 40, Currency: "CNY", HasPrice: true}
	applySteamHistory(&game, []byte(`{"status":"ok","result":{"lowest_info":{"price":"119.20","date":"1754006400","discount":60},"lowest_info_v2":{"currency":"CNY"},"prices":[]}}`))
	if !game.HasLowest || game.LowestPrice != 119.2 || game.LowestDiscount != 60 || game.LowestDate == "" {
		t.Fatalf("game history = %+v", game)
	}
	text := formatSteamGame(game)
	for _, want := range []string{"艾尔登法环", "¥178.80", "-40%", "参考史低：¥119.20", "-60%", "1245620"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted %q missing %q", text, want)
		}
	}
}

func TestApplySteamHistoryFallsBackToPricePoints(t *testing.T) {
	game := steamGame{}
	applySteamHistory(&game, []byte(`{"status":"ok","result":{"prices":[{"price":90,"date":"2026-01-01","discount":50,"currency":"CNY"},{"price":80,"date":"2026-02-01","discount":60,"currency":"CNY"}]}}`))
	if !game.HasLowest || game.LowestPrice != 80 || game.LowestDate != "2026-02-01" || game.LowestDiscount != 60 {
		t.Fatalf("fallback history = %+v", game)
	}
}

func TestFormatEpicDeals(t *testing.T) {
	text := formatEpicDeals(
		[]epicDeal{{Title: "本周游戏", URL: "https://example/current", EndsAt: time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)}},
		[]epicDeal{{Title: "下周游戏", StartsAt: time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)}},
	)
	for _, want := range []string{"Epic 本周限免", "本周游戏", "https://example/current", "下期预告", "下周游戏"} {
		if !strings.Contains(text, want) {
			t.Fatalf("formatted %q missing %q", text, want)
		}
	}
}

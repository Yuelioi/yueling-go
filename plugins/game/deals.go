package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/plugins/catalog"
	"github.com/Yuelioi/yueling-go/services/httpclient"
)

const (
	epicFreeGamesURL  = "https://store-site-backend-static.ak.epicgames.com/freeGamesPromotions"
	steamSearchURL    = "https://store.steampowered.com/api/storesearch/"
	steamDetailsURL   = "https://store.steampowered.com/api/appdetails"
	hexboxHistoryURL  = "https://api.xiaoheihe.cn/game/get_game_prices/history/v2"
	dealResponseLimit = 12 << 20
)

var steamAppPattern = regexp.MustCompile(`(?i)(?:store\.steampowered\.com/app/)?(\d{4,10})`)
var steamStoreLinkPattern = regexp.MustCompile(`(?i)https?://store\.steampowered\.com/app/\d{4,10}(?:/[^\s]*)?`)

type epicDeal struct {
	Title    string
	URL      string
	StartsAt time.Time
	EndsAt   time.Time
}

type epicGameRecord struct {
	Title       string `json:"title"`
	OfferType   string `json:"offerType"`
	ProductSlug string `json:"productSlug"`
	Price       struct {
		TotalPrice struct {
			OriginalPrice int `json:"originalPrice"`
		} `json:"totalPrice"`
	} `json:"price"`
	Promotions struct {
		PromotionalOffers         []epicPromotionGroup `json:"promotionalOffers"`
		UpcomingPromotionalOffers []epicPromotionGroup `json:"upcomingPromotionalOffers"`
	} `json:"promotions"`
	OfferMappings []struct {
		PageSlug string `json:"pageSlug"`
	} `json:"offerMappings"`
	CatalogNS struct {
		Mappings []struct {
			PageSlug string `json:"pageSlug"`
		} `json:"mappings"`
	} `json:"catalogNs"`
}

type epicPromotionGroup struct {
	PromotionalOffers []struct {
		StartDate       string `json:"startDate"`
		EndDate         string `json:"endDate"`
		DiscountSetting struct {
			DiscountPercentage int `json:"discountPercentage"`
		} `json:"discountSetting"`
	} `json:"promotionalOffers"`
}

type steamGame struct {
	AppID           int
	Name            string
	HeaderImage     string
	IsFree          bool
	ComingSoon      bool
	ReleaseDate     string
	InitialPrice    int
	CurrentPrice    int
	DiscountPercent int
	Currency        string
	HasPrice        bool
	LowestPrice     float64
	LowestDiscount  int
	LowestDate      string
	LowestCurrency  string
	HasLowest       bool
}

func RegisterDeals(b *bot.Bot) {
	b.OnCommand("限免", "Epic限免", "epic限免", "喜加一").
		Plugin(catalog.PluginGameDeals).
		Handle(func(ctx *bot.CommandContext) error {
			ctx.React(bot.EmojiProcessing)
			current, upcoming, err := fetchEpicFreeGames(bot.Now())
			if err != nil {
				return ctx.Reply("Epic 限免读取失败，稍后再试。")
			}
			return ctx.Reply(formatEpicDeals(current, upcoming))
		})

	b.OnCommand("史低", "查价", "Steam", "steam").
		Plugin(catalog.PluginGameDeals).
		Handle(func(ctx *bot.CommandContext) error {
			query := strings.TrimSpace(strings.Join(ctx.Args, " "))
			if query == "" {
				return ctx.Reply("用法：史低 <游戏名 / Steam 链接 / appid>")
			}
			if utf8.RuneCountInString(query) > 100 {
				return ctx.Reply("游戏名太长了。")
			}
			ctx.React(bot.EmojiProcessing)
			game, err := fetchSteamGame(query)
			if err != nil {
				return ctx.Reply("没查到这个游戏，试试 Steam 商店里的完整名称或 appid。")
			}
			return sendSteamGame(ctx.GroupContext, game)
		})

	// Steam links are commonly pasted directly into chat. Parse them without
	// requiring a second command; a preceding explicit command wins to avoid a
	// duplicate response for "史低 <Steam URL>".
	b.OnRegex(`(?i)https?://store\.steampowered\.com/app/\d{4,10}(?:/[^\s]*)?`).
		Plugin(catalog.PluginGameDeals).
		Priority(5).
		Handle(func(ctx *bot.GroupContext) error {
			if ctx.CommandMatched() {
				return nil
			}
			link := steamStoreLinkPattern.FindString(ctx.Text())
			if link == "" {
				return nil
			}
			ctx.React(bot.EmojiProcessing)
			game, err := fetchSteamGame(link)
			if err != nil {
				return nil
			}
			return sendSteamGame(ctx, game)
		})
}

func sendSteamGame(ctx *bot.GroupContext, game steamGame) error {
	msg := bot.Msg()
	if cover := fetchDealCover(game.HeaderImage); len(cover) > 0 {
		msg.ImageBytes(cover)
	}
	msg.Text(formatSteamGame(game))
	return ctx.SendMsg(msg.Build())
}

func fetchEpicFreeGames(now time.Time) ([]epicDeal, []epicDeal, error) {
	params := url.Values{"locale": {"zh-CN"}, "country": {"CN"}, "allowCountries": {"CN"}}
	data, err := dealGet(epicFreeGamesURL + "?" + params.Encode())
	if err != nil {
		return nil, nil, err
	}
	return parseEpicFreeGames(data, now)
}

func parseEpicFreeGames(data []byte, now time.Time) ([]epicDeal, []epicDeal, error) {
	var payload struct {
		Data struct {
			Catalog struct {
				SearchStore struct {
					Elements []epicGameRecord `json:"elements"`
				} `json:"searchStore"`
			} `json:"Catalog"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, nil, err
	}
	var current, upcoming []epicDeal
	for _, record := range payload.Data.Catalog.SearchStore.Elements {
		// Permanently free titles and DLC are not weekly giveaways.
		if record.Price.TotalPrice.OriginalPrice <= 0 || (record.OfferType != "" && record.OfferType != "BASE_GAME") {
			continue
		}
		for _, group := range record.Promotions.PromotionalOffers {
			for _, promotion := range group.PromotionalOffers {
				deal, ok := epicDealFromPromotion(record, promotion.StartDate, promotion.EndDate)
				if ok && promotion.DiscountSetting.DiscountPercentage == 0 && !now.Before(deal.StartsAt) && now.Before(deal.EndsAt) {
					current = appendUniqueEpicDeal(current, deal)
				}
			}
		}
		for _, group := range record.Promotions.UpcomingPromotionalOffers {
			for _, promotion := range group.PromotionalOffers {
				deal, ok := epicDealFromPromotion(record, promotion.StartDate, promotion.EndDate)
				if ok && promotion.DiscountSetting.DiscountPercentage == 0 && now.Before(deal.StartsAt) {
					upcoming = appendUniqueEpicDeal(upcoming, deal)
				}
			}
		}
	}
	return current, upcoming, nil
}

func epicDealFromPromotion(record epicGameRecord, startText, endText string) (epicDeal, bool) {
	start, err1 := time.Parse(time.RFC3339, startText)
	end, err2 := time.Parse(time.RFC3339, endText)
	if err1 != nil || err2 != nil || strings.TrimSpace(record.Title) == "" {
		return epicDeal{}, false
	}
	slug := ""
	if len(record.OfferMappings) > 0 {
		slug = record.OfferMappings[0].PageSlug
	}
	if slug == "" && len(record.CatalogNS.Mappings) > 0 {
		slug = record.CatalogNS.Mappings[0].PageSlug
	}
	if slug == "" {
		slug = record.ProductSlug
	}
	link := "https://store.epicgames.com/zh-CN/free-games"
	if slug != "" && !strings.Contains(slug, "/") {
		link = "https://store.epicgames.com/zh-CN/p/" + url.PathEscape(slug)
	}
	return epicDeal{Title: strings.TrimSpace(record.Title), URL: link, StartsAt: start, EndsAt: end}, true
}

func appendUniqueEpicDeal(rows []epicDeal, next epicDeal) []epicDeal {
	for _, row := range rows {
		if row.Title == next.Title {
			return rows
		}
	}
	return append(rows, next)
}

func formatEpicDeals(current, upcoming []epicDeal) string {
	var sb strings.Builder
	sb.WriteString("🎁 Epic 本周限免")
	if len(current) == 0 {
		sb.WriteString("\n暂时没有读到正在赠送的游戏。")
	} else {
		for i, deal := range current {
			fmt.Fprintf(&sb, "\n\n%d. %s\n领取至 %s\n%s", i+1, deal.Title, formatDealTime(deal.EndsAt), deal.URL)
		}
	}
	if len(upcoming) > 0 {
		sb.WriteString("\n\n下期预告")
		for i, deal := range upcoming {
			if i >= 4 {
				break
			}
			fmt.Fprintf(&sb, "\n· %s（%s 开始）", deal.Title, formatDealTime(deal.StartsAt))
		}
	}
	return sb.String()
}

func formatDealTime(value time.Time) string {
	return value.In(bot.Now().Location()).Format("1月2日 15:04")
}

func fetchSteamGame(query string) (steamGame, error) {
	appid, err := resolveSteamAppID(query)
	if err != nil {
		return steamGame{}, err
	}
	detailsParams := url.Values{"appids": {strconv.Itoa(appid)}, "cc": {"cn"}, "l": {"schinese"}}
	detailsData, err := dealGet(steamDetailsURL + "?" + detailsParams.Encode())
	if err != nil {
		return steamGame{}, err
	}
	game, err := parseSteamDetails(detailsData, appid)
	if err != nil {
		return steamGame{}, err
	}
	// The history endpoint is case-sensitive: lower-case cn returns data while
	// upper-case CN currently returns an empty result.
	historyParams := url.Values{"appid": {strconv.Itoa(appid)}, "platf": {"steam"}, "cc": {"cn"}, "days": {"720"}}
	if historyData, historyErr := dealGet(hexboxHistoryURL+"?"+historyParams.Encode(), "Referer", fmt.Sprintf("https://www.xiaoheihe.cn/app/topic/game/pc/%d", appid)); historyErr == nil {
		applySteamHistory(&game, historyData)
	}
	return game, nil
}

func resolveSteamAppID(query string) (int, error) {
	if match := steamAppPattern.FindStringSubmatch(strings.TrimSpace(query)); match != nil && (strings.Contains(query, "steampowered.com/app/") || match[0] == strings.TrimSpace(query)) {
		return strconv.Atoi(match[1])
	}
	params := url.Values{"term": {query}, "cc": {"cn"}, "l": {"schinese"}}
	data, err := dealGet(steamSearchURL + "?" + params.Encode())
	if err != nil {
		return 0, err
	}
	return parseSteamSearch(data)
}

func parseSteamSearch(data []byte) (int, error) {
	var payload struct {
		Items []struct {
			ID int `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || len(payload.Items) == 0 || payload.Items[0].ID == 0 {
		return 0, fmt.Errorf("not found")
	}
	return payload.Items[0].ID, nil
}

func parseSteamDetails(data []byte, appid int) (steamGame, error) {
	var payload map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			Name        string `json:"name"`
			HeaderImage string `json:"header_image"`
			IsFree      bool   `json:"is_free"`
			ReleaseDate struct {
				ComingSoon bool   `json:"coming_soon"`
				Date       string `json:"date"`
			} `json:"release_date"`
			PriceOverview struct {
				Currency        string `json:"currency"`
				Initial         int    `json:"initial"`
				Final           int    `json:"final"`
				DiscountPercent int    `json:"discount_percent"`
			} `json:"price_overview"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return steamGame{}, err
	}
	entry, ok := payload[strconv.Itoa(appid)]
	if !ok || !entry.Success || entry.Data.Name == "" {
		return steamGame{}, fmt.Errorf("not found")
	}
	p := entry.Data.PriceOverview
	return steamGame{
		AppID:           appid,
		Name:            entry.Data.Name,
		HeaderImage:     entry.Data.HeaderImage,
		IsFree:          entry.Data.IsFree,
		ComingSoon:      entry.Data.ReleaseDate.ComingSoon,
		ReleaseDate:     entry.Data.ReleaseDate.Date,
		InitialPrice:    p.Initial,
		CurrentPrice:    p.Final,
		DiscountPercent: p.DiscountPercent,
		Currency:        p.Currency,
		HasPrice:        p.Currency != "" || p.Initial > 0 || p.Final > 0,
	}, nil
}

func applySteamHistory(game *steamGame, data []byte) {
	var payload struct {
		Status string `json:"status"`
		Result struct {
			LowestInfo struct {
				Price    any `json:"price"`
				Date     any `json:"date"`
				Discount int `json:"discount"`
			} `json:"lowest_info"`
			LowestInfoV2 struct {
				Currency string `json:"currency"`
			} `json:"lowest_info_v2"`
			Prices []struct {
				Price    any    `json:"price"`
				Date     any    `json:"date"`
				Discount int    `json:"discount"`
				Currency string `json:"currency"`
			} `json:"prices"`
		} `json:"result"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if decoder.Decode(&payload) != nil || (payload.Status != "" && payload.Status != "ok") {
		return
	}
	price, ok := dealFloat(payload.Result.LowestInfo.Price)
	date := dealDate(payload.Result.LowestInfo.Date)
	discount := payload.Result.LowestInfo.Discount
	currency := payload.Result.LowestInfoV2.Currency
	if !ok {
		for _, point := range payload.Result.Prices {
			value, pointOK := dealFloat(point.Price)
			if !pointOK || (ok && value >= price) {
				continue
			}
			price, ok = value, true
			date = dealDate(point.Date)
			discount = point.Discount
			currency = point.Currency
		}
	}
	if ok {
		game.LowestPrice = price
		game.LowestDiscount = discount
		game.LowestDate = date
		game.LowestCurrency = strings.ToUpper(currency)
		game.HasLowest = true
	}
}

func dealFloat(value any) (float64, bool) {
	switch value := value.(type) {
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	case float64:
		return value, true
	case string:
		parsed, err := strconv.ParseFloat(value, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func dealDate(value any) string {
	switch value := value.(type) {
	case json.Number:
		seconds, err := value.Int64()
		if err == nil {
			return time.Unix(seconds, 0).In(bot.Now().Location()).Format("2006-01-02")
		}
	case float64:
		return time.Unix(int64(value), 0).In(bot.Now().Location()).Format("2006-01-02")
	case string:
		if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
			return time.Unix(seconds, 0).In(bot.Now().Location()).Format("2006-01-02")
		}
		if len(value) >= 10 {
			return value[:10]
		}
	}
	return ""
}

func formatSteamGame(game steamGame) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "🎮 %s\n", game.Name)
	switch {
	case game.IsFree:
		sb.WriteString("当前：免费游玩\n")
	case game.ComingSoon:
		fmt.Fprintf(&sb, "状态：尚未发售（%s）\n", game.ReleaseDate)
	case game.HasPrice:
		fmt.Fprintf(&sb, "当前：%s", formatSteamCents(game.CurrentPrice, game.Currency))
		if game.DiscountPercent > 0 {
			fmt.Fprintf(&sb, "  -%d%%  原价 %s", game.DiscountPercent, formatSteamCents(game.InitialPrice, game.Currency))
		}
		sb.WriteByte('\n')
	default:
		sb.WriteString("当前：国区暂无价格\n")
	}
	if game.HasLowest {
		fmt.Fprintf(&sb, "参考史低：%s", formatDealMoney(game.LowestPrice, game.LowestCurrency))
		if game.LowestDiscount > 0 {
			fmt.Fprintf(&sb, "  -%d%%", game.LowestDiscount)
		}
		if game.LowestDate != "" {
			fmt.Fprintf(&sb, "（%s）", game.LowestDate)
		}
		sb.WriteByte('\n')
	} else {
		sb.WriteString("参考史低：暂未查到\n")
	}
	fmt.Fprintf(&sb, "Steam：https://store.steampowered.com/app/%d/\n", game.AppID)
	sb.WriteString("价格仅供参考，购买前请以商店结算页为准")
	return sb.String()
}

func formatSteamCents(cents int, currency string) string {
	return formatDealMoney(float64(cents)/100, currency)
}

func formatDealMoney(value float64, currency string) string {
	prefix := strings.ToUpper(currency) + " "
	if currency == "" || strings.EqualFold(currency, "CNY") || strings.EqualFold(currency, "RMB") || strings.EqualFold(currency, "CN") {
		prefix = "¥"
	}
	if value == float64(int64(value)) {
		return fmt.Sprintf("%s%.0f", prefix, value)
	}
	return fmt.Sprintf("%s%.2f", prefix, value)
}

func fetchDealCover(rawURL string) []byte {
	if rawURL == "" {
		return nil
	}
	data, err := httpclient.GetPublicBytesLimit(rawURL, 5<<20, "Referer", "https://store.steampowered.com/")
	if err != nil {
		return nil
	}
	return data
}

func dealGet(rawURL string, headers ...string) ([]byte, error) {
	return httpclient.GetPublicBytesLimit(rawURL, dealResponseLimit, headers...)
}

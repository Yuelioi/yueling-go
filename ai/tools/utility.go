package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Yuelioi/yueling-go/ai"
)

func init() {
	registerCalc()
	registerTimeNow()
	registerExchangeRate()
}

// ── Calculator ────────────────────────────────────────────────────────────────

func registerCalc() {
	ai.Register(ai.ToolMeta{
		Name:        "calc",
		Description: "计算数学表达式，支持加减乘除、括号、乘方",
		Tags:        []string{"工具", "计算"},
		Triggers:    []string{"计算", "算一下", "算算", "等于多少", "多少钱"},
		Slots:       []string{"计算", "等于", "+", "-", "*", "/", "×", "÷"},
		Params: []ai.Param{
			{Name: "expr", Type: "string", Description: "数学表达式，如 (3+5)*2 或 2^10", Required: true},
		},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			expr := ctx.String("expr")
			expr = strings.ReplaceAll(expr, "×", "*")
			expr = strings.ReplaceAll(expr, "÷", "/")
			expr = strings.ReplaceAll(expr, "，", "")
			expr = strings.ReplaceAll(expr, " ", "")

			result, err := evalExpr(expr)
			if err != nil {
				return "表达式有误：" + err.Error(), nil
			}
			if result == math.Trunc(result) {
				return fmt.Sprintf("%s = %d", expr, int64(result)), nil
			}
			return fmt.Sprintf("%s = %g", expr, result), nil
		},
	})
}

// evalExpr: recursive descent parser for +,-,*,/,^,(  )
func evalExpr(s string) (float64, error) {
	p := &parser{src: s, pos: 0}
	v, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos != len(p.src) {
		return 0, fmt.Errorf("unexpected character at pos %d", p.pos)
	}
	return v, nil
}

type parser struct {
	src string
	pos int
}

func (p *parser) peek() byte {
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *parser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for p.peek() == '+' || p.peek() == '-' {
		op := p.src[p.pos]
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *parser) parseTerm() (float64, error) {
	left, err := p.parsePower()
	if err != nil {
		return 0, err
	}
	for p.peek() == '*' || p.peek() == '/' {
		op := p.src[p.pos]
		p.pos++
		right, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("除以零")
			}
			left /= right
		}
	}
	return left, nil
}

func (p *parser) parsePower() (float64, error) {
	base, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	if p.peek() == '^' {
		p.pos++
		exp, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

func (p *parser) parseFactor() (float64, error) {
	if p.peek() == '-' {
		p.pos++
		v, err := p.parseFactor()
		return -v, err
	}
	if p.peek() == '(' {
		p.pos++
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("缺少右括号")
		}
		p.pos++
		return v, nil
	}
	return p.parseNumber()
}

func (p *parser) parseNumber() (float64, error) {
	start := p.pos
	for p.pos < len(p.src) && (unicode.IsDigit(rune(p.src[p.pos])) || p.src[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("expected number at pos %d", p.pos)
	}
	return strconv.ParseFloat(p.src[start:p.pos], 64)
}

// ── Time ──────────────────────────────────────────────────────────────────────

func registerTimeNow() {
	ai.Register(ai.ToolMeta{
		Name:        "time_now",
		Description: "获取当前日期和时间",
		Tags:        []string{"工具", "时间"},
		Triggers:    []string{"现在几点", "今天几号", "今天星期", "现在时间", "日期"},
		Slots:       []string{"时间", "日期", "几点", "今天", "星期"},
		Params:      []ai.Param{},
		Handler: func(ctx *ai.ToolContext) (string, error) {
			loc, _ := time.LoadLocation("Asia/Shanghai")
			now := time.Now().In(loc)
			weekdays := []string{"日", "一", "二", "三", "四", "五", "六"}
			return fmt.Sprintf("现在是北京时间 %s，星期%s",
				now.Format("2006年01月02日 15:04:05"),
				weekdays[now.Weekday()],
			), nil
		},
	})
}

// ── Exchange Rate ─────────────────────────────────────────────────────────────

var erClient = &http.Client{Timeout: 8 * time.Second}

func registerExchangeRate() {
	ai.Register(ai.ToolMeta{
		Name:        "exchange_rate",
		Description: "查询两种货币之间的实时汇率，如 USD→CNY",
		Tags:        []string{"工具", "汇率", "货币"},
		Triggers:    []string{"汇率", "换算", "兑换", "美元", "人民币", "日元", "欧元"},
		Slots:       []string{"汇率", "兑", "换", "多少钱"},
		Params: []ai.Param{
			{Name: "from", Type: "string", Description: "源货币代码，如 USD、CNY、JPY", Required: true},
			{Name: "to", Type: "string", Description: "目标货币代码，如 CNY、USD、EUR", Required: true},
			{Name: "amount", Type: "number", Description: "要换算的金额，默认1", Required: false},
		},
		Handler: exchangeRateHandler,
	})
}

func exchangeRateHandler(ctx *ai.ToolContext) (string, error) {
	from := strings.ToUpper(strings.TrimSpace(ctx.String("from")))
	to := strings.ToUpper(strings.TrimSpace(ctx.String("to")))
	amount := ctx.Float("amount")
	if amount == 0 {
		amount = 1
	}
	if from == "" || to == "" {
		return "请提供源货币和目标货币代码", nil
	}

	url := fmt.Sprintf("https://open.er-api.com/v6/latest/%s", from)
	resp, err := erClient.Get(url)
	if err != nil {
		return "汇率查询失败：网络错误", nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		Result string             `json:"result"`
		Rates  map[string]float64 `json:"rates"`
	}
	if err := json.Unmarshal(body, &data); err != nil || data.Result != "success" {
		return "汇率查询失败：数据解析错误", nil
	}

	rate, ok := data.Rates[to]
	if !ok {
		return fmt.Sprintf("不支持的货币代码：%s", to), nil
	}

	converted := amount * rate
	if amount == 1 {
		return fmt.Sprintf("1 %s = %.4f %s", from, rate, to), nil
	}
	return fmt.Sprintf("%.2f %s = %.2f %s（汇率 %.4f）", amount, from, converted, to, rate), nil
}

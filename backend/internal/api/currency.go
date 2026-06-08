package api

import "math"

// CurrencyInfo describes a supported currency (fiat or crypto).
type CurrencyInfo struct {
	Code     string `json:"code"`
	Type     string `json:"type"` // fiat | crypto
	Decimals int    `json:"decimals"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	// CoinGeckoID is set for crypto so we can look up its USD price.
	CoinGeckoID string `json:"-"`
}

// currencyList is the ordered catalog. Order drives account creation and UI.
var currencyList = []CurrencyInfo{
	{Code: "CRC", Type: "fiat", Decimals: 2, Symbol: "₡", Name: "Colón"},
	{Code: "USD", Type: "fiat", Decimals: 2, Symbol: "$", Name: "Dólar"},
	{Code: "BTC", Type: "crypto", Decimals: 8, Symbol: "₿", Name: "Bitcoin", CoinGeckoID: "bitcoin"},
	{Code: "ETH", Type: "crypto", Decimals: 8, Symbol: "Ξ", Name: "Ethereum", CoinGeckoID: "ethereum"},
	{Code: "USDT", Type: "crypto", Decimals: 2, Symbol: "₮", Name: "Tether USD", CoinGeckoID: "tether"},
}

var currencyByCode = func() map[string]CurrencyInfo {
	m := make(map[string]CurrencyInfo, len(currencyList))
	for _, c := range currencyList {
		m[c.Code] = c
	}
	return m
}()

func validCurrency(code string) bool {
	_, ok := currencyByCode[code]
	return ok
}

func decimalsFor(code string) int {
	if c, ok := currencyByCode[code]; ok {
		return c.Decimals
	}
	return 2
}

// toMinor converts a major-unit amount (e.g. 1.5 BTC) into integer minor units
// (e.g. 150000000 satoshis) for the given currency.
func toMinor(amount float64, code string) int64 {
	return int64(math.Round(amount * math.Pow(10, float64(decimalsFor(code)))))
}

// majorOf converts integer minor units back to a major-unit float.
func majorOf(minor int64, code string) float64 {
	return float64(minor) / math.Pow(10, float64(decimalsFor(code)))
}

// allCurrencyCodes returns every supported currency code in catalog order.
func allCurrencyCodes() []string {
	codes := make([]string, len(currencyList))
	for i, c := range currencyList {
		codes[i] = c.Code
	}
	return codes
}

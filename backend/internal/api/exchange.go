package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var errInvalidRate = errors.New("invalid exchange rate from source")

// ExchangeRate is the USD/CRC reference rate (colones per dollar) from BCCR.
type ExchangeRate struct {
	Buy    float64 `json:"buy"`  // compra
	Sell   float64 `json:"sell"` // venta
	Date   string  `json:"date"`
	Source string  `json:"source"`
}

// Rates bundles the fiat reference rate, crypto USD prices, and a unified
// USD-per-unit table used for conversions between any two currencies.
type Rates struct {
	Crc        ExchangeRate       `json:"crc"`
	Crypto     map[string]float64 `json:"crypto"`     // USD price per coin (by code)
	UsdPerUnit map[string]float64 `json:"usdPerUnit"` // USD value of 1 unit of each currency
	Currencies []CurrencyInfo     `json:"currencies"`
	UpdatedAt  string             `json:"updatedAt"`
}

var (
	fallbackRate = ExchangeRate{Buy: 505, Sell: 515, Source: "fallback"}
	// Approximate USD prices used only when CoinGecko is unreachable and we
	// have no previously-cached value.
	cryptoFallback = map[string]float64{
		"BTC": 63000, "ETH": 1700, "USDT": 1, "USDC": 1, "BNB": 600,
		"SOL": 150, "XRP": 0.5, "ADA": 0.4, "DOGE": 0.1, "TRX": 0.12,
		"DOT": 4, "LTC": 80, "LINK": 12, "AVAX": 25, "MATIC": 0.5,
	}
	// Approximate USD value of 1 unit of each non-USD/CRC fiat, used only when
	// frankfurter.app is unreachable and we have no cached value.
	fiatFallback = map[string]float64{
		"EUR": 1.08, "MXN": 0.055,
	}
)

type ratesCache struct {
	mu        sync.Mutex
	rates     Rates
	fetchedAt time.Time
}

var rateStore = &ratesCache{}

const rateTTL = 5 * time.Minute

func (a *App) getRates(ctx context.Context) Rates {
	rateStore.mu.Lock()
	defer rateStore.mu.Unlock()

	if !rateStore.fetchedAt.IsZero() && time.Since(rateStore.fetchedAt) < rateTTL {
		return rateStore.rates
	}

	prev := rateStore.rates // last good values (may be zero on first run)

	crc, err := fetchHaciendaRate(ctx)
	if err != nil {
		if prev.Crc.Sell > 0 {
			crc = prev.Crc // serve stale rather than fallback
		} else {
			crc = fallbackRate
		}
	}
	crypto, err := fetchCryptoPrices(ctx)
	if err != nil {
		if len(prev.Crypto) > 0 {
			crypto = prev.Crypto // keep last good prices
		} else {
			crypto = cloneFloatMap(cryptoFallback)
		}
	}
	// USD-per-unit for non-USD/CRC fiat (EUR, MXN…) via frankfurter.app.
	fiat, err := fetchFiatRates(ctx)
	if err != nil {
		fiat = map[string]float64{}
		for code, fb := range fiatFallback {
			if prev.UsdPerUnit[code] > 0 {
				fiat[code] = prev.UsdPerUnit[code] // serve last good rate
			} else {
				fiat[code] = fb
			}
		}
	}

	usdPerUnit := map[string]float64{}
	for _, c := range currencyList {
		switch {
		case c.Code == "USD":
			usdPerUnit[c.Code] = 1
		case c.Code == "CRC":
			if crc.Sell > 0 {
				usdPerUnit[c.Code] = 1 / crc.Sell
			}
		case c.Type == "fiat":
			if v, ok := fiat[c.Code]; ok && v > 0 {
				usdPerUnit[c.Code] = v
			} else if fb, ok := fiatFallback[c.Code]; ok {
				usdPerUnit[c.Code] = fb
			}
		case c.Type == "crypto":
			if p, ok := crypto[c.Code]; ok && p > 0 {
				usdPerUnit[c.Code] = p
			} else if p, ok := cryptoFallback[c.Code]; ok {
				usdPerUnit[c.Code] = p
			}
		}
	}

	rateStore.rates = Rates{
		Crc:        crc,
		Crypto:     crypto,
		UsdPerUnit: usdPerUnit,
		Currencies: currencyList,
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	rateStore.fetchedAt = time.Now()
	return rateStore.rates
}

func fetchHaciendaRate(ctx context.Context) (ExchangeRate, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet,
		"https://api.hacienda.go.cr/indicadores/tc/dolar", nil)
	if err != nil {
		return ExchangeRate{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ExchangeRate{}, err
	}
	defer resp.Body.Close()

	var body struct {
		Compra struct {
			Fecha string  `json:"fecha"`
			Valor float64 `json:"valor"`
		} `json:"compra"`
		Venta struct {
			Fecha string  `json:"fecha"`
			Valor float64 `json:"valor"`
		} `json:"venta"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ExchangeRate{}, err
	}
	if body.Compra.Valor <= 0 || body.Venta.Valor <= 0 {
		return ExchangeRate{}, errInvalidRate
	}
	return ExchangeRate{
		Buy:    body.Compra.Valor,
		Sell:   body.Venta.Valor,
		Date:   body.Venta.Fecha,
		Source: "Ministerio de Hacienda CR",
	}, nil
}

func fetchCryptoPrices(ctx context.Context) (map[string]float64, error) {
	ids := []string{}
	idToCode := map[string]string{}
	for _, c := range currencyList {
		if c.Type == "crypto" && c.CoinGeckoID != "" {
			ids = append(ids, c.CoinGeckoID)
			idToCode[c.CoinGeckoID] = c.Code
		}
	}
	if len(ids) == 0 {
		return map[string]float64{}, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	endpoint := "https://api.coingecko.com/api/v3/simple/price?ids=" +
		url.QueryEscape(strings.Join(ids, ",")) + "&vs_currencies=usd"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body map[string]struct {
		USD float64 `json:"usd"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	prices := map[string]float64{}
	for id, v := range body {
		if code, ok := idToCode[id]; ok && v.USD > 0 {
			prices[code] = v.USD
		}
	}
	if len(prices) == 0 {
		return nil, errInvalidRate
	}
	return prices, nil
}

// fetchFiatRates returns the USD value of 1 unit of each non-USD/CRC fiat
// currency (e.g. EUR, MXN), sourced from the free frankfurter.app feed.
func fetchFiatRates(ctx context.Context) (map[string]float64, error) {
	codes := extraFiatCodes()
	if len(codes) == 0 {
		return map[string]float64{}, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	endpoint := "https://api.frankfurter.app/latest?base=USD&symbols=" +
		url.QueryEscape(strings.Join(codes, ","))
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body struct {
		Rates map[string]float64 `json:"rates"` // units of each currency per 1 USD
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for code, perUSD := range body.Rates {
		if perUSD > 0 {
			out[code] = 1 / perUSD // invert to USD value of 1 unit
		}
	}
	if len(out) == 0 {
		return nil, errInvalidRate
	}
	return out, nil
}

func cloneFloatMap(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (a *App) handleExchangeRate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.getRates(r.Context()).Crc)
}

func (a *App) handleRates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.getRates(r.Context()))
}

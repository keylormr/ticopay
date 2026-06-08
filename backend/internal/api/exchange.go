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
	fallbackRate   = ExchangeRate{Buy: 505, Sell: 515, Source: "fallback"}
	cryptoFallback = map[string]float64{"BTC": 60000, "ETH": 3000, "USDT": 1}
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

	crc, err := fetchHaciendaRate(ctx)
	if err != nil {
		crc = fallbackRate
	}
	crypto, err := fetchCryptoPrices(ctx)
	if err != nil {
		crypto = cloneFloatMap(cryptoFallback)
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

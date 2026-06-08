package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// ExchangeRate is the USD/CRC reference rate (colones per dollar).
type ExchangeRate struct {
	Buy    float64 `json:"buy"`  // compra: colones the system pays per USD sold
	Sell   float64 `json:"sell"` // venta:  colones charged per USD bought
	Date   string  `json:"date"`
	Source string  `json:"source"`
}

// Fallback used only if Hacienda's API is unreachable on a cold start.
var fallbackRate = ExchangeRate{Buy: 505, Sell: 515, Date: "", Source: "fallback"}

type rateCache struct {
	mu        sync.Mutex
	rate      ExchangeRate
	fetchedAt time.Time
}

var rates = &rateCache{rate: fallbackRate}

const rateTTL = time.Hour

// getExchangeRate returns a cached rate, refreshing from Hacienda when stale.
func (a *App) getExchangeRate(ctx context.Context) ExchangeRate {
	rates.mu.Lock()
	defer rates.mu.Unlock()

	if !rates.fetchedAt.IsZero() && time.Since(rates.fetchedAt) < rateTTL {
		return rates.rate
	}

	r, err := fetchHaciendaRate(ctx)
	if err != nil {
		if rates.fetchedAt.IsZero() {
			return rates.rate // still the fallback
		}
		return rates.rate // serve stale rather than fail
	}
	rates.rate = r
	rates.fetchedAt = time.Now()
	return r
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
		Dolar struct {
			Compra struct {
				Fecha string  `json:"fecha"`
				Valor float64 `json:"valor"`
			} `json:"compra"`
			Venta struct {
				Fecha string  `json:"fecha"`
				Valor float64 `json:"valor"`
			} `json:"venta"`
		} `json:"dolar"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ExchangeRate{}, err
	}

	return ExchangeRate{
		Buy:    body.Dolar.Compra.Valor,
		Sell:   body.Dolar.Venta.Valor,
		Date:   body.Dolar.Venta.Fecha,
		Source: "Ministerio de Hacienda CR",
	}, nil
}

func (a *App) handleExchangeRate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.getExchangeRate(r.Context()))
}

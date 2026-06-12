package api

import "testing"

func TestToMinorMajorRoundTrip(t *testing.T) {
	cases := []struct {
		code   string
		amount float64
		minor  int64
	}{
		{"CRC", 5000, 500000},
		{"USD", 1.50, 150},
		{"EUR", 0.01, 1},
		{"MXN", 99.99, 9999},
		{"BTC", 1, 100000000},      // 8 decimals
		{"BTC", 0.00000001, 1},     // 1 satoshi
		{"XRP", 2.5, 2500000},      // 6 decimals
		{"USDT", 10, 1000},         // stablecoin keeps 2 decimals
	}
	for _, c := range cases {
		if got := toMinor(c.amount, c.code); got != c.minor {
			t.Errorf("toMinor(%v, %s) = %d, want %d", c.amount, c.code, got, c.minor)
		}
		if got := majorOf(c.minor, c.code); got != c.amount {
			t.Errorf("majorOf(%d, %s) = %v, want %v", c.minor, c.code, got, c.amount)
		}
	}
}

func TestToMinorRounds(t *testing.T) {
	// Float artifacts must round, not truncate: 0.1+0.2 → 30 cents.
	if got := toMinor(0.1+0.2, "USD"); got != 30 {
		t.Errorf("toMinor(0.1+0.2, USD) = %d, want 30", got)
	}
}

func TestCurrencyCatalog(t *testing.T) {
	for _, code := range []string{"CRC", "USD", "EUR", "MXN", "BTC", "ETH"} {
		if !validCurrency(code) {
			t.Errorf("validCurrency(%s) = false, want true", code)
		}
	}
	for _, code := range []string{"GBP", "", "btc", "XXX"} {
		if validCurrency(code) {
			t.Errorf("validCurrency(%q) = true, want false", code)
		}
	}

	codes := allCurrencyCodes()
	if len(codes) != len(currencyList) {
		t.Fatalf("allCurrencyCodes returned %d codes, want %d", len(codes), len(currencyList))
	}
	// Catalog order drives account creation and UI: CRC must come first.
	if codes[0] != "CRC" {
		t.Errorf("first currency = %s, want CRC", codes[0])
	}

	// Unknown currencies default to 2 decimals (defensive).
	if d := decimalsFor("XXX"); d != 2 {
		t.Errorf("decimalsFor(XXX) = %d, want 2", d)
	}
}

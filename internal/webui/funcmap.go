package webui

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"
)

// TemplateFuncs is the FuncMap registered on every template.
var TemplateFuncs = template.FuncMap{
	"money":      money,
	"rp":         rp,
	"meter":      meter,
	"qtyText":    qtyText,
	"dt":         dt,
	"dd":         dd,
	"dateInput":  dateInput,
	"dateString": dateString,
	"add":        func(a, b float64) float64 { return a + b },
	"sub":        func(a, b float64) float64 { return a - b },
	"mul":        func(a, b float64) float64 { return a * b },
	"div": func(a, b float64) float64 {
		if b == 0 {
			return 0
		}
		return a / b
	},
	"slice":     sliceStr,
	"dict":      dict,
	"json":      jsonStr,
	"upper":     strings.ToUpper,
	"hasPrefix": strings.HasPrefix,
}

// money formats as "Rp 1.234" with Indonesian thousands dot.
func money(v float64) string {
	return "Rp " + rp(v)
}

// rp formats as "1.234" (thousands separator with dots, no prefix).
func rp(v float64) string {
	abs := math.Round(math.Abs(v))
	s := fmt.Sprintf("%.0f", abs)
	// Insert dots every 3 digits from the right.
	n := len(s)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteRune('.')
		}
		b.WriteRune(c)
	}
	if v < 0 {
		return "-" + b.String()
	}
	return b.String()
}

// meter formats as "1,5" (comma decimal, trims trailing zeros).
func meter(v float64) string {
	// Round to 2 decimal places.
	rounded := math.Round(v*100) / 100
	s := fmt.Sprintf("%.2f", rounded)
	// Replace dot with comma, then trim trailing zeros.
	s = strings.Replace(s, ".", ",", 1)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ",")
	return s
}

// qtyText formats as "1,5 m" or "3 pcs".
func qtyText(value float64, unit string) string {
	if strings.ToLower(unit) == "meter" {
		return meter(value) + " m"
	}
	// For pcs: whole number if possible.
	if value == math.Trunc(value) {
		return fmt.Sprintf("%.0f %s", value, unit)
	}
	return meter(value) + " " + unit
}

// dt formats as "02/01/2006 15:04:05".
func dt(t time.Time) string {
	return t.Format("02/01/2006 15:04:05")
}

// dd formats as "02/01/2006".
func dd(t time.Time) string {
	return t.Format("02/01/2006")
}

// dateInput parses "02 Jan 2006" → "2006-01-02" for HTML date input.
func dateInput(s string) string {
	t, err := time.Parse("02 Jan 2006", s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02")
}

// dateString parses "2006-01-02" → "02/01/2006".
func dateString(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("02/01/2006")
}

// sliceStr returns s[:n] safely.
func sliceStr(s string, n int) string {
	runes := []rune(s)
	if n > len(runes) {
		n = len(runes)
	}
	if n < 0 {
		n = 0
	}
	return string(runes[:n])
}

// dict builds a map from key-value pairs (for template use).
func dict(pairs ...any) map[string]any {
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, _ := pairs[i].(string)
		m[key] = pairs[i+1]
	}
	return m
}

// jsonStr marshals v to a JSON string (safe for template use).
func jsonStr(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

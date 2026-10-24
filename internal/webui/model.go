package webui

import (
	"fmt"
	"strings"
	"time"
)

// UIProduct maps to the template's product shape.
type UIProduct struct {
	ID        uint
	SKU       string // = product.Code
	Name      string
	Category  string  // = product.CategoryName or ""
	StoreID   uint
	StoreName string
	Unit      string
	Price     float64 // = SellPrice.InexactFloat64()
	Cost      float64 // = CostPrice (admin only; 0 for cashier)
	Stock     float64 // = primary store qty
	MinStock  float64
	Color     string
	Material  string
}

func (p UIProduct) Low() bool { return p.MinStock > 0 && p.Stock <= p.MinStock }

// UISaleItem is one line of a sale.
type UISaleItem struct {
	SKU   string
	Name  string
	Qty   float64
	Unit  string
	Price float64
}

func (i UISaleItem) Sub() float64 { return i.Qty * i.Price }

// UIPayment is one payment record on a sale.
type UIPayment struct {
	Date   time.Time
	Amount float64
	Method string
}

// UISale is a single order in UI form.
type UISale struct {
	No              string
	Date            time.Time
	Dept            string
	Customer        string
	Sales           string
	Tax             string
	Subtotal        float64
	DiscountPercent float64
	DiscountAmount  float64
	Total           float64
	Paid            float64
	Change          float64
	Payments        []UIPayment
	Cashier         string
	Computer        string
	Note            string
	Items           []UISaleItem
}

func (s UISale) Pending() bool { return s.Paid+1e-9 < s.Total }
func (s UISale) Balance() float64 {
	if s.Total <= s.Paid {
		return 0
	}
	return s.Total - s.Paid
}
func (s UISale) Count() int { return len(s.Items) }
func (s UISale) Hay() string {
	return strings.ToLower(strings.Join([]string{s.No, s.Customer, s.Sales, s.Cashier, s.Note}, " "))
}
func (s UISale) Meters() float64 {
	var m float64
	for _, i := range s.Items {
		if strings.ToLower(i.Unit) == "meter" {
			m += i.Qty
		}
	}
	return m
}
func (s UISale) QuantitySummary() string {
	totals := map[string]float64{}
	for _, i := range s.Items {
		totals[strings.ToLower(i.Unit)] += i.Qty
	}
	return buildQtySummary(totals)
}

// UITransferItem is one line of a stock transfer.
type UITransferItem struct {
	SKU  string
	Name string
	Qty  float64
	Unit string
}

// UITransfer is a stock transfer in UI form.
type UITransfer struct {
	No      string
	Date    time.Time
	Type    string // "in" or "out"
	From    string
	To      string
	Note    string
	Cashier string
	Items   []UITransferItem
}

func (t UITransfer) Count() int { return len(t.Items) }
func (t UITransfer) Meters() float64 {
	var m float64
	for _, i := range t.Items {
		m += i.Qty
	}
	return m
}
func (t UITransfer) QuantitySummary() string {
	totals := map[string]float64{}
	for _, i := range t.Items {
		totals[strings.ToLower(i.Unit)] += i.Qty
	}
	return buildQtySummary(totals)
}

// UISupplierBill is a supplier invoice in UI form.
type UISupplierBill struct {
	ID           string
	Supplier     string
	InvoiceNo    string
	InvoiceDate  string
	ReceivedDate string
	Amount       float64
	DueDate      string
	Status       string
}

// DayStat holds one day's revenue summary.
type DayStat struct {
	Key     string
	Label   string
	Total   float64
	Count   int
	Pct     float64
	PctText string
	Text    string
}

// Agg holds one aggregated product row (top items, etc.).
type Agg struct {
	Name    string
	SKU     string
	Unit    string
	Meter   float64
	Total   float64
	Pct     float64
	PctText string
}

// CommissionRow is one salesperson commission summary row.
type CommissionRow struct {
	Date       time.Time
	SaleNo     string
	Sales      string
	Customer   string
	SKU        string
	Item       string
	Qty        float64
	Unit       string
	Rate       float64
	Commission float64
}

// PageData is the full data bag passed to every template.
type PageData struct {
	Page        string
	Title       string
	Company     string
	DeptLabel   string
	Cashier     string
	CashierRole string
	Build       string
	Now         string

	Products    []UIProduct
	LowStock    []UIProduct
	Categories  []string
	SalesPeople []string

	Sales       []UISale
	RecentSales []UISale

	Transfers    []UITransfer
	TransferType string

	SupplierBills                                                                    []UISupplierBill
	SupplierName                                                                     string
	SupplierBillCount, SupplierPaidCount, SupplierUnpaidCount, SupplierOverdueCount int
	SupplierPaidTotal, SupplierUnpaidTotal, SupplierOverdueTotal                    float64

	CommissionRows []CommissionRow

	Today     DayStat
	Yesterday DayStat
	Days      []DayStat

	Revenue       float64
	TrxCount      int
	MeterSold     float64
	Avg           float64
	AvgDaily      float64
	StockValue    float64
	ChangeClass   string
	ChangeText    string
	ChangePercent float64

	TopItems []Agg
	TopMax   float64
	TopEmpty bool
}

// buildQtySummary formats meter + pcs totals as Indonesian text, e.g. "1,5 m + 3 pcs".
func buildQtySummary(totals map[string]float64) string {
	var parts []string
	if v, ok := totals["meter"]; ok && v > 0 {
		parts = append(parts, meter(v)+" m")
	}
	// All non-meter units.
	for unit, v := range totals {
		if unit == "meter" || v <= 0 {
			continue
		}
		if v == float64(int(v)) {
			parts = append(parts, fmt.Sprintf("%.0f %s", v, unit))
		} else {
			parts = append(parts, meter(v)+" "+unit)
		}
	}
	return strings.Join(parts, " + ")
}

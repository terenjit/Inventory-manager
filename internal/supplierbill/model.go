package supplierbill

import "time"

// Bill is the DB row. Date fields use time.Time so parseTime=true works correctly.
type Bill struct {
	ID           uint       `db:"id"            json:"id"`
	Supplier     string     `db:"supplier_name" json:"supplier"`
	InvoiceNo    string     `db:"invoice_no"    json:"invoiceNo"`
	InvoiceDate  time.Time  `db:"invoice_date"  json:"-"`
	ReceivedDate *time.Time `db:"received_date" json:"-"`
	DueDate      *time.Time `db:"due_date"      json:"-"`
	Amount       float64    `db:"amount"        json:"amount"`
	Status       string     `db:"status"        json:"-"`
	CreatedAt    time.Time  `db:"created_at"    json:"-"`
}

func (b Bill) StatusLabel() string {
	if b.Status == "paid" {
		return "Lunas"
	}
	return "Belum Lunas"
}

// Response is the JSON shape sent to the front-end.
// Dates are formatted as "YYYY-MM-DD" so <input type="date"> works correctly.
type Response struct {
	ID           uint    `json:"id"`
	Supplier     string  `json:"supplier"`
	InvoiceNo    string  `json:"invoiceNo"`
	InvoiceDate  string  `json:"invoiceDate"`
	ReceivedDate string  `json:"receivedDate"`
	Amount       float64 `json:"amount"`
	DueDate      string  `json:"dueDate"`
	Status       string  `json:"status"` // "Lunas" or "Belum Lunas"
}

func toResponse(b *Bill) Response {
	resp := Response{
		ID:          b.ID,
		Supplier:    b.Supplier,
		InvoiceNo:   b.InvoiceNo,
		InvoiceDate: b.InvoiceDate.Format("2006-01-02"),
		Amount:      b.Amount,
		Status:      b.StatusLabel(),
	}
	if b.ReceivedDate != nil {
		resp.ReceivedDate = b.ReceivedDate.Format("2006-01-02")
	}
	if b.DueDate != nil {
		resp.DueDate = b.DueDate.Format("2006-01-02")
	}
	return resp
}

type CreateInput struct {
	Supplier     string  `json:"supplier"     binding:"required"`
	InvoiceNo    string  `json:"invoiceNo"    binding:"required"`
	InvoiceDate  string  `json:"invoiceDate"  binding:"required"`
	ReceivedDate string  `json:"receivedDate"`
	Amount       float64 `json:"amount"`
	DueDate      string  `json:"dueDate"`
	Status       string  `json:"status"` // "Lunas" or "Belum Lunas"
}

type UpdateInput = CreateInput

// dbStatus converts "Lunas"/"Belum Lunas" → "paid"/"unpaid".
func dbStatus(label string) string {
	if label == "Lunas" {
		return "paid"
	}
	return "unpaid"
}

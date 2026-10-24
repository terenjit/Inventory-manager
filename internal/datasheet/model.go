package datasheet

// RowError describes a validation error on a specific row and column.
type RowError struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

// ImportResult is returned after a successful import.
type ImportResult struct {
	Upserted int `json:"upserted"` // rows inserted or updated
	Skipped  int `json:"skipped"`  // rows with no changes
}

// Excel column positions (1-based).
const (
	colCode      = 1 // A: KodeItem
	colName      = 2 // B: NamaItem
	colCategory  = 3 // C: Jenis
	colUnit      = 4 // D: Satuan
	colCostPrice = 5 // E: HargaPokok
	colSellPrice = 6 // F: HargaJual
	colStock     = 7 // G: Stok (optional)
)

// Headers for the exported file.
var headers = []string{
	"KodeItem", "NamaItem", "Jenis", "Satuan", "HargaPokok", "HargaJual", "Stok",
}

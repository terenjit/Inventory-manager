package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"inventory-manager/internal/config"
	"inventory-manager/internal/db"
)

type discrepancy struct {
	StoreID     uint            `db:"store_id"`
	ProductID   uint            `db:"product_id"`
	StoreName   string          `db:"store_name"`
	ProductCode string          `db:"product_code"`
	CurrentQty  decimal.Decimal `db:"current_qty"`
	SumDelta    decimal.Decimal `db:"sum_delta"`
	Diff        decimal.Decimal `db:"diff"`
}

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	database, err := db.New(cfg.DBDSN)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer database.Close()

	ctx := context.Background()

	var discrepancies []discrepancy
	err = database.SelectContext(ctx, &discrepancies,
		`SELECT i.store_id, i.product_id,
		        st.name AS store_name, p.code AS product_code,
		        i.qty AS current_qty,
		        COALESCE(SUM(sm.delta), 0) AS sum_delta,
		        i.qty - COALESCE(SUM(sm.delta), 0) AS diff
		   FROM inventory i
		   JOIN stores st ON st.id = i.store_id
		   JOIN products p ON p.id = i.product_id
		   LEFT JOIN stock_movements sm
		          ON sm.store_id = i.store_id AND sm.product_id = i.product_id
		  GROUP BY i.store_id, i.product_id
		 HAVING ABS(diff) > 0.001
		  ORDER BY i.store_id, i.product_id`,
	)
	if err != nil {
		log.Fatalf("query: %v", err)
	}

	if len(discrepancies) == 0 {
		fmt.Println("✓ Ledger invariant OK: SUM(delta) == current_qty for all rows")
		os.Exit(0)
	}

	fmt.Printf("✗ Ledger discrepancies found (%d):\n\n", len(discrepancies))
	fmt.Printf("%-5s %-10s %-20s %-15s %-15s %-10s\n",
		"Store", "Code", "StoreName", "current_qty", "sum_delta", "diff")
	fmt.Println(string(make([]byte, 80)))

	for _, d := range discrepancies {
		fmt.Printf("%-5d %-10s %-20s %-15s %-15s %-10s\n",
			d.StoreID, d.ProductCode, d.StoreName,
			d.CurrentQty, d.SumDelta, d.Diff,
		)
	}
	os.Exit(1)
}

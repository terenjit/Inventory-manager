# Textile POS

Inventory and point-of-sale web app for a two-store textile (fabric) shop in Indonesia.
Fabric is sold by the meter. The app will be sold to and run by a small business owner,
so correctness of stock and money matters more than feature count.

## Stack

- Backend: Go 1.22+, Gin, MySQL 8.0.16+ (InnoDB, utf8mb4)
- DB access: sqlc (MySQL engine) for queries; explicit `database/sql` transactions for multi-step flows
- Migrations: golang-migrate, files in `migrations/`
- Decimals: `github.com/shopspring/decimal`. Never use float64 for meters or money.
- Frontend: existing UI in `web/`. It currently keeps data in localStorage and will be wired to the API.
- Local dev: docker compose (MySQL) and `make` targets

## Commands

<!-- Fill in during Phase 1 and keep updated -->
- `make up`: start MySQL
- `make migrate-up` / `make migrate-down`
- `make run`: start API
- `make test`: unit + integration tests (needs `make up`)
- `make lint`

## Domain

- Roles: `admin` and `cashier`. A cashier belongs to exactly one store (`users.store_id`).
  Admin has no store and must pass `store_id` explicitly where a store is needed.
- Two stores today; design for N.
- Products are one shared catalog. Stock is per store in `inventory`, in meters.
- UI text is Indonesian. Code, identifiers, comments, and commit messages are English.
- "Store" means a physical shop. Do not name the persistence layer "store"; use `repo` / `db`.

| UI term | Meaning | Tables |
|---|---|---|
| Kasir / Transaksi Baru | checkout | orders, order_items, payments |
| Barang Keluar | send stock to another store | stock_transfers (status `sent`) |
| Barang Masuk | receive a pending transfer | stock_transfers (status `received`) |
| Daftar Supplier / Nota | supplier purchase, adds stock | supplier_invoices, supplier_invoice_items |
| Stock Opname | physical stock count | stock_opnames, stock_opname_lines |
| Daftar Sales | salespeople picked on an order | salespeople |
| Harga Jual / Harga Modal | sell / cost price per meter | products.sell_price / cost_price |
| Dibayar / Sisa | paid / remaining | orders.paid_amount / total - paid_amount |

## Invariants (never break these)

1. Only `internal/stock` writes to `inventory`. Every change to `inventory.qty_m` happens in the
   same transaction as exactly one `stock_movements` row with the matching `delta_m` and `qty_after_m`.
2. For every (store, product): `SUM(stock_movements.delta_m) = inventory.qty_m`.
3. Stock never goes negative. Lock rows with `SELECT ... FOR UPDATE ORDER BY product_id`, check,
   then `UPDATE ... WHERE qty_m >= ?` and require `RowsAffected() == 1`.
4. A cashier's store comes from the authenticated session, never from the request body or query.
   Cashiers may READ any store's stock but WRITE only to their own store.
5. Cashiers never receive `cost_price`, margin, or supplier invoice data. Enforce this in response
   DTOs on the server, not only in the UI.
6. Nothing financial is hard-deleted. Products: `is_active = false`. Orders: `status = 'void'` with a
   reason, stock restored via a `sale_void` movement, admin only.
7. Money is DECIMAL(15,2) Rupiah; line totals are rounded to whole Rupiah. `paid_amount <= total`.
   Payment methods: QRIS, TRANSFER, CASH. Cash above the amount due is change and is not stored as paid.
8. Quantities are DECIMAL(12,2) meters, `> 0`, and a multiple of 0.1.
9. Store timestamps in UTC. Derive business dates (reports, daily document numbers) in WIB (fixed UTC+7).
10. Product codes are stored uppercase and matched case-insensitively.

## Conventions

- Layout: `cmd/server`, `internal/<domain>` (e.g. `auth`, `product`, `stock`, `order`, `transfer`,
  `purchase`, `opname`, `report`), `internal/httpapi` for Echo handlers and routing, `internal/db` for sqlc output.
- Handlers are thin. Business logic lives in services. Service methods take `(ctx, actor, input)`,
  where `actor` comes from the session.
- Typed domain errors, mapped to HTTP in one place: 400 validation, 401, 403, 404, 409 conflict
  (including insufficient stock).
- Every multi-step write is one transaction. Retry once on MySQL deadlock (error 1213).
- JSON is snake_case. Decimals are sent and received as strings (`"12.50"`) to avoid JS float errors.
- Pagination: `?page=&limit=` (max 100), response includes `total`.
- Tests touching stock or money are integration tests against real MySQL, not mocks. Any flow that
  decrements stock gets a concurrency test (parallel requests must never oversell).
- Reference implementation for checkout: `docs/reference/order_service.go`.

## How to work in this repo

- Read `docs/PLAN.md`. Work only on the phase named in the prompt.
- For changes touching more than one or two files, propose a plan and wait for approval before writing code.
- Never edit an applied migration; add a new numbered migration instead.
- Say why before adding any new dependency.
- Run `make test` (and `make lint`) before reporting a phase done, and report what you ran and the result.
- Keep the Commands section above accurate.

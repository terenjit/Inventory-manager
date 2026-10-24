# Textile POS — Build Plan

Each phase ends with working, tested code and a git commit. Don't start a phase until the
previous one is merged. Schema baseline: `migrations/001_init.up.sql`.

## Open decisions (confirm with the owner before the phase that needs them)

| # | Question | Needed by |
|---|---|---|
| D1 | Does the shop charge PPN on sales? If yes, what rate, and is it shown on receipts? | Phase 4 |
| D2 | Can a cashier void their own sale, or only admin? Time limit? | Phase 5 |
| D3 | Wholesale / per-roll pricing different from per-meter price? | Phase 3 |
| D4 | Receipt printing: thermal printer (58mm/80mm) or none? | Phase 10 |
| D5 | Does the admin also sell at the counter (needs a store picker in Kasir)? | Phase 4 |
| D6 | Who receives transfers at the destination: the cashier there, or admin? | Phase 6 |
| D7 | Hosting: who pays, and is it a one-time sale or monthly fee incl. maintenance? | Phase 11 |

## Phase 1 — Project skeleton
- Repo layout per CLAUDE.md, `go.mod`, Echo server with `/healthz`
- Config from env (DB DSN, JWT secret, port, CORS origin)
- docker compose: MySQL 8 for dev, separate DB for tests
- golang-migrate wired up; apply `001_init`
- sqlc config; Makefile; golangci-lint; structured logging (slog) with request IDs
- Central error → HTTP mapping; JSON decimal-as-string helpers
- **Done when:** `make up && make migrate-up && make run` works, `/healthz` checks DB, `make test` runs one integration test against MySQL.

## Phase 2 — Auth, users, store scoping
- Seed command: create stores and the first admin
- `POST /auth/login`, `POST /auth/logout`, `GET /auth/me` (JWT in httpOnly cookie, 12h)
- Passwords: bcrypt. Login rate limit. Check `is_active` on every request.
- Middleware builds `Actor{UserID, Role, StoreID}`; helpers `RequireAdmin`, `ResolveStore(actor, requested)`
- Admin CRUD for users ("Data User"), deactivate instead of delete
- **Done when:** tests prove a cashier cannot act on another store even when sending its `store_id`, and deactivated users are rejected.

## Phase 3 — Master data
- Admin CRUD: categories, suppliers, salespeople, products (soft delete, uppercase codes)
- `GET /products` with search (code, name, color, supplier), category filter, low-stock filter, pagination
- Stock shown per store: `stock: [{store_id, store_name, qty_m}]`, low-stock flag vs `min_stock_m`
- Cashier response DTO without `cost_price` / margin
- `GET /stores` (for pickers)
- **Done when:** a cashier sees both stores' stock but no cost fields; admin sees margin.

## Phase 4 — Stock package + checkout (Kasir)
- `internal/stock`: the only writer of `inventory`; functions to lock, decrement, increment, each writing `stock_movements`
- `POST /orders` based on `docs/reference/order_service.go`, refactored to use `internal/stock`
- `GET /orders/:id` (items, payments), `GET /orders` with filters: store, cashier, method, status, date range (WIB)
- Doc numbers `INV/{STORE}/{YYYYMMDD}/{NNNN}`
- **Done when:** concurrency test (e.g. 20 parallel orders on a product with stock for 10) sells exactly 10; ledger invariant holds; change is computed for cash.

## Phase 5 — Open bills, payments, void
- `POST /orders/:id/payments` (Buka / Lanjutkan Bill), status unpaid → partial → paid
- `POST /orders/:id/void` with reason: restores stock (`sale_void`), keeps record
- Optional: edit an unpaid order (Ubah). Only if D2 decided; otherwise skip.
- **Done when:** a voided order restores stock exactly once (double-void returns 409).

## Phase 6 — Transfers (Barang Keluar / Masuk)
- `POST /transfers` (send): decrements source, status `sent`
- `GET /transfers?direction=incoming&status=sent` (pending for my store)
- `POST /transfers/:id/receive`: increments destination; `POST /transfers/:id/cancel`: returns stock to source
- Cashier sends only from own store and receives only into own store
- **Done when:** stock in transit is visible, and receive/cancel cannot both happen.

## Phase 7 — Supplier purchases (Daftar Supplier / Nota)
- Admin CRUD for supplier invoices with items; `POST /purchases/:id/receive` adds stock (`purchase`)
- `POST /purchases/:id/mark-paid`; list with "belum lunas" total and due-date filter
- Optionally update `products.cost_price` from the latest purchase (ask first)
- **Done when:** receiving twice is rejected; unpaid total matches the UI card.

## Phase 8 — Stock opname
- Start session (one draft per store), add lines (snapshot `system_qty_m`), enter counts, save
- Finalize: apply `delta = counted - system_snapshot` to current stock via `opname` movements
- Printable summary data (selisih, nilai selisih at cost)
- **Done when:** a sale made during an open opname is not lost after finalize.

## Phase 9 — Excel import / export (Datasheet)
- Export products to .xlsx (excelize)
- Import: validate all rows first, report errors per row, then upsert by code in one transaction
- Stock column on import requires choosing a target store and writes `import` movements
- **Done when:** a file with one bad row imports nothing and returns the row errors.

## Phase 10 — Reports & dashboard
- Dashboard: today's omzet vs yesterday, transaction count, meters sold, low stock, 7-day chart, recent transactions
- Sales report with the Penjualan filters; payment method breakdown; sales by hour
- Stock report: kartu stok per product (movement history)
- Receipt data endpoint for printing (D4)
- `make check-ledger`: verifies invariant 2 for all rows
- **Done when:** report totals reconcile with `payments` and `stock_movements`.

## Phase 11 — Frontend integration & deploy
- Replace localStorage in `web/` with API calls, page by page
- Role-based menu: cashier sees Dashboard (own store), Kasir, Transaksi (own store), Daftar Item (read-only), Barang Masuk/Keluar
- Fix UI issues noted in review (opname subtitle, mixed language, naming)
- Deploy: single VPS, docker compose, Caddy for HTTPS, daily MySQL backup off-server with a tested restore
- **Done when:** owner can run a full day (sell, transfer, receive nota, opname) on staging.

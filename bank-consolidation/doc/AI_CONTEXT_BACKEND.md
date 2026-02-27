# Backend Context

## 1. Overview
The backend is a monolithic Go application that serves as both the API server and the static file server for the frontend. It uses the Gin web framework and GORM for database interactions with SQLite.

## 2. Technology Stack
- **Language:** Go (Golang)
- **Web Framework:** Gin (`github.com/gin-gonic/gin`)
- **Database:** SQLite
- **ORM:** GORM (`gorm.io/gorm`)
- **Containerization:** Docker (via `docker-compose`)

## 3. Project Structure
- `main.go`: Entry point. Initializes the database and starts the server.
- `db.go`: Database connection and schema migration logic.
- `internal/`
    - `controllers/`: Contains the business logic for handling HTTP requests.
        - `bank_entries.go`: CRUD for bank transactions, CSV upload, and reconciliation logic.
        - `bank_types.go`: Management of bank types (e.g., BCA, Mandiri).
        - `invoices.go`: Invoice management.
        - `categories.go`, `transactions.go`: Other supporting controllers.
    - `routes/`: Defines the API routes and static file serving configuration.
- `models/`: GORM struct definitions representing database tables.
    - `BankEntry.go`: Represents a bank transaction row.
    - `BankEntryInvoice.go`: Junction table for many-to-many relationship between BankEntries and Invoices (reconciliation).
    - `BankType.go`: Lookup table for bank codes.
    - `InvoiceHeader.go`: Invoice master data.

## 4. Key Features & Logic

### 4.1. Serving the Frontend
The backend serves the Next.js static export from the `frontend/out` directory.
- Root (`/`) serves `index.html`.
- Static assets (`/_next`, `/themes`, etc.) are served directly.
- A fallback mechanism handles client-side routing (SPA behavior) by serving `.html` files or `index.html` for unknown routes that don't start with `/api`.

### 4.2. Bank Entries & Reconciliation
- **BankEntry Model**: Stores transaction data (Date, Description, Amount, Type (CR/DB), BankCode).
    - Includes `Delta` (computed field): Difference between `Amount` and `MatchedTotal`.
    - Includes `AttachedInvoices`: List of invoices reconciled against this entry.
- **Reconciliation**:
    - Endpoint: `POST /api/v1/bank-entries/:id/reconcile`
    - Logic: Matches a bank entry with one or more invoices.
    - Validation: Ensures total matched amount does not exceed invoice total.
    - Storage: Records are saved in `bank_entry_invoices` table.

### 4.3. Pagination & Filtering
- API endpoints (e.g., `GET /api/v1/bank-entries`) support:
    - `limit` and `offset` for pagination.
    - `bankCode`, `amountType` (CR/DB), and `month` (YYYY-MM) for filtering.
    - Response format includes standard pagination metadata (`total`, `limit`, `offset`, `hasNext`).

## 5. Database Schema (Simplified)
- **bank_entries**: `id` (PK), `transaction_date`, `description`, `amount`, `amount_type`, `bank_code`, `fingerprint` (for duplicate detection).
- **bank_types**: `id` (PK), `code`, `name`, `description`.
- **invoice_headers**: `id` (PK), `invoice_no`, `total_amount`, `status`.
- **bank_entry_invoices**: `bank_entry_id` (FK), `invoice_header_id` (FK), `matched_amount`.

## 6. Running the Backend
- **Standard**: `go run .` (Starts on port 8585 by default).
- **Docker**: `docker-compose up` (Runs both backend and frontend container, though frontend container might be just for build/dev depending on config).


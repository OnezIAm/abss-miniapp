# Frontend Context

## 1. Overview
This directory contains the Next.js application for the Bank Consolidation tool. It is a single-page application (SPA) that interacts with the Go backend API.

## 2. Key Directories
- `app/(main)`: Contains the main application routes and pages.
    - `banking/`: The core banking transaction management page.
    - `bank-types/`: Management of bank types.
    - `invoices/`: Invoice listing and management.
- `app/layout`: Global layout components (TopBar, SideBar, AppMenu).
- `demo/service`: Service layer for API interaction (e.g., `BankTypeService`).
- `public/`: Static assets.

## 3. Core Features

### 3.1 Banking Page (`/banking`)
- **Purpose**: Manage bank transactions, upload CSVs, and reconcile with invoices.
- **Key Components**:
    - `DataTable`: Displays transactions (from CSV or DB).
    - `Dialog` (Reconcile): Allows selecting invoices to match against a bank entry.
    - `Dialog` (View Attached): Shows invoices linked to a reconciled entry.
- **State Management**:
    - `transactions`: Local state for CSV data.
    - `dbEntries`: State for data fetched from the backend.
    - `dataSource`: Toggles between "csv" and "db" views.

### 3.2 Reconciliation Logic
- **Frontend**:
    - User selects a bank entry -> Opens Reconcile Dialog.
    - Fetches invoices (optionally filtered by customer or status).
    - User selects invoices to match.
    - `allocations` state tracks how much of each invoice is paid by the entry.
    - Submits to `POST /api/v1/bank-entries/:id/reconcile`.
- **Backend**:
    - Stores the mapping in `bank_entry_invoices`.
    - Updates `matched_total` and `delta` for the bank entry.

### 3.3 Data Flow
1.  **CSV Upload**: User uploads CSV -> Parsed in browser -> Displayed in table -> User clicks "Upload to DB" -> `POST /api/v1/bank-entries/bulk`.
2.  **DB View**: User selects Bank & Month -> `GET /api/v1/bank-entries` -> Displayed in table.
3.  **Export**: User selects date range -> `GET /api/v1/bank-entries/export/reconciled` -> Downloads CSV.

## 4. UI/UX Guidelines
- Use PrimeReact components for consistency.
- Show clear feedback (Toasts) for success/error actions.
- Use `formatCurrency` for all monetary values.
- Ensure dates are formatted comfortably for the user (e.g., `dd/mm/yyyy`).

## 5. Deployment
- Run `npm run build` to generate the static export in `out/`.
- The `out/` directory is served by the Go backend.

# Frontend Context

## 1. Overview
The frontend is a single-page application (SPA) built with React and Next.js, styled using PrimeReact components. It serves as the primary interface for managing bank transactions, reconciliation, and bank types.

## 2. Technology Stack
- **Framework:** Next.js (Static Export mode)
- **UI Library:** PrimeReact
- **State Management:** React Hooks (`useState`, `useEffect`, `useContext`)
- **HTTP Client:** Axios
- **Styling:** SCSS, PrimeFlex, PrimeIcons

## 3. Project Structure
- `app/`
    - `(main)/`: Main application layout.
        - `banking/`: The core page for managing transactions (`page.tsx`).
        - `bank-types/`: CRUD page for Bank Types (`page.tsx`).
        - `invoices/`: View and manage invoices (`page.tsx`).
    - `layout/`: Global layout components (Topbar, Sidebar, AppMenu).
- `demo/service/`: Mock services (e.g., `ProductService`) used for initial prototyping (some still in use).
- `public/`: Static assets.

## 4. Key Features & Logic

### 4.1. Banking Page (`/banking`)
- **Functionality**:
    - Lists bank transactions with pagination.
    - Filters by Bank Code, Direction (In/Out), Month.
    - Allows manual entry creation.
    - **Reconciliation Dialog**: Matches a bank entry with multiple invoices.
    - **Attached Invoices View**: Read-only dialog showing linked invoices.
    - **Delta Calculation**: Shows discrepancy between transaction amount and matched invoice total.
- **Components**: `DataTable` (transactions), `Dialog` (reconcile/view), `Dropdown` (bank selection).

### 4.2. Bank Types Page (`/bank-types`)
- **Functionality**:
    - Lists configured bank types.
    - CRUD operations (Create, Read, Update, Delete) via a dialog form.
- **Integration**: Calls `BankTypeService` which wraps the API.

### 4.3. API Integration
- **Base URL**: `/api/v1` (Relative path, proxied by Next.js in dev or served directly by Go backend in prod).
- **Service Layer**:
    - `BankTypeService`: Methods like `getBankTypes()`, `createBankType()`.
    - `api.ts` (implied): Axios instance with interceptors for error handling.

## 5. Build & Deployment
- **Build Command**: `npm run build` (Generates static files in `out/`).
- **Deployment**: The `out/` directory is served by the Go backend.
- **Configuration**: `next.config.js` is set for `output: 'export'`.

## 6. Development Workflow
- **Dev Server**: `npm run dev` (Starts Next.js dev server on port 3000).
- **Backend API**: Needs the Go backend running on port 8585 (proxied or CORS enabled).


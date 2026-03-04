DELETE FROM bank_entry_invoices;
DELETE FROM invoice_details;
DELETE FROM invoice_headers;
DELETE FROM transaction_categories;
DELETE FROM transactions;
DELETE FROM bank_entries;
VACUUM;

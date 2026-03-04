# Panduan Cleanup Data

Dokumen ini menjelaskan langkah-langkah untuk membersihkan data pada database aplikasi Bank Consolidation.

**PERHATIAN:** Proses ini akan menghapus semua data transaksi, invoice, dan rekonsiliasi secara permanen. Pastikan Anda telah melakukan backup sebelum melanjutkan.

## Lokasi File Penting

- **Database**: `data/bank.db`
- **Script Cleanup**: `data/cleanup_data.sql`

## Langkah 1: Backup Database (Sangat Disarankan)

Sebelum menghapus data, buat salinan file database untuk keamanan.

```bash
cp data/bank.db data/bank.db.backup
```

## Langkah 2: Menjalankan Script Cleanup

Anda dapat menggunakan command line `sqlite3` yang biasanya sudah tersedia di macOS/Linux.

### Cara 1: Menggunakan Makefile (Direkomendasikan)

Jika Anda berada di terminal, cara termudah adalah menggunakan perintah make yang telah disediakan:

```bash
make clean-db
```

Perintah ini akan:

1. Meminta konfirmasi keamanan.
2. Membuat backup otomatis (`data/bank.db.bak`).
3. Menjalankan script cleanup.

### Cara 2: Menggunakan Command Line (Manual)

Jalankan perintah berikut dari root folder proyek (`bank-consolidation/`):

```bash
sqlite3 data/bank.db < data/cleanup_data.sql
```

Perintah ini akan mengeksekusi semua perintah SQL yang ada di dalam file `cleanup_data.sql` terhadap database `bank.db`.

### Cara 3: Menggunakan GUI (DB Browser for SQLite)

Jika Anda lebih nyaman menggunakan aplikasi visual:

1. Buka aplikasi **DB Browser for SQLite**.
2. Buka file database `data/bank.db`.
3. Pilih tab **Execute SQL**.
4. Buka file SQL `data/cleanup_data.sql` atau copy-paste isinya ke area editor.
5. Klik tombol **Play** (Execute all/selected SQL).
6. Klik **Write Changes** untuk menyimpan perubahan ke disk.

## Langkah 3: Verifikasi

Setelah script dijalankan, Anda dapat memverifikasi bahwa tabel telah kosong dengan perintah:

```bash
sqlite3 data/bank.db "SELECT count(*) FROM bank_entries;"
```

Output harusnya `0`.

## Apa yang Dihapus?

Script ini akan menghapus data dari tabel-tabel berikut:

1. `bank_entry_invoices`: Data relasi rekonsiliasi.
2. `invoice_details`: Detail item pada invoice.
3. `invoice_headers`: Data utama invoice.
4. `transaction_categories`: Kategori transaksi.
5. `transactions`: Transaksi manual/jurnal.
6. `bank_entries`: Data mutasi bank yang diimport.

Terakhir, perintah `VACUUM` akan dijalankan untuk mengoptimalkan ukuran file database.

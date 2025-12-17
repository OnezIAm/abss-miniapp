"use client";
import React, { useMemo, useState, useCallback, useRef } from "react";
import { DataTable } from "primereact/datatable";
import { Column } from "primereact/column";
import { Button } from "primereact/button";
import { Tag } from "primereact/tag";
import { SelectButton } from "primereact/selectbutton";
import { Dropdown } from "primereact/dropdown";
import { Calendar } from "primereact/calendar";
import { InputNumber } from "primereact/inputnumber";
import { Toast } from "primereact/toast";
import { Dialog } from "primereact/dialog";
import { api } from "@/app/lib/api";

type Transaction = {
  id: number;
  date: string;
  description: string;
  branch?: string;
  amount: number;
  direction?: "in" | "out";
  balance?: number;
  status: "posted" | "pending";
};

type Bank = "BCA" | "CIMB";
type DataSource = "csv" | "db";
type BankEntry = {
  id?: string;
  transactionDate: string;
  description: string;
  branch: string;
  amount: number;
  amountType: string;
  balance: number;
  bankCode: string;
  attachedCount?: number;
  matchedTotal?: number;
};

const BankingPage = () => {
  const toast = useRef<Toast | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [directionFilter, setDirectionFilter] = useState<"all" | "in" | "out">(
    "all"
  );
  const [rowsPerPage, setRowsPerPage] = useState<number>(5);
  const [bank, setBank] = useState<Bank | null>(null);
  const [dataSource, setDataSource] = useState<DataSource>("csv");
  const [dbEntries, setDbEntries] = useState<BankEntry[]>([]);
  const [monthFilter, setMonthFilter] = useState<Date | null>(null);
  const [dbTotal, setDbTotal] = useState<number>(0);
  const [dbOffset, setDbOffset] = useState<number>(0);
  const [recVisible, setRecVisible] = useState<boolean>(false);
  const [recEntry, setRecEntry] = useState<{ id?: string; amount: number; description: string } | null>(null);
  const [invoiceItems, setInvoiceItems] = useState<any[]>([]);
  const [invoiceTotal, setInvoiceTotal] = useState<number>(0);
  const [invoiceRows, setInvoiceRows] = useState<number>(10);
  const [invoiceFirst, setInvoiceFirst] = useState<number>(0);
  const [invoiceLoading, setInvoiceLoading] = useState<boolean>(false);
  const [invoiceSelection, setInvoiceSelection] = useState<any[]>([]);
  const [recIncludeIds, setRecIncludeIds] = useState<string[]>([]);
  const [reconciliations, setReconciliations] = useState<Record<string, { invoiceIds: string[]; none?: boolean; delta: number }>>({});
  const [allocations, setAllocations] = useState<Record<string, number>>({});

  const totalBalance = useMemo(() => {
    return transactions.reduce((acc, t) => acc + t.amount, 0);
  }, [transactions]);

  const formatCurrency = (value: number) =>
    value.toLocaleString("en-US", {
      style: "currency",
      currency: "IDR",
    });

  const parseCSV = (text: string): string[][] => {
    const rows: string[][] = [];
    let current: string[] = [];
    let field = "";
    let inQuotes = false;

    const pushField = () => {
      current.push(field.trim());
      field = "";
    };

    const pushRow = () => {
      // ignore empty rows
      if (current.length > 0 && current.some((f) => f.length > 0)) {
        rows.push(current);
      }
      current = [];
    };

    for (let i = 0; i < text.length; i++) {
      const c = text[i];
      const isNewline = c === "\n";
      if (c === '"') {
        if (inQuotes && text[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = !inQuotes;
        }
      } else if (c === "," && !inQuotes) {
        pushField();
      } else if (isNewline && !inQuotes) {
        pushField();
        pushRow();
      } else if (c === "\r") {
        // skip
      } else {
        field += c;
      }
    }
    // flush last
    pushField();
    pushRow();

    return rows;
  };

  const toNumber = (s: string): number | undefined => {
    if (!s) return undefined;
    const cleaned = s.replace(/[^\d.,\-]/g, "").replace(/,/g, "");
    const num = parseFloat(cleaned);
    return isNaN(num) ? undefined : num;
  };

  const detectDirection = (s: string): "in" | "out" | undefined => {
    if (!s) return undefined;
    const upper = s.toUpperCase();
    if (upper.includes("CR")) return "in";
    if (upper.includes("DR") || upper.includes("DB")) return "out";
    return undefined;
  };

  const parseStatementRows = (rows: string[][]): Transaction[] => {
    const headerIdx = rows.findIndex(
      (r) =>
        r.length >= 5 &&
        r[0].toLowerCase().includes("tanggal") &&
        r[1].toLowerCase().includes("keterangan") &&
        r[2].toLowerCase().includes("cabang") &&
        r[3].toLowerCase().includes("jumlah") &&
        r[4].toLowerCase().includes("saldo")
    );
    if (headerIdx < 0) return [];

    const dataRows = rows.slice(headerIdx + 1);
    const txs: Transaction[] = [];
    let id = 1;

    for (const r of dataRows) {
      if (r.length < 5) continue;
      const date = r[0];
      const description = r[1];
      const branch = r[2];
      const jumlah = r[3];
      const saldo = r[4];

      const direction = detectDirection(jumlah);
      const amountAbs = toNumber(jumlah);
      const balanceNum = toNumber(saldo);
      if (!amountAbs) continue;

      const amount =
        direction === "out" ? -Math.abs(amountAbs) : Math.abs(amountAbs);

      txs.push({
        id: id++,
        date,
        description,
        branch: (branch || "").trim() || "UNKNOWN",
        amount,
        direction,
        balance: balanceNum,
        status: "posted",
      });
    }
    return txs;
  };

  const isDate = (s: string) => /^\d{1,2}\/\d{1,2}\/\d{4}$/.test(s.trim());

  const parseGenericRows = (rows: string[][]): Transaction[] => {
    const txs: Transaction[] = [];
    let id = 1;
    for (const r of rows) {
      if (r.length < 4) continue;
      if (!isDate(r[0])) continue;
      const date = r[0];
      const description = r[1];
      const branch = r[2] ?? "";
      const jumlah = r[3];
      const saldo = r[4] ?? "";
      const direction = detectDirection(jumlah);
      const amountAbs = toNumber(jumlah);
      const balanceNum = toNumber(saldo);
      if (!amountAbs) continue;
      const amount =
        direction === "out" ? -Math.abs(amountAbs) : Math.abs(amountAbs);
      txs.push({
        id: id++,
        date,
        description,
        branch: (branch || "").trim() || "UNKNOWN",
        amount,
        direction,
        balance: balanceNum,
        status: "posted",
      });
    }
    return txs;
  };

  const parseByBank = (selectedBank: Bank, rows: string[][]): Transaction[] => {
    if (selectedBank === "BCA") {
      const bca = parseStatementRows(rows);
      if (bca.length > 0) return bca;
      return parseGenericRows(rows);
    }
    return parseGenericRows(rows);
  };

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;
      if (!bank) return;
      const reader = new FileReader();
      reader.onload = () => {
        const text = (reader.result as string) || "";
        const rows = parseCSV(text);
        const txs = parseByBank(bank, rows);
        setTransactions(txs);
        setDataSource("csv");
      };
      reader.readAsText(file);
    },
    [bank]
  );

  const totalIn = useMemo(
    () =>
      transactions
        .filter((t) => t.amount > 0)
        .reduce((acc, t) => acc + t.amount, 0),
    [transactions]
  );
  const totalOut = useMemo(
    () =>
      transactions
        .filter((t) => t.amount < 0)
        .reduce((acc, t) => acc + Math.abs(t.amount), 0),
    [transactions]
  );
  const filteredTransactions = useMemo(() => {
    if (directionFilter === "all") return transactions;
    return transactions.filter((t) =>
      directionFilter === "in" ? t.amount > 0 : t.amount < 0
    );
  }, [transactions, directionFilter]);

  const formatDateToISO = (dateStr: string) => {
    const parts = dateStr.split("/");
    if (parts.length === 3) {
      const d = parts[0].padStart(2, "0");
      const m = parts[1].padStart(2, "0");
      const y = parts[2];
      return `${y}-${m}-${d}`;
    }
    const d2 = new Date(dateStr);
    if (!isNaN(d2.getTime())) {
      const y = d2.getFullYear();
      const m = `${d2.getMonth() + 1}`.padStart(2, "0");
      const d = `${d2.getDate()}`.padStart(2, "0");
      return `${y}-${m}-${d}`;
    }
    return dateStr;
  };

  const formatDisplayDate = (s: string) => {
    const iso = formatDateToISO(s);
    const d = new Date(iso);
    return isNaN(d.getTime()) ? s : d.toLocaleDateString();
  };

  const toBankEntry = (t: Transaction): BankEntry => ({
    transactionDate: formatDateToISO(t.date),
    description: t.description,
    branch: (t.branch || "").trim() || "UNKNOWN",
    amount: Math.abs(t.amount),
    amountType: t.amount >= 0 ? "CR" : "DB",
    balance: t.balance ?? 0,
    bankCode: bank || "",
  });

  const uploadToDatabase = async () => {
    const toUpload = filteredTransactions;
    if (!toUpload.length) {
      toast.current?.show({
        severity: "warn",
        summary: "No Data",
        detail: "Nothing to upload",
        life: 3000,
      });
      return;
    }
    try {
      const payload = toUpload.map(toBankEntry);
      const { data } = await api.post("/bank-entries/bulk", payload);
      toast.current?.show({
        severity: "success",
        summary: "Uploaded",
        detail:
          typeof data === "object" && data
            ? `Inserted ${data.inserted ?? "-"}, skipped ${data.skipped ?? "-"}`
            : `Uploaded ${payload.length} entries`,
        life: 3000,
      });
      await refreshDatabase();
      setDataSource("db");
    } catch (e: any) {
      const status = e?.response?.status;
      const detail =
        e?.response?.data?.message ||
        (typeof e?.response?.data === "string" ? e.response.data : undefined) ||
        e?.message ||
        "Upload error";
      console.error("Upload error", e);
      toast.current?.show({
        severity: "error",
        summary: status ? `Error ${status}` : "Error",
        detail,
        life: 6000,
      });
    }
  };

  const normalizeResponse = (respData: any) => {
    const entries =
      Array.isArray(respData)
        ? respData
        : Array.isArray(respData?.data)
        ? respData.data
        : Array.isArray(respData?.items)
        ? respData.items
        : [];
    const p = respData?.pagination || {};
    return {
      entries,
      total: typeof p.total === "number" ? p.total : entries.length,
      limit: typeof p.limit === "number" ? p.limit : rowsPerPage,
      offset: typeof p.offset === "number" ? p.offset : 0,
      hasNext: !!p.hasNext,
    };
  };

  const refreshDatabase = async (
    offset = 0,
    limit = rowsPerPage,
    opts?: { amountType?: string; month?: string }
  ) => {
    try {
      if (!bank) {
        toast.current?.show({
          severity: "warn",
          summary: "Select Bank",
          detail: "Please select a bank to load entries",
          life: 3000,
        });
        return;
      }
      const amountTypeParam =
        opts?.amountType !== undefined
          ? opts.amountType
          : directionFilter === "in"
          ? "CR"
          : directionFilter === "out"
          ? "DB"
          : undefined;
      const monthParam =
        opts?.month !== undefined ? opts.month : selectedMonthKey || undefined;
      const { data } = await api.get<BankEntry[]>("/bank-entries", {
        params: {
          bankCode: bank,
          limit,
          offset,
          amountType: amountTypeParam,
          month: monthParam,
        },
      });
      const norm = normalizeResponse(data);
      setDbEntries(norm.entries);
      setDbTotal(norm.total);
      setDbOffset(norm.offset);
      toast.current?.show({
        severity: "info",
        summary: "Loaded",
        detail: `Loaded ${norm.entries.length}/${norm.total} entries`,
        life: 3000,
      });
    } catch (e: any) {
      const status = e?.response?.status;
      const detail =
        e?.response?.data?.message ||
        (typeof e?.response?.data === "string" ? e.response.data : undefined) ||
        e?.message ||
        "Fetch error";
      console.error("Refresh DB error", e);
      toast.current?.show({
        severity: "error",
        summary: status ? `Error ${status}` : "Error",
        detail,
        life: 6000,
      });
    }
  };


  const selectedMonthKey = useMemo(() => {
    if (!monthFilter) return "";
    const y = monthFilter.getFullYear();
    const m = `${monthFilter.getMonth() + 1}`.padStart(2, "0");
    return `${y}-${m}`;
  }, [monthFilter]);

  const filteredDbTransactions = useMemo(() => {
    if (!bank) return [];
    let data = Array.isArray(dbEntries) ? dbEntries : [];
    data = data.filter((e) => e.bankCode === bank);
    return data.map((e, idx) => ({
      id: idx + 1,
      entryId: e.id,
      date: e.transactionDate,
      description: e.description,
      branch: e.branch,
      amount: e.amountType === "CR" ? e.amount : -e.amount,
      direction: e.amountType === "CR" ? "in" : "out",
      balance: e.balance,
      status: "posted",
      bankCode: e.bankCode,
      attachedCount: e.attachedCount,
      matchedTotal: e.matchedTotal,
    }));
  }, [dbEntries, directionFilter, selectedMonthKey]);

  const tableData =
    dataSource === "csv"
      ? filteredTransactions.map((t) => ({ ...t, bankCode: bank || "" }))
      : filteredDbTransactions;

  const refreshInvoices = async (offset = 0, limit = invoiceRows, includeIds?: string[]) => {
    try {
      setInvoiceLoading(true);
      const params: any = { limit, offset, excludeFullyPaid: true };
      const idsToInclude = includeIds || recIncludeIds;
      if (idsToInclude.length > 0) {
        params.includeIds = idsToInclude.join(",");
      }
      const { data } = await api.get("/invoices", { params });
      const entries =
        Array.isArray(data)
          ? data
          : Array.isArray(data?.data)
          ? data.data
          : Array.isArray(data?.items)
          ? data.items
          : [];
      const p = data?.pagination || {};
      setInvoiceItems(entries);
      setInvoiceTotal(typeof p.total === "number" ? p.total : entries.length);
      setInvoiceRows(typeof p.limit === "number" ? p.limit : limit);
      setInvoiceFirst(typeof p.offset === "number" ? p.offset : offset);
      return entries;
    } catch (e: any) {
      const status = e?.response?.status;
      const detail =
        e?.response?.data?.message ||
        (typeof e?.response?.data === "string" ? e.response.data : undefined) ||
        e?.message ||
        "Fetch error";
      toast.current?.show({
        severity: "error",
        summary: status ? `Error ${status}` : "Error",
        detail,
        life: 6000,
      });
      return [];
    } finally {
      setInvoiceLoading(false);
    }
  };

  const openReconcile = async (row: any) => {
    if (!row?.entryId) {
      toast.current?.show({
        severity: "warn",
        summary: "Unavailable",
        detail: "Reconciliation only available for database records",
        life: 3000,
      });
      return;
    }
    setRecEntry({ id: row.entryId, amount: Math.abs(row.amount), description: row.description, });
    
    let ids: string[] = [];
    let selection: any[] = [];

    if (row.attachedCount > 0) {
      try {
        const { data } = await api.get(`/bank-entries/${row.entryId}/invoices`);
        selection = data;
        ids = data.map((x: any) => String(x.id));
        
        const newAllocations: Record<string, number> = {};
        data.forEach((d: any) => {
            newAllocations[d.id] = d.matchedAmount;
        });
        setAllocations(newAllocations);

        setReconciliations((prev) => ({
          ...prev,
          [row.entryId]: { invoiceIds: ids, delta: Math.abs(row.amount) - (row.matchedTotal || 0) },
        }));
      } catch (e) {
        console.error("Fetch attached invoices error", e);
      }
    } else {
        setAllocations({});
    }

    setRecIncludeIds(ids);
    setRecVisible(true);
    const fetchedItems = await refreshInvoices(0, invoiceRows, ids);
    
    // Ensure selection uses the exact objects from the table data to avoid reference issues
    // and ensure they are visually selected.
    if (ids.length > 0 && fetchedItems && fetchedItems.length > 0) {
        const attachedSet = new Set(ids);
        const validSelection = fetchedItems.filter((item: any) => attachedSet.has(String(item.id)));
        setInvoiceSelection(validSelection);
    } else {
        setInvoiceSelection([]);
    }
  };

  const invoiceSum = useMemo(() => {
    return invoiceSelection.reduce((acc, it) => {
      // Use allocation if set, otherwise 0
      const alloc = allocations[it.id];
      if (typeof alloc === 'number') return acc + alloc;
      
      const val = typeof it.totalAmount === "number" ? it.totalAmount : Number(it.totalAmount || 0);
      return acc + (isNaN(val) ? 0 : val);
    }, 0);
  }, [invoiceSelection, allocations]);

  const currentDelta = useMemo(() => {
    const amt = recEntry ? recEntry.amount : 0;
    return amt - invoiceSum;
  }, [recEntry, invoiceSum]);

  const saveReconcile = async (entryId: string, invoices: any[], note?: string) => {
    try {
      const payload = {
        invoices: invoices.map((inv) => ({
          id: inv.id,
          amount: allocations[inv.id] !== undefined ? allocations[inv.id] : inv.totalAmount, 
        })),
        note,
        mode: "replace",
      };
      await api.post(`/bank-entries/${entryId}/reconcile`, payload);
      toast.current?.show({
        severity: "success",
        summary: "Reconciled",
        detail: "Saved successfully",
      });
      const newIds = invoices.map((i) => String(i.id));
      setRecIncludeIds(newIds);
      await refreshInvoices(invoiceFirst, invoiceRows, newIds);
    } catch (e: any) {
      console.error("Reconcile error", e);
      toast.current?.show({
        severity: "error",
        summary: "Error",
        detail: "Failed to save reconciliation",
      });
    }
  };

  const attachSelected = async () => {
    if (!recEntry?.id) return;
    // const ids = invoiceSelection.map((x) => String(x.id));

    await saveReconcile(recEntry.id, invoiceSelection);

    try {
      const { data: updatedEntry } = await api.get<BankEntry>(
        `/bank-entries/${recEntry.id}`
      );
      setDbEntries((prev) =>
        prev.map((e) => (e.id === recEntry.id ? updatedEntry : e))
      );
      // Clear local override so table uses fresh DB data
      setReconciliations((prev) => {
        const next = { ...prev };
        delete next[recEntry.id!];
        return next;
      });
    } catch (e) {
      console.error("Failed to refresh entry", e);
    }

    setRecVisible(false);
  };

  const setNoRecords = async () => {
    if (!recEntry?.id) return;

    await saveReconcile(recEntry.id, []);

    try {
      const { data: updatedEntry } = await api.get<BankEntry>(
        `/bank-entries/${recEntry.id}`
      );
      setDbEntries((prev) =>
        prev.map((e) => (e.id === recEntry.id ? updatedEntry : e))
      );
      setReconciliations((prev) => {
        const next = { ...prev };
        delete next[recEntry.id!];
        return next;
      });
    } catch (e) {
      console.error("Failed to refresh entry", e);
    }

    setInvoiceSelection([]);
    setRecVisible(false);
  };

  return (
    <div className="grid">
      <div className="col-12">
        <div className="card">
          <div className="flex justify-content-between align-items-center mb-3">
            <h5>Banking Overview</h5>
            <div className="flex gap-2 align-items-center">
              <Dropdown
                value={bank}
                onChange={(e) => setBank(e.value)}
                options={[
                  { label: "BCA", value: "BCA" },
                  { label: "CIMB", value: "CIMB" },
                ]}
                placeholder="Select Bank"
              />
              <input
                type="file"
                accept=".csv,text/csv"
                onChange={handleFileChange}
                disabled={!bank}
              />
              <Button
                label="Clear"
                icon="pi pi-trash"
                className="p-button-secondary"
                onClick={() => setTransactions([])}
              />
              <Button
                label="Upload to DB"
                icon="pi pi-upload"
                onClick={uploadToDatabase}
                disabled={!filteredTransactions.length || !bank}
              />
              <Button
                label="Refresh DB"
                icon="pi pi-sync"
                severity="secondary"
                onClick={() => refreshDatabase(0, rowsPerPage)}
                disabled={!bank}
              />
            </div>
          </div>
          <div className="grid">
            <div className="col-12 md:col-4">
              <div className="flex justify-content-between mb-3">
                <div>
                  <span className="block text-500 font-medium mb-2">
                    Accounts
                  </span>
                  <div className="text-900 font-medium text-xl">1</div>
                </div>
                <div
                  className="flex align-items-center justify-content-center bg-blue-100 border-round"
                  style={{ width: "2.5rem", height: "2.5rem" }}
                >
                  <i className="pi pi-wallet text-blue-500 text-xl" />
                </div>
              </div>
              <span className="text-500">Active accounts</span>
            </div>
            <div className="col-12 md:col-4">
              <div className="flex justify-content-between mb-3">
                <div>
                  <span className="block text-500 font-medium mb-2">
                    Net Change
                  </span>
                  <div className="text-900 font-medium text-xl">
                    {formatCurrency(totalBalance)}
                  </div>
                </div>
                <div
                  className="flex align-items-center justify-content-center bg-green-100 border-round"
                  style={{ width: "2.5rem", height: "2.5rem" }}
                >
                  <i className="pi pi-dollar text-green-500 text-xl" />
                </div>
              </div>
              <span className="text-500">Sum of money in minus out</span>
            </div>
            <div className="col-12 md:col-4">
              <div className="flex justify-content-between mb-3">
                <div>
                  <span className="block text-500 font-medium mb-2">
                    Totals
                  </span>
                  <div className="text-900 font-medium text-sm">
                    In:{" "}
                    <span className="text-green-600">
                      {formatCurrency(totalIn)}
                    </span>{" "}
                    · Out:{" "}
                    <span className="text-red-500">
                      {formatCurrency(totalOut)}
                    </span>
                  </div>
                </div>
                <div
                  className="flex align-items-center justify-content-center bg-cyan-100 border-round"
                  style={{ width: "2.5rem", height: "2.5rem" }}
                >
                  <i className="pi pi-refresh text-cyan-500 text-xl" />
                </div>
              </div>
              <span className="text-500">Calculated from uploaded CSV</span>
            </div>
          </div>
        </div>
      </div>

      <div className="col-12">
        <div className="card">
          <div className="flex justify-content-between align-items-center mb-3">
            <h5>Transactions</h5>
            <div className="flex align-items-center gap-3">
              <div className="flex align-items-center gap-2">
                <span className="text-600">Source</span>
                <SelectButton
                  value={dataSource}
                  onChange={async (e) => {
                    if (e.value === "db" && !bank) {
                      toast.current?.show({
                        severity: "warn",
                        summary: "Select Bank",
                        detail: "Please select a bank to view database entries",
                        life: 3000,
                      });
                      return;
                    }
                    if (e.value === "db" && bank) {
                      await refreshDatabase(0, rowsPerPage);
                    }
                    setDataSource(e.value);
                  }}
                  options={[
                    { label: "CSV Data", value: "csv" },
                    { label: "Database", value: "db" },
                  ]}
                />
              </div>
              <div className="flex align-items-center gap-2">
                <span className="text-600">Show</span>
                <Dropdown
                  value={rowsPerPage}
                  options={[5, 10, 20, 50].map((n) => ({ label: String(n), value: n }))}
                  onChange={async (e) => {
                    setRowsPerPage(e.value);
                    if (dataSource === "db" && bank) {
                      await refreshDatabase(0, e.value);
                    }
                  }}
                  placeholder="Rows"
                />
                <span className="text-600">rows</span>
              </div>
              <div className="flex align-items-center gap-2">
                <span className="text-600">Filter</span>
                <SelectButton
                  value={directionFilter}
                  onChange={async (e) => {
                    setDirectionFilter(e.value);
                    if (dataSource === "db" && bank) {
                      const at =
                        e.value === "in"
                          ? "CR"
                          : e.value === "out"
                          ? "DB"
                          : undefined;
                      await refreshDatabase(0, rowsPerPage, {
                        amountType: at,
                        month: selectedMonthKey || undefined,
                      });
                    }
                  }}
                  options={[
                    { label: "All", value: "all" },
                    { label: "In", value: "in" },
                    { label: "Out", value: "out" },
                  ]}
                />
              </div>
              <div className="flex align-items-center gap-2">
                <span className="text-600">Month</span>
                <Calendar
                  value={monthFilter}
                  onChange={async (e) => {
                    setMonthFilter(e.value as Date);
                    if (dataSource === "db" && bank) {
                      const d = e.value as Date;
                      const y = d.getFullYear();
                      const m = `${d.getMonth() + 1}`.padStart(2, "0");
                      const mk = `${y}-${m}`;
                      const at =
                        directionFilter === "in"
                          ? "CR"
                          : directionFilter === "out"
                          ? "DB"
                          : undefined;
                      await refreshDatabase(0, rowsPerPage, {
                        amountType: at,
                        month: mk,
                      });
                    }
                  }}
                  view="month"
                  dateFormat="mm/yy"
                  showIcon
                  appendTo="self"
                />
              </div>
            </div>
          </div>
          <DataTable
            value={tableData}
            rows={rowsPerPage}
            rowsPerPageOptions={[5, 10, 20, 50]}
            paginator
            lazy={dataSource === "db"}
            totalRecords={dataSource === "db" ? dbTotal : tableData.length}
            first={dataSource === "db" ? dbOffset : 0}
            onPage={async (e) => {
              if (dataSource === "db" && bank) {
                setRowsPerPage(e.rows);
                await refreshDatabase(e.first, e.rows);
              }
            }}
            responsiveLayout="scroll"
            dataKey="id"
          >
            <Column
              field="date"
              header="Date"
              style={{ width: "15%" }}
              body={(t: Transaction) => <span>{formatDisplayDate(t.date)}</span>}
            />
            <Column field="bankCode" header="Bank" style={{ width: "10%" }} />
            <Column
              field="description"
              header="Description"
              style={{ width: "40%" }}
            />
            <Column field="branch" header="Branch" style={{ width: "10%" }} />
            <Column
              field="amount"
              header="Amount"
              style={{ width: "15%" }}
              body={(t: Transaction) => (
                <span
                  className={t.amount < 0 ? "text-red-500" : "text-green-600"}
                >
                  {formatCurrency(t.amount)}
                </span>
              )}
            />
            <Column
              header="Direction"
              style={{ width: "10%" }}
              body={(t: Transaction) =>
                t.direction ? (
                  <Tag
                    value={t.direction === "in" ? "In" : "Out"}
                    severity={t.direction === "in" ? "success" : "danger"}
                  />
                ) : (
                  <Tag value="Unknown" severity="warning" />
                )
              }
            />
            <Column
              field="balance"
              header="Balance"
              style={{ width: "10%" }}
              body={(t: Transaction) =>
                t.balance !== undefined ? (
                  <span>{formatCurrency(t.balance)}</span>
                ) : (
                  <span>-</span>
                )
              }
            />
            <Column
              field="status"
              header="Status"
              style={{ width: "15%" }}
              body={(t: Transaction) => (
                <span
                  className={t.status === "pending" ? "text-600" : "text-700"}
                >
                  {t.status === "pending" ? "Pending" : "Posted"}
                </span>
              )}
            />
            <Column
              header="Delta"
              style={{ width: "10%" }}
              body={(row: any) => {
                const rec = row.entryId ? reconciliations[row.entryId] : undefined;
                let val = 0;
                let hasData = false;

                if (rec) {
                  val = rec.delta;
                  hasData = true;
                } else if (typeof row.matchedTotal === "number" && row.attachedCount > 0) {
                  val = Math.abs(row.amount) - row.matchedTotal;
                  hasData = true;
                }

                if (!hasData) return <span>-</span>;
                const ok = Math.abs(val) < 0.0001;
                return <Tag value={ok ? "0" : formatCurrency(val)} severity={ok ? "success" : "warning"} />;
              }}
            />
            <Column
              header="Actions"
              style={{ width: "12%" }}
              body={(row: any) => (
                <Button
                  label="Reconcile"
                  icon="pi pi-link"
                  onClick={() => openReconcile(row)}
                  disabled={dataSource !== "db" || !row.entryId}
                />
              )}
            />
          </DataTable>
          <div className="mt-3 text-600">
            Upload a CSV in the bank statement format to populate the table. We
            auto-detect money in/out using the "CR"/"DR"/"DB" suffix in the
            Jumlah column.
          </div>
        </div>
      </div>
      <Dialog
        visible={recVisible}
        onHide={() => setRecVisible(false)}
        header="Reconcile Invoices"
        style={{ width: "60vw" }}
        modal
      >
        <div className="mb-3">
          <div className="text-700">
            Record:{" "}
            <strong>{recEntry ? formatCurrency(recEntry.amount) : "-"}</strong>
            {" · "}
            Invoices Sum: <strong>{formatCurrency(invoiceSum)}</strong>
            {" · "}
            Delta:{" "}
            <strong className={Math.abs(currentDelta) < 0.0001 ? "text-green-600" : "text-orange-500"}>
              {formatCurrency(currentDelta)}
            </strong>
          </div>
        </div>
        <DataTable
          value={invoiceItems}
          paginator
          rows={invoiceRows}
          rowsPerPageOptions={[10, 20, 50]}
          lazy
          totalRecords={invoiceTotal}
          first={invoiceFirst}
          onPage={(e) => {
            setInvoiceRows(e.rows);
            setInvoiceFirst(e.first);
            refreshInvoices(e.first, e.rows);
          }}
          loading={invoiceLoading}
          responsiveLayout="scroll"
          selection={invoiceSelection}
          onSelectionChange={(e) => {
            const newSelection = e.value;
            setInvoiceSelection(newSelection);
            setAllocations((prev) => {
              const next = { ...prev };
              newSelection.forEach((item: any) => {
                // If not set, init default
                if (next[item.id] === undefined) {
                  // If it's a new item, default to remaining amount
                  const remaining = (Number(item.totalAmount) || 0) - (Number(item.paidAmount) || 0);
                  next[item.id] = remaining > 0 ? remaining : 0;
                }
              });
              return next;
            });
          }}
          dataKey="id"
          isDataSelectable={(e) => {
            const paid = Number(e.data.paidAmount || 0);
            const total = Number(e.data.totalAmount || 0);
            const isSelected = invoiceSelection.some((sel) => sel.id === e.data.id);
            return paid < total || isSelected;
          }}
        >
          <Column selectionMode="multiple" headerStyle={{ width: "3rem" }} />
          <Column field="invoiceNo" header="Invoice No" />
          <Column field="invoiceDate" header="Date" body={(r) => <span>{formatDisplayDate(r.invoiceDate)}</span>} />
          <Column field="customerName" header="Customer" />
          <Column field="totalAmount" header="Total" body={(r) => <span>{formatCurrency(Number(r.totalAmount || 0))}</span>} />
          <Column field="paidAmount" header="Paid" body={(r) => <span>{formatCurrency(Number(r.paidAmount || 0))}</span>} />
          <Column
            header="To Pay"
            body={(r) => {
              // Only show input if selected
              const isSelected = invoiceSelection.some((s) => s.id === r.id);
              if (!isSelected) return null;

              return (
                <InputNumber
                  value={allocations[r.id] ?? 0}
                  onValueChange={(e) => {
                    setAllocations((prev) => ({ ...prev, [r.id]: e.value || 0 }));
                  }}
                  mode="currency"
                  currency="IDR"
                  locale="en-US"
                  minFractionDigits={2}
                />
              );
            }}
          />
          <Column
            header="Payment Status"
            body={(r) => {
              const paid = Number(r.paidAmount || 0);
              const total = Number(r.totalAmount || 0);
              if (paid >= total) return <Tag severity="success" value="Paid" />;
              if (paid > 0) return <Tag severity="warning" value="Partial" />;
              return <Tag severity="info" value="Unpaid" />;
            }}
          />
        </DataTable>
        <div className="flex justify-content-end gap-2 mt-3">
          <Button label="Set No Records" severity="secondary" onClick={setNoRecords} />
          <Button label="Attach Selected" icon="pi pi-check" onClick={attachSelected} />
        </div>
      </Dialog>
      <Toast ref={toast} />
    </div>
  );
};

export default BankingPage;

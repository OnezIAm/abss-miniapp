"use client";
import React, { useMemo, useState, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
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
import { BankTypeService } from "@/demo/service/BankTypeService";

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

interface BankModel {
  id: string;
  code: string;
  name: string;
  description: string;
  format?: string;
}

type DataSource = "csv" | "db";

type AttachedInvoice = {
  id: string;
  invoiceNo: string;
  invoiceDate: string;
  customerName: string;
  status: string;
  totalAmount: number;
  matchedAmount: number;
};

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
  delta?: number;
  attachedInvoices?: AttachedInvoice[];
};

const BankingPage = () => {
  const router = useRouter();
  const toast = useRef<Toast | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [directionFilter, setDirectionFilter] = useState<"all" | "in" | "out">(
    "all"
  );
  const [rowsPerPage, setRowsPerPage] = useState<number>(5);
  const [bank, setBank] = useState<string | null>(null);
  const [bankList, setBankList] = useState<BankModel[]>([]);
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

  const [viewAttachedVisible, setViewAttachedVisible] = useState(false);
  const [viewAttachedEntry, setViewAttachedEntry] = useState<{ id?: string; description: string } | null>(null);
  const [viewAttachedInvoices, setViewAttachedInvoices] = useState<AttachedInvoice[]>([]);

  // Manual Entry State
  const [manualVisible, setManualVisible] = useState(false);
  const [manualData, setManualData] = useState({
    date: new Date(),
    description: "",
    amount: 0,
    type: "CR",
  });

  // Finalize
  const [selectedDbEntries, setSelectedDbEntries] = useState<any[]>([]);
  const [finalizeLoading, setFinalizeLoading] = useState(false);

  React.useEffect(() => {
    BankTypeService.getBankTypes()
      .then((data) => setBankList(data))
      .catch((err) => console.error("Failed to fetch banks", err));
  }, []);

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
        balance: amountAbs, // Use amount as initial balance (remaining to reconcile)
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
        balance: amountAbs, // Use amount as initial balance (remaining to reconcile)
        status: "posted",
      });
    }
    return txs;
  };

  const parseByBank = (selectedBank: string, rows: string[][]): Transaction[] => {
    // Find the bank object to get its format
    const bankObj = bankList.find((b) => b.code === selectedBank);
    // Default to GENERIC if not found or no format specified
    const format = bankObj?.format || "GENERIC";

    if (format === "BCA") {
      const bca = parseStatementRows(rows);
      if (bca.length > 0) return bca;
      return parseGenericRows(rows);
    }
    
    // For DANAMON or GENERIC, use generic parser
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
    [bank, bankList]
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
    opts?: { amountType?: string; month?: string; bankCode?: string }
  ) => {
    try {
      const activeBank = opts?.bankCode || bank;
      if (!activeBank) {
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
          bankCode: activeBank,
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
      // Use delta as balance (remaining to reconcile)
      balance:
        typeof e.delta === "number"
          ? e.delta
          : Math.abs(e.amount) - (e.matchedTotal || 0),
      status: "posted",
      bankCode: e.bankCode,
      attachedCount: e.attachedCount,
      matchedTotal: e.matchedTotal,
      delta: e.delta,
      attachedInvoices: e.attachedInvoices,
    }));
  }, [dbEntries, bank]);

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

    try {
      const { data } = await api.get(`/bank-entries/${row.entryId}/invoices`);
      selection = data;
      ids = data.map((x: any) => String(x.id));

      const newAllocations: Record<string, number> = {};
      data.forEach((d: any) => {
        newAllocations[d.id] = d.matchedAmount;
      });
      setAllocations(newAllocations);

      const baseDelta =
        typeof row.delta === "number"
          ? row.delta
          : Math.abs(row.amount) - (row.matchedTotal || 0);

      setReconciliations((prev) => ({
        ...prev,
        [row.entryId]: { invoiceIds: ids, delta: baseDelta },
      }));
    } catch (e) {
      console.error("Fetch attached invoices error", e);
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
      
      const total = typeof it.totalAmount === "number" ? it.totalAmount : Number(it.totalAmount || 0);
      const paid = typeof it.paidAmount === "number" ? it.paidAmount : Number(it.paidAmount || 0);
      const remaining = total - paid;
      return acc + (isNaN(remaining) ? 0 : Math.max(remaining, 0));
    }, 0);
  }, [invoiceSelection, allocations]);

  const currentDelta = useMemo(() => {
    const amt = recEntry ? recEntry.amount : 0;
    return amt - invoiceSum;
  }, [recEntry, invoiceSum]);

  const openViewAttached = async (row: any) => {
    if (!row?.entryId) {
      toast.current?.show({
        severity: "warn",
        summary: "Unavailable",
        detail: "Attached invoices only available for database records",
        life: 3000,
      });
      return;
    }
    try {
      const { data } = await api.get<AttachedInvoice[]>(
        `/bank-entries/${row.entryId}/invoices`
      );
      setViewAttachedEntry({ id: row.entryId, description: row.description });
      setViewAttachedInvoices(Array.isArray(data) ? data : []);
      setViewAttachedVisible(true);
    } catch (e: any) {
      console.error("Failed to load attached invoices", e);
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
    }
  };

  const saveReconcile = async (entryId: string, invoices: any[], note?: string) => {
    try {
      const payload = {
        invoices: invoices.map((inv) => ({
          id: inv.id,
          amount:
            allocations[inv.id] !== undefined
              ? allocations[inv.id]
              : Math.max(
                  (Number(inv.totalAmount) || 0) - (Number(inv.paidAmount) || 0),
                  0
                ),
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
      setReconciliations((prev) => {
        const next = { ...prev };
        delete next[recEntry.id!];
        return next;
      });
      setInvoiceSelection([]);
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
      setInvoiceSelection([]);
    } catch (e) {
      console.error("Failed to refresh entry", e);
    }

    setRecVisible(false);
  };

  const saveManualEntry = async () => {
    if (!bank || !manualData.amount || !manualData.description) {
        toast.current?.show({ severity: "warn", summary: "Validation", detail: "Please fill all fields" });
        return;
    }
    try {
      await api.post("/bank-entries", {
        transactionDate: manualData.date,
        description: manualData.description,
        branch: "MANUAL",
        amount: manualData.amount,
        amountType: manualData.type,
        bankCode: bank,
        balance: 0
      });
      toast.current?.show({ severity: "success", summary: "Saved", detail: "Entry added" });
      setManualVisible(false);
      setManualData({ date: new Date(), description: "", amount: 0, type: "CR" });
      refreshDatabase();
    } catch (e) {
      console.error(e);
      toast.current?.show({ severity: "error", summary: "Error", detail: "Failed to save" });
    }
  };

  const [exportVisible, setExportVisible] = useState(false);
  const [exportRange, setExportRange] = useState<(Date | null)[] | null>(null);

  const handleExport = async () => {
    if (!exportRange || !exportRange[0] || !exportRange[1]) {
      toast.current?.show({ severity: "warn", summary: "Validation", detail: "Please select start and end date" });
      return;
    }
    const [start, end] = exportRange;
    if (!start || !end) return;

    try {
      const formatDate = (d: Date) => {
        const year = d.getFullYear();
        const month = String(d.getMonth() + 1).padStart(2, '0');
        const day = String(d.getDate()).padStart(2, '0');
        return `${year}-${month}-${day}`;
      };
      const startStr = formatDate(start);
      const endStr = formatDate(end);
      const url = `/bank-entries/export/reconciled?startDate=${startStr}&endDate=${endStr}`;
      
      // Trigger download
      const response = await api.get(url, { responseType: "blob" });
      const href = window.URL.createObjectURL(response.data);
      const link = document.createElement("a");
      link.href = href;
      link.setAttribute("download", `reconciled_${startStr}_${endStr}.csv`);
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      setExportVisible(false);
    } catch (e) {
      console.error("Export error", e);
      toast.current?.show({ severity: "error", summary: "Error", detail: "Export failed" });
    }
  };

  const handleFinalize = async () => {
    if (!selectedDbEntries.length) return;
    setFinalizeLoading(true);
    try {
      const ids = selectedDbEntries.map((e: any) => e.entryId).filter(Boolean);
      // Assuming an endpoint for finalizing/updating status exists
      await api.put("/bank-entries/finalize", { ids });
      toast.current?.show({
        severity: "success",
        summary: "Finalized",
        detail: `${ids.length} entries finalized`,
      });
      setSelectedDbEntries([]);
      refreshDatabase(dbOffset, rowsPerPage);
    } catch (e) {
      console.error(e);
      toast.current?.show({ severity: "error", summary: "Error", detail: "Failed to finalize" });
    } finally {
      setFinalizeLoading(false);
    }
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
                onChange={(e) => {
                    setBank(e.value);
                    if (dataSource === "db") {
                        refreshDatabase(0, rowsPerPage, { bankCode: e.value });
                    }
                }}
                options={bankList}
                optionLabel="name"
                optionValue="code"
                placeholder="Select Bank"
                className="w-full md:w-14rem"
              />
              <input
                type="file"
                accept=".csv,text/csv"
                onChange={handleFileChange}
                disabled={!bank}
              />
              {bank && (
                  <Button label="Add Entry" icon="pi pi-plus" onClick={() => setManualVisible(true)} className="p-button-success" />
              )}
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
              <Button
                label="Export Reconciled"
                icon="pi pi-download"
                severity="help"
                onClick={() => setExportVisible(true)}
              />
            </div>
          </div>
          <div className="grid">
            <div className="col-12 md:col-4">
              <div className="flex justify-content-between mb-3">
                <div>
                  <span className="block text-500 font-medium mb-2">
                    Current View Count
                  </span>
                  <div className="text-900 font-medium text-xl">{dataSource === 'csv' ? filteredTransactions.length : dbEntries.length}</div>
                </div>
                <div
                  className="flex align-items-center justify-content-center bg-blue-100 border-round"
                  style={{ width: "2.5rem", height: "2.5rem" }}
                >
                  <i className="pi pi-list text-blue-500 text-xl" />
                </div>
              </div>
              <span className="text-500">Entries in current view</span>
            </div>
            <div className="col-12 md:col-4">
              <div className="flex justify-content-between mb-3">
                <div>
                  <span className="block text-500 font-medium mb-2">
                    Net Change (View)
                  </span>
                  <div className="text-900 font-medium text-xl">
                    {formatCurrency(
                        dataSource === 'csv' 
                            ? totalBalance 
                            : dbEntries.reduce((acc, e) => acc + (e.amountType === 'CR' ? e.amount : -e.amount), 0)
                    )}
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
                    Totals (View)
                  </span>
                  <div className="text-900 font-medium text-sm">
                    In:{" "}
                    <span className="text-green-600">
                      {formatCurrency(
                          dataSource === 'csv' 
                              ? totalIn 
                              : dbEntries.filter(e => e.amountType === 'CR').reduce((acc, e) => acc + e.amount, 0)
                      )}
                    </span>{" "}
                    Out:{" "}
                    <span className="text-pink-600">
                      {formatCurrency(
                        dataSource === 'csv' 
                            ? totalOut 
                            : dbEntries.filter(e => e.amountType === 'DB').reduce((acc, e) => acc + e.amount, 0)
                      )}
                    </span>
                  </div>
                </div>
                <div
                  className="flex align-items-center justify-content-center bg-purple-100 border-round"
                  style={{ width: "2.5rem", height: "2.5rem" }}
                >
                  <i className="pi pi-comment text-purple-500 text-xl" />
                </div>
              </div>
              <span className="text-500">Inflows and outflows</span>
            </div>
          </div>
        </div>
      </div>

      <Dialog
        header="Export Reconciled Transactions"
        visible={exportVisible}
        style={{ width: "30vw" }}
        onHide={() => setExportVisible(false)}
      >
          <div className="flex flex-column gap-2">
            <label htmlFor="export-dates">Select Date Range</label>
            <Calendar
                id="export-dates"
                value={exportRange}
                onChange={(e) => setExportRange(e.value as any)}
                selectionMode="range"
                readOnlyInput
                showIcon
            />
            <div className="flex justify-content-end mt-4">
                <Button label="Cancel" icon="pi pi-times" onClick={() => setExportVisible(false)} className="p-button-text" />
                <Button label="Export" icon="pi pi-check" onClick={handleExport} autoFocus />
            </div>
          </div>
      </Dialog>

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
          {dataSource === "db" && (
            <div className="flex justify-content-end mb-3 gap-2">
              <Button
                label="View Finalized"
                icon="pi pi-list"
                severity="info"
                onClick={() => router.push("/banking/finalized")}
              />
              <Button
                label="Finalize Selected"
                icon="pi pi-check-square"
                severity="success"
                onClick={handleFinalize}
                disabled={!selectedDbEntries.length || finalizeLoading}
                loading={finalizeLoading}
              />
            </div>
          )}
          <DataTable
            value={tableData}
            selection={selectedDbEntries}
            onSelectionChange={(e) => setSelectedDbEntries(e.value)}
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
            {dataSource === "db" && (
              <Column selectionMode="multiple" headerStyle={{ width: "3rem" }} />
            )}
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
                } else if (typeof row.delta === "number") {
                  val = row.delta;
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
              style={{ width: "18%" }}
              body={(row: any) => (
                <div className="flex gap-2">
                  <Button
                    label="View"
                    icon="pi pi-eye"
                    className="p-button-text p-button-sm"
                    onClick={() => openViewAttached(row)}
                    disabled={dataSource !== "db" || !row.entryId}
                  />
                  <Button
                    label="Reconcile"
                    icon="pi pi-link"
                    className="p-button-sm"
                    onClick={() => openReconcile(row)}
                    disabled={dataSource !== "db" || !row.entryId}
                  />
                </div>
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
      <Dialog
        visible={viewAttachedVisible}
        onHide={() => setViewAttachedVisible(false)}
        header={
          viewAttachedEntry
            ? `Attached Invoices – ${viewAttachedEntry.description}`
            : "Attached Invoices"
        }
        style={{ width: "50vw" }}
        modal
      >
        <DataTable value={viewAttachedInvoices} responsiveLayout="scroll" dataKey="id">
          <Column field="invoiceNo" header="Invoice No" />
          <Column field="invoiceDate" header="Date" />
          <Column field="customerName" header="Customer" />
          <Column
            field="totalAmount"
            header="Total"
            body={(row: AttachedInvoice) => formatCurrency(row.totalAmount)}
          />
          <Column
            field="matchedAmount"
            header="Allocated"
            body={(row: AttachedInvoice) => formatCurrency(row.matchedAmount)}
          />
          <Column field="status" header="Status" />
        </DataTable>
      </Dialog>
      <Toast ref={toast} />
      <Dialog header="Manual Entry" visible={manualVisible} onHide={() => setManualVisible(false)} style={{ width: '400px' }}>
          <div className="flex flex-column gap-3">
              <div className="flex flex-column gap-2">
                  <label>Date</label>
                  <Calendar value={manualData.date} onChange={(e) => setManualData({...manualData, date: e.value as Date})} showIcon />
              </div>
              <div className="flex flex-column gap-2">
                  <label>Description</label>
                  <input className="p-inputtext p-component" value={manualData.description} onChange={(e) => setManualData({...manualData, description: e.target.value})} />
              </div>
              <div className="flex flex-column gap-2">
                  <label>Amount</label>
                  <InputNumber value={manualData.amount} onValueChange={(e) => setManualData({...manualData, amount: e.value || 0})} mode="currency" currency="IDR" />
              </div>
              <div className="flex flex-column gap-2">
                  <label>Type</label>
                  <SelectButton value={manualData.type} options={[{label: "Money In", value: "CR"}, {label: "Money Out", value: "DB"}]} onChange={(e) => setManualData({...manualData, type: e.value})} />
              </div>
              <Button label="Save" onClick={saveManualEntry} />
          </div>
      </Dialog>
    </div>
  );
};

export default BankingPage;

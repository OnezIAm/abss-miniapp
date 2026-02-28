"use client";
import React, { useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { DataTable } from "primereact/datatable";
import { Column } from "primereact/column";
import { Button } from "primereact/button";
import { Tag } from "primereact/tag";
import { Dropdown } from "primereact/dropdown";
import { Calendar } from "primereact/calendar";
import { Toast } from "primereact/toast";
import { Dialog } from "primereact/dialog";
import { api } from "@/app/lib/api";
import { BankTypeService } from "@/demo/service/BankTypeService";

type BankType = {
  code: string;
  name: string;
  description?: string;
};

type FinalizedEntry = {
  id: string;
  transactionDate: string;
  description: string;
  branch?: string;
  amount: number;
  amountType: "CR" | "DB";
  bankCode: string;
  attachedCount?: number;
  matchedTotal?: number;
  delta?: number;
};

type AttachedInvoice = {
  id: string;
  invoiceNo?: string;
  invoiceDate?: string;
  customerName?: string;
  totalAmount?: number;
  paidAmount?: number;
  matchedAmount?: number;
};

const FinalizedReconciledPage = () => {
  const router = useRouter();
  const toast = useRef<Toast>(null);

  const [bankList, setBankList] = useState<BankType[]>([]);
  const [bank, setBank] = useState<string>("");
  const [entries, setEntries] = useState<FinalizedEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [dateFilter, setDateFilter] = useState<Date | null>(new Date());

  const [viewVisible, setViewVisible] = useState(false);
  const [viewEntry, setViewEntry] = useState<FinalizedEntry | null>(null);
  const [viewInvoices, setViewInvoices] = useState<AttachedInvoice[]>([]);
  const [viewLoading, setViewLoading] = useState(false);

  const [exportVisible, setExportVisible] = useState(false);
  const [exportRange, setExportRange] = useState<(Date | null)[] | null>(null);

  React.useEffect(() => {
    BankTypeService.getBankTypes()
      .then((data) => {
        const list = Array.isArray(data) ? data : Array.isArray(data?.data) ? data.data : [];
        setBankList(list);
        if (!bank && list.length > 0) setBank(list[0].code);
      })
      .catch((err) => {
        console.error("Failed to fetch banks", err);
        toast.current?.show({
          severity: "error",
          summary: "Failed to load banks",
          detail: String(err?.message || err),
        });
      });
  }, []);

  const normalizeEntries = (respData: any): FinalizedEntry[] => {
    const list = Array.isArray(respData)
      ? respData
      : Array.isArray(respData?.data)
      ? respData.data
      : Array.isArray(respData?.items)
      ? respData.items
      : [];
    return list;
  };

  const formatCurrency = (value: number) =>
    value.toLocaleString("en-US", { style: "currency", currency: "IDR" });

  const formatDisplayDate = (s: string) => {
    if (!s) return "-";
    const d = new Date(s);
    if (isNaN(d.getTime())) return s;
    return d.toLocaleDateString("en-GB");
  };

  const refresh = async () => {
    if (!bank) return;
    setLoading(true);
    try {
      const params: any = { bankCode: bank, showFinalized: "true", reconciledOnly: "true" };
      if (dateFilter) {
        // Backend expects startDate/endDate. 
        // If we select a single date, we pass it as both start and end to filter for that day.
        const year = dateFilter.getFullYear();
        const month = String(dateFilter.getMonth() + 1).padStart(2, '0');
        const day = String(dateFilter.getDate()).padStart(2, '0');
        const dateStr = `${year}-${month}-${day}`;
        params.startDate = dateStr;
        params.endDate = dateStr;
      }
      const res = await api.get("/bank-entries", { params });
      setEntries(normalizeEntries(res.data));
    } catch (err: any) {
      console.error(err);
      toast.current?.show({
        severity: "error",
        summary: "Failed to load finalized reconciled entries",
        detail: String(err?.response?.data || err?.message || err),
      });
    } finally {
      setLoading(false);
    }
  };

  React.useEffect(() => {
    refresh();
  }, [bank, dateFilter]);

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
      // We want to export reconciled entries that are finalized
      const url = `/bank-entries/export/reconciled?startDate=${startStr}&endDate=${endStr}&showFinalized=true`;
      
      const response = await api.get(url, { responseType: "blob" });
      const href = window.URL.createObjectURL(response.data);
      const link = document.createElement("a");
      link.href = href;
      link.setAttribute("download", `finalized_reconciled_${startStr}_${endStr}.csv`);
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      setExportVisible(false);
    } catch (e) {
      console.error("Export error", e);
      toast.current?.show({ severity: "error", summary: "Error", detail: "Export failed" });
    }
  };

  const openView = async (entry: FinalizedEntry) => {
    setViewEntry(entry);
    setViewVisible(true);
    setViewLoading(true);
    setViewInvoices([]);
    try {
      const res = await api.get(`/bank-entries/${entry.id}/invoices`);
      const list = Array.isArray(res.data) ? res.data : Array.isArray(res.data?.data) ? res.data.data : [];
      setViewInvoices(list);
    } catch (err: any) {
      console.error(err);
      toast.current?.show({
        severity: "error",
        summary: "Failed to load attached invoices",
        detail: String(err?.response?.data || err?.message || err),
      });
    } finally {
      setViewLoading(false);
    }
  };

  const rows = useMemo(() => {
    return entries.map((e) => {
      const signedAmount = e.amountType === "CR" ? e.amount : -e.amount;
      const matched = Number(e.matchedTotal || 0);
      const delta =
        typeof e.delta === "number" ? e.delta : Math.abs(Number(e.amount || 0)) - matched;
      return {
        ...e,
        signedAmount,
        matched,
        delta,
      };
    });
  }, [entries]);

  return (
    <div className="grid">
      <div className="col-12">
        <div className="card">
          <Toast ref={toast} />

          <div className="flex align-items-center justify-content-between gap-3 flex-wrap mb-3">
            <div className="flex align-items-center gap-2 flex-wrap">
              <Button
                label="Back"
                icon="pi pi-arrow-left"
                className="p-button-text"
                onClick={() => router.push("/banking")}
              />
              <h5 className="m-0">Finalized Reconciled</h5>
            </div>

            <div className="flex align-items-center gap-2 flex-wrap">
              <Calendar
                value={dateFilter}
                onChange={(e) => setDateFilter(e.value as Date)}
                dateFormat="dd/mm/yy"
                showIcon
                placeholder="Select Date"
              />
              <Dropdown
                value={bank}
                options={bankList}
                optionLabel="name"
                optionValue="code"
                placeholder="Select bank"
                onChange={(e) => setBank(e.value)}
                style={{ minWidth: 260 }}
              />
              <Button
                label="Refresh"
                icon="pi pi-refresh"
                onClick={refresh}
                loading={loading}
              />
              <Button
                label="Export"
                icon="pi pi-download"
                severity="success"
                onClick={() => setExportVisible(true)}
              />
            </div>
          </div>

          <DataTable value={rows} loading={loading} paginator rows={20} rowsPerPageOptions={[20, 50, 100]}>
            <Column field="transactionDate" header="Date" body={(r) => <span>{formatDisplayDate(r.transactionDate)}</span>} />
            <Column field="description" header="Description" />
            <Column
              field="signedAmount"
              header="Amount"
              body={(r) => <span>{formatCurrency(Number(r.signedAmount || 0))}</span>}
            />
            <Column
              field="matched"
              header="Matched"
              body={(r) => <span>{formatCurrency(Number(r.matched || 0))}</span>}
            />
            <Column
              header="Delta"
              body={(r) => {
                const val = Number(r.delta || 0);
                const ok = Math.abs(val) < 0.0001;
                return <Tag value={ok ? "0" : formatCurrency(val)} severity={ok ? "success" : "warning"} />;
              }}
            />
            <Column
              field="attachedCount"
              header="Invoices"
              body={(r) => <span>{Number(r.attachedCount || 0)}</span>}
            />
            <Column
              header="Actions"
              body={(r) => (
                <Button
                  label="View"
                  icon="pi pi-eye"
                  className="p-button-text p-button-sm"
                  onClick={() => openView(r)}
                />
              )}
            />
          </DataTable>

          <Dialog
            visible={viewVisible}
            onHide={() => setViewVisible(false)}
            header={`Attached Invoices${viewEntry ? ` · ${formatCurrency(viewEntry.amountType === "CR" ? viewEntry.amount : -viewEntry.amount)}` : ""}`}
            style={{ width: "60vw" }}
            modal
          >
            <DataTable value={viewInvoices} loading={viewLoading} paginator rows={10} rowsPerPageOptions={[10, 20, 50]}>
              <Column field="invoiceNo" header="Invoice No" />
              <Column field="invoiceDate" header="Date" body={(r) => <span>{formatDisplayDate(r.invoiceDate || "")}</span>} />
              <Column field="customerName" header="Customer" />
              <Column
                field="totalAmount"
                header="Total"
                body={(r) => <span>{formatCurrency(Number(r.totalAmount || 0))}</span>}
              />
              <Column
                field="paidAmount"
                header="Paid"
                body={(r) => <span>{formatCurrency(Number(r.paidAmount || 0))}</span>}
              />
              <Column
                field="matchedAmount"
                header="Matched"
                body={(r) => <span>{formatCurrency(Number(r.matchedAmount || 0))}</span>}
              />
            </DataTable>
          </Dialog>

          <Dialog
            header="Export Finalized Transactions"
            visible={exportVisible}
            style={{ width: '450px' }}
            modal
            onHide={() => setExportVisible(false)}
          >
             <div className="flex flex-column gap-2">
                <label htmlFor="range">Date Range</label>
                <Calendar
                    id="range"
                    value={exportRange}
                    onChange={(e) => setExportRange(e.value as any)}
                    selectionMode="range"
                    readOnlyInput
                    showIcon
                    dateFormat="dd/mm/yy"
                    className="w-full"
                />
            </div>
            <div className="flex justify-content-end gap-2 mt-3">
                <Button label="Cancel" icon="pi pi-times" onClick={() => setExportVisible(false)} className="p-button-text" />
                <Button label="Export" icon="pi pi-check" onClick={handleExport} autoFocus />
            </div>
          </Dialog>
        </div>
      </div>
    </div>
  );
};

export default FinalizedReconciledPage;


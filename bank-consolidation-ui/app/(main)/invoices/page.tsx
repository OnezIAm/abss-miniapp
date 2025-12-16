"use client";
import React, { useEffect, useMemo, useRef, useState } from "react";
import { DataTable } from "primereact/datatable";
import { Column } from "primereact/column";
import { Button } from "primereact/button";
import { Toast } from "primereact/toast";
import { Tag } from "primereact/tag";
import { api } from "@/app/lib/api";

type Invoice = Record<string, any>;

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
    limit: typeof p.limit === "number" ? p.limit : 10,
    offset: typeof p.offset === "number" ? p.offset : 0,
  };
};

const formatCurrency = (value: number, currency = "IDR") =>
  Number(value).toLocaleString("en-US", { style: "currency", currency });

const toDisplayDate = (s: string) => {
  if (!s) return "";
  const d = new Date(s);
  return isNaN(d.getTime()) ? s : d.toLocaleDateString();
};

const InvoicesPage = () => {
  const toast = useRef<Toast | null>(null);
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [rows, setRows] = useState<number>(10);
  const [first, setFirst] = useState<number>(0);
  const [loading, setLoading] = useState<boolean>(false);

  const keys = useMemo(() => {
    const k = new Set<string>();
    for (const item of invoices) {
      Object.keys(item || {}).forEach((x) => k.add(x));
    }
    return Array.from(k);
  }, [invoices]);

  const refresh = async (offset = 0, limit = rows) => {
    try {
      setLoading(true);
      const { data } = await api.get("/invoices", {
        params: { limit, offset },
      });
      const norm = normalizeResponse(data);
      setInvoices(norm.entries);
      setTotal(norm.total);
      setRows(norm.limit);
      setFirst(norm.offset);
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
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh(0, rows);
  }, []);

  const renderCell = (row: any, field: string) => {
    const v = row?.[field];
    if (v === null || v === undefined) return "-";
    if (typeof v === "number" && /amount|total|subtotal|grand/i.test(field)) {
      return <span>{formatCurrency(v)}</span>;
    }
    if (typeof v === "string" && /date/i.test(field)) {
      return <span>{toDisplayDate(v)}</span>;
    }
    if (typeof v === "string" && /status/i.test(field)) {
      const sev =
        /paid|complete/i.test(v)
          ? "success"
          : /cancel|void/i.test(v)
          ? "danger"
          : /due|pending/i.test(v)
          ? "warn"
          : "info";
      return <Tag value={v} severity={sev as any} />;
    }
    return <span>{String(v)}</span>;
  };

  return (
    <div className="grid">
      <div className="col-12">
        <div className="card">
          <div className="flex justify-content-between align-items-center mb-3">
            <h5>Invoices</h5>
            <div className="flex gap-2 align-items-center">
              <Button
                label="Refresh"
                icon="pi pi-sync"
                onClick={() => refresh(first, rows)}
                loading={loading}
              />
            </div>
          </div>
          <DataTable
            value={invoices}
            paginator
            rows={rows}
            rowsPerPageOptions={[10, 20, 50]}
            lazy
            totalRecords={total}
            first={first}
            onPage={(e) => {
              setRows(e.rows);
              setFirst(e.first);
              refresh(e.first, e.rows);
            }}
            loading={loading}
            responsiveLayout="scroll"
            dataKey="id"
          >
            {keys.map((k) => (
              <Column
                key={k}
                field={k}
                header={k}
                body={(row) => renderCell(row, k)}
              />
            ))}
          </DataTable>
        </div>
      </div>
      <Toast ref={toast} />
    </div>
  );
};

export default InvoicesPage;

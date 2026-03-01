"use client";
import React, { useEffect, useMemo, useRef, useState } from "react";
import { DataTable, DataTableExpandedRows } from "primereact/datatable";
import { Column } from "primereact/column";
import { Button } from "primereact/button";
import { Toast } from "primereact/toast";
import { Tag } from "primereact/tag";
import { Toolbar } from "primereact/toolbar";
import { api } from "@/app/lib/api";

type Invoice = Record<string, any>;

const normalizeResponse = (respData: any) => {
  const entries = Array.isArray(respData)
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

const flattenInvoice = (inv: Invoice): Invoice => {
  const result: Invoice = {};
  const recurse = (cur: any, prop: string) => {
    if (Object(cur) !== cur) {
      result[prop] = cur;
    } else if (Array.isArray(cur)) {
      // Skip arrays or stringify if needed. For now, skip to keep table clean.
      // result[prop] = JSON.stringify(cur);
    } else {
      let isEmpty = true;
      for (const p in cur) {
        isEmpty = false;
        recurse(cur[p], prop ? `${prop}.${p}` : p);
      }
      if (isEmpty && prop) {
        result[prop] = {};
      }
    }
  };
  recurse(inv, "");
  // Ensure id is preserved at top level if it existed in some nested form or was assigned
  if (inv.id) result.id = inv.id;
  return result;
};

const InvoicesPage = () => {
  const toast = useRef<Toast | null>(null);
  const [dbInvoices, setDbInvoices] = useState<Invoice[]>([]);
  const [localInvoices, setLocalInvoices] = useState<Invoice[]>([]);
  const [dbTotal, setDbTotal] = useState<number>(0);
  const [dbRows, setDbRows] = useState<number>(10);
  const [dbFirst, setDbFirst] = useState<number>(0);
  const [localFirst, setLocalFirst] = useState<number>(0);
  const [localRows, setLocalRows] = useState<number>(10);
  const [loading, setLoading] = useState<boolean>(false);
  const [dataSource, setDataSource] = useState<"db" | "local">("db");

  const [expandedRows, setExpandedRows] = useState<
    any[] | DataTableExpandedRows | undefined
  >(undefined);

  const activeInvoices = useMemo(() => {
    if (dataSource === "db") return dbInvoices;
    return localInvoices;
  }, [dataSource, dbInvoices, localInvoices]);

  const keys = useMemo(() => {
    if (activeInvoices.length === 0) return [];

    const k = new Set<string>();
    const sample = activeInvoices.slice(0, 10);

    for (const item of sample) {
      Object.keys(item || {}).forEach((key) => {
        // Exclude detail arrays/objects from main columns to keep it clean,
        // but ensure we have at least some columns.
        // actually, let's just show everything that isn't the detail array
        if (key !== "detail" && key !== "details" && key !== "items") {
          k.add(key);
        }
      });
    }

    // If still empty (e.g. data only has detail?), force add some known keys if present
    if (k.size === 0 && sample.length > 0) {
      return Object.keys(sample[0]);
    }

    return Array.from(k);
  }, [activeInvoices]);

  const refresh = async (offset = 0, limit = dbRows) => {
    try {
      setLoading(true);
      const { data } = await api.get("/invoices", {
        params: { limit, offset },
      });
      const norm = normalizeResponse(data);
      setDbInvoices(norm.entries);
      setDbTotal(norm.total);
      setDbRows(norm.limit);
      setDbFirst(norm.offset);
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
    // Initial load for DB
    refresh(0, dbRows);
  }, []);

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      // Reset input value to allow re-uploading the same file
      event.target.value = "";

      const reader = new FileReader();
      reader.onload = (e) => {
        try {
          const content = e.target?.result as string;
          const parsed = JSON.parse(content);

          let dataToLoad: Invoice[] = [];
          if (Array.isArray(parsed)) {
            dataToLoad = parsed;
          } else if (typeof parsed === "object" && parsed !== null) {
            // Handle wrapper objects like { data: [...] } or { items: [...] }
            if (Array.isArray(parsed.data)) dataToLoad = parsed.data;
            else if (Array.isArray(parsed.items)) dataToLoad = parsed.items;
            else if (Array.isArray(parsed.invoices))
              dataToLoad = parsed.invoices;
          }

          if (dataToLoad.length > 0) {
            // Ensure each item has an id for UI keying
            const enriched = dataToLoad.map((item, index) => ({
              ...item,
              id: item.id ?? `local-${index}`,
            }));
            setLocalInvoices(enriched);
            setDataSource("local");
            setLocalFirst(0); // Reset pagination for new file
            toast.current?.show({
              severity: "success",
              summary: "Loaded",
              detail: `Loaded ${enriched.length} invoices`,
            });
          } else {
            toast.current?.show({
              severity: "error",
              summary: "Invalid Format",
              detail: "File must contain a JSON array of invoices",
            });
          }
        } catch (error) {
          console.error("JSON Parse error:", error);
          toast.current?.show({
            severity: "error",
            summary: "Parse Error",
            detail: "Invalid JSON file",
          });
        }
      };
      reader.readAsText(file);
    }
  };

  const saveToDb = async () => {
    try {
      setLoading(true);

      // Transform payload: map 'detail' to 'details' as expected by backend
      const payload = localInvoices.map((inv, idx) => {
        const { detail, details, id, invoiceHeaderId, ...rest } = inv;
        const invoiceDetails = detail || details || [];

        // Use existing ID or fallback to the generated one
        const headerId = invoiceHeaderId || id || `local-${Date.now()}-${idx}`;

        const mappedDetails = Array.isArray(invoiceDetails)
          ? invoiceDetails.map((d: any, dIdx: number) => ({
              ...d,
              invoiceDetailId:
                d.invoiceDetailId || d.id || `${headerId}-d-${dIdx}`,
              invoiceHeaderId: headerId,
              productId: d.productId || d.product_id || "UNKNOWN",
              // Ensure numbers
              qty: Number(d.qty || 0),
              unitPrice: Number(d.unitPrice || 0),
              amount: Number(d.amount || 0),
              ppn: Number(d.ppn || 0),
              ppnPercent: Number(d.ppnPercent || 0),
            }))
          : [];

        return {
          ...rest,
          invoiceHeaderId: headerId,
          detail: mappedDetails,
          // Ensure required fields
          invoiceNo: inv.invoiceNo || `INV-${Date.now()}-${idx}`,
          invoiceDate:
            inv.invoiceDate || new Date().toISOString().split("T")[0],
          customerId: inv.customerId || "CUST-GENERIC",
          customerName: inv.customerName || "Generic Customer",
          companyCode: inv.companyCode || "DEFAULT",
          totalAmount: Number(inv.totalAmount || 0),
          totalTax: Number(inv.totalTax || 0),
        };
      });

      await api.post("/invoices/bulk", payload);

      toast.current?.show({
        severity: "success",
        summary: "Saved",
        detail: "Invoices saved to DB",
      });
      // Refresh DB data and switch view
      await refresh(0, dbRows);
      setDataSource("db");
      setLocalInvoices([]);
      setLocalFirst(0);
    } catch (e: any) {
      console.error(e);
      const msg = e.response?.data || e.message || "Failed to save data";
      toast.current?.show({
        severity: "error",
        summary: "Save Failed",
        detail: typeof msg === "string" ? msg : "Validation error",
      });
    } finally {
      setLoading(false);
    }
  };

  const renderCell = (row: any, field: string) => {
    const v = row?.[field];
    if (v === null || v === undefined) return "-";

    // Handle objects/arrays in main columns by stringifying or showing type
    if (typeof v === "object") {
      if (Array.isArray(v))
        return <Tag value={`Array[${v.length}]`} severity="info" />;
      return <Tag value="Object" severity="warning" />;
    }

    if (typeof v === "number" && /amount|total|subtotal|grand/i.test(field)) {
      return <span>{formatCurrency(v)}</span>;
    }
    if (typeof v === "string" && /date/i.test(field)) {
      return <span>{toDisplayDate(v)}</span>;
    }
    if (typeof v === "string" && /status/i.test(field)) {
      const sev = /paid|complete/i.test(v)
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

  const InvoiceDetail = ({ invoice }: { invoice: any }) => {
    const [details, setDetails] = useState<any[]>(
      invoice.details || invoice.detail || invoice.items || [],
    );
    const [loading, setLoading] = useState(false);

    useEffect(() => {
      if (
        details.length > 0 ||
        !invoice.id ||
        String(invoice.id).startsWith("local-")
      )
        return;

      setLoading(true);
      api
        .get(`/invoices/${invoice.id}`)
        .then((res) => {
          if (res.data && Array.isArray(res.data.details)) {
            setDetails(res.data.details);
          }
        })
        .catch((err) => console.error("Failed to fetch invoice details", err))
        .finally(() => setLoading(false));
    }, [invoice.id]);

    if (loading) return <div className="p-3">Loading details...</div>;

    if (!details || details.length === 0)
      return <div className="p-3">No details available</div>;

    return (
      <div className="p-3">
        <h5>Details for {invoice.invoiceNo || invoice.id}</h5>
        <DataTable value={details} size="small" showGridlines>
          <Column field="productName" header="Product" />
          <Column field="qty" header="Qty" />
          <Column
            field="unitPrice"
            header="Price"
            body={(d: any) => formatCurrency(d.unitPrice)}
          />
          <Column
            field="amount"
            header="Amount"
            body={(d: any) => formatCurrency(d.amount)}
          />
          <Column
            field="ppn"
            header="Tax"
            body={(d: any) => formatCurrency(d.ppn)}
          />
        </DataTable>
      </div>
    );
  };

  const rowExpansionTemplate = (data: Invoice) => {
    return <InvoiceDetail invoice={data} />;
  };

  return (
    <div className="grid">
      <div className="col-12">
        <div className="card">
          <Toolbar
            className="mb-4"
            left={
              <div className="flex gap-2 align-items-center">
                <Button
                  label="DB Data"
                  icon="pi pi-database"
                  className={
                    dataSource === "db"
                      ? "p-button-primary"
                      : "p-button-outlined"
                  }
                  onClick={() => setDataSource("db")}
                />
                <div className="flex align-items-center">
                  <label
                    htmlFor="json-upload"
                    className="p-button p-button-outlined cursor-pointer"
                  >
                    <i className="pi pi-upload mr-2"></i>
                    Import JSON
                  </label>
                  <input
                    id="json-upload"
                    type="file"
                    accept="application/json"
                    onChange={handleFileChange}
                    style={{ display: "none" }}
                  />
                </div>
                {dataSource === "local" && (
                  <Button
                    label="Clear"
                    icon="pi pi-trash"
                    className="p-button-outlined p-button-secondary"
                    onClick={() => {
                      setLocalInvoices([]);
                      setDataSource("db");
                    }}
                  />
                )}
              </div>
            }
            right={
              dataSource === "local" && (
                <Button
                  label="Save to DB"
                  icon="pi pi-cloud-upload"
                  severity="success"
                  onClick={saveToDb}
                  loading={loading}
                  disabled={localInvoices.length === 0}
                />
              )
            }
          />
          <div className="flex justify-content-between align-items-center mb-3">
            <h5>
              {dataSource === "local" ? "Preview Import Data" : "Invoices"}
            </h5>
            <div className="flex gap-2 align-items-center">
              <Button
                label="Refresh"
                icon="pi pi-sync"
                onClick={() => refresh(dbFirst, dbRows)}
                loading={loading}
                disabled={dataSource === "local"}
              />
            </div>
          </div>
          <DataTable
            value={activeInvoices}
            paginator
            rows={dataSource === "db" ? dbRows : localRows}
            rowsPerPageOptions={[10, 20, 50]}
            lazy={dataSource === "db"}
            totalRecords={dataSource === "db" ? dbTotal : localInvoices.length}
            first={dataSource === "db" ? dbFirst : localFirst}
            onPage={(e) => {
              if (dataSource === "db") {
                setDbRows(e.rows);
                setDbFirst(e.first);
                refresh(e.first, e.rows);
              } else {
                setLocalRows(e.rows);
                setLocalFirst(e.first);
              }
            }}
            loading={loading}
            responsiveLayout="scroll"
            dataKey="id"
            expandedRows={expandedRows}
            onRowToggle={(e) => setExpandedRows(e.data)}
            rowExpansionTemplate={rowExpansionTemplate}
          >
            <Column expander style={{ width: "3em" }} />
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

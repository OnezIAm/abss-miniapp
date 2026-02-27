import React, { useEffect, useState } from 'react';
import api from '../api';

const InvoiceList = () => {
  const [invoices, setInvoices] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetchInvoices();
  }, []);

  const fetchInvoices = async () => {
    try {
      setLoading(true);
      const response = await api.get('/reports/invoices');
      setInvoices(response.data || []);
      setError(null);
    } catch (err) {
      setError('Failed to fetch invoices');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const formatCurrency = (amount) => {
    return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(amount);
  };

  if (loading) return <div>Loading invoices...</div>;
  if (error) return <div className="error">{error}</div>;

  return (
    <div className="invoice-list">
      <h2>Invoice List</h2>
      <table>
        <thead>
          <tr>
            <th>Invoice No</th>
            <th>Date</th>
            <th>Customer</th>
            <th>Total Amount</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {invoices.length === 0 ? (
            <tr><td colspan="5">No invoices found</td></tr>
          ) : (
            invoices.map((inv) => (
              <tr key={inv.headerId}>
                <td>{inv.invoiceNo}</td>
                <td>{new Date(inv.invoiceDate).toLocaleDateString()}</td>
                <td>{inv.customerName}</td>
                <td>{formatCurrency(inv.totalAmount)}</td>
                <td>
                  <span className={`status-badge ${inv.status.toLowerCase()}`}>
                    {inv.status}
                  </span>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
};

export default InvoiceList;

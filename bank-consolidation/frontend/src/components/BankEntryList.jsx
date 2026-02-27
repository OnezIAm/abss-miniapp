import React, { useEffect, useState } from 'react';
import api from '../api';

const BankEntryList = () => {
  const [entries, setEntries] = useState([]);
  const [loading, setLoading] = useState(false);
  const [bankCode, setBankCode] = useState('BCA');
  const [uploadBankCode, setUploadBankCode] = useState('BCA');
  const [file, setFile] = useState(null);
  const [uploadStatus, setUploadStatus] = useState('');

  useEffect(() => {
    fetchEntries();
  }, [bankCode]);

  const fetchEntries = async () => {
    try {
      setLoading(true);
      const response = await api.get(`/bank-entries?bankCode=${bankCode}`);
      setEntries(response.data || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleFileChange = (e) => {
    setFile(e.target.files[0]);
  };

  const handleUpload = async () => {
    if (!file) {
      alert('Please select a file');
      return;
    }

    const formData = new FormData();
    formData.append('file', file);
    formData.append('bankCode', uploadBankCode);

    try {
      setUploadStatus('Uploading...');
      const response = await api.post('/bank-entries/upload', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      });
      setUploadStatus(`Success! ${response.data.count} entries saved.`);
      
      // Refresh list if we are viewing the same bank
      if (bankCode === uploadBankCode) {
        fetchEntries();
      }
      setFile(null);
      // Reset file input manually if needed, or use ref
      document.getElementById('csvFile').value = '';
    } catch (err) {
      console.error(err);
      setUploadStatus('Error: ' + (err.response?.data || err.message));
    }
  };

  const formatCurrency = (amount) => {
    return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(amount);
  };

  return (
    <div className="bank-entry-list">
      <h2>Bank Entries</h2>

      <div className="card upload-section">
        <h3>Upload CSV</h3>
        <div className="form-group">
          <label>Bank Code:</label>
          <select value={uploadBankCode} onChange={(e) => setUploadBankCode(e.target.value)}>
            <option value="BCA">BCA</option>
            <option value="MANDIRI">MANDIRI</option>
            <option value="BRI">BRI</option>
          </select>
        </div>
        <div className="form-group">
          <input type="file" id="csvFile" accept=".csv" onChange={handleFileChange} />
        </div>
        <button onClick={handleUpload} disabled={!file}>Upload & Save</button>
        {uploadStatus && <p className="status-message">{uploadStatus}</p>}
      </div>

      <div className="filter-section">
        <label>Filter Bank Code: </label>
        <select value={bankCode} onChange={(e) => setBankCode(e.target.value)}>
          <option value="BCA">BCA</option>
          <option value="MANDIRI">MANDIRI</option>
          <option value="BRI">BRI</option>
        </select>
      </div>

      {loading ? (
        <p>Loading...</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Date</th>
              <th>Description</th>
              <th>Branch</th>
              <th>Amount</th>
              <th>Type</th>
              <th>Bank Code</th>
            </tr>
          </thead>
          <tbody>
            {entries.length === 0 ? (
              <tr><td colSpan="6">No entries found</td></tr>
            ) : (
              entries.map((entry) => (
                <tr key={entry.id}>
                  <td>{new Date(entry.transactionDate).toLocaleDateString()}</td>
                  <td>{entry.description}</td>
                  <td>{entry.branch}</td>
                  <td>{formatCurrency(entry.amount)}</td>
                  <td>{entry.amountType}</td>
                  <td>{entry.bankCode}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      )}
    </div>
  );
};

export default BankEntryList;

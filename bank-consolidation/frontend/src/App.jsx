import { useState } from 'react'
import './App.css'
import InvoiceList from './components/InvoiceList'
import BankEntryList from './components/BankEntryList'

function App() {
  const [activeTab, setActiveTab] = useState('invoices')

  return (
    <div className="app-container">
      <header>
        <h1>Bank Consolidation App</h1>
        <nav>
          <button 
            className={activeTab === 'invoices' ? 'active' : ''} 
            onClick={() => setActiveTab('invoices')}
          >
            Invoices
          </button>
          <button 
            className={activeTab === 'bank-entries' ? 'active' : ''} 
            onClick={() => setActiveTab('bank-entries')}
          >
            Bank Entries
          </button>
        </nav>
      </header>
      
      <main>
        {activeTab === 'invoices' ? <InvoiceList /> : <BankEntryList />}
      </main>
    </div>
  )
}

export default App

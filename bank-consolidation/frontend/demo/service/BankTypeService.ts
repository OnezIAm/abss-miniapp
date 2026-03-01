export const BankTypeService = {
    async getBankTypes() {
        // Use relative path for production (served by Go), or absolute for dev if needed.
        // For now, assuming relative path works when served from same origin.
        // If running npm run dev (port 3000) and backend on 8080, we need full URL.
        const baseUrl = typeof window !== 'undefined' && window.location.port === '3000' 
            ? 'http://localhost:8080' 
            : ''; 
            
        const res = await fetch(`${baseUrl}/api/v1/bank-types`, {
            headers: { 'Cache-Control': 'no-cache' }
        });
        if (!res.ok) throw new Error('Failed to fetch bank types');
        return res.json();
    },

    async createBankType(bankType: { name: string; code: string; description: string; format?: string }) {
        const baseUrl = typeof window !== 'undefined' && window.location.port === '3000' 
            ? 'http://localhost:8080' 
            : '';

        const res = await fetch(`${baseUrl}/api/v1/bank-types`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(bankType)
        });
        if (!res.ok) {
            const err = await res.text();
            throw new Error(err || 'Failed to create bank type');
        }
        return res.json();
    },

    async updateBankType(id: string, bankType: { name: string; code: string; description: string; format?: string }) {
        const baseUrl = typeof window !== 'undefined' && window.location.port === '3000' 
            ? 'http://localhost:8080' 
            : '';

        const res = await fetch(`${baseUrl}/api/v1/bank-types/${id}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(bankType)
        });
        if (!res.ok) {
            const err = await res.text();
            throw new Error(err || 'Failed to update bank type');
        }
        return res.json();
    }
};

'use client';

import React, { useState, useEffect, useRef } from 'react';
import { DataTable } from 'primereact/datatable';
import { Column } from 'primereact/column';
import { Button } from 'primereact/button';
import { Dialog } from 'primereact/dialog';
import { InputText } from 'primereact/inputtext';
import { InputTextarea } from 'primereact/inputtextarea';
import { Dropdown } from 'primereact/dropdown';
import { Toast } from 'primereact/toast';
import { Toolbar } from 'primereact/toolbar';
import { BankTypeService } from '@/demo/service/BankTypeService';

const BankTypesPage = () => {
    const [bankTypes, setBankTypes] = useState([]);
    const [bankTypeDialog, setBankTypeDialog] = useState(false);
    const [bankType, setBankType] = useState({ id: '', name: '', code: '', description: '', format: 'GENERIC' });
    const [submitted, setSubmitted] = useState(false);
    const toast = useRef<Toast>(null);

    const formatOptions = [
        { label: 'Generic', value: 'GENERIC' },
        { label: 'BCA', value: 'BCA' },
        { label: 'Danamon', value: 'DANAMON' }
    ];

    useEffect(() => {
        loadBankTypes();
    }, []);

    const loadBankTypes = () => {
        BankTypeService.getBankTypes().then((data) => setBankTypes(data));
    };

    const openNew = () => {
        setBankType({ id: '', name: '', code: '', description: '', format: 'GENERIC' });
        setSubmitted(false);
        setBankTypeDialog(true);
    };

    const editBankType = (bankType: any) => {
        setBankType({ ...bankType, format: bankType.format || 'GENERIC' });
        setBankTypeDialog(true);
    };

    const hideDialog = () => {
        setSubmitted(false);
        setBankTypeDialog(false);
    };

    const saveBankType = async () => {
        setSubmitted(true);

        if (bankType.name.trim() && bankType.code.trim()) {
            try {
                if (bankType.id) {
                    await BankTypeService.updateBankType(bankType.id, bankType);
                    toast.current?.show({ severity: 'success', summary: 'Successful', detail: 'Bank Type Updated', life: 3000 });
                } else {
                    await BankTypeService.createBankType(bankType);
                    toast.current?.show({ severity: 'success', summary: 'Successful', detail: 'Bank Type Created', life: 3000 });
                }
                loadBankTypes();
                setBankTypeDialog(false);
                setBankType({ id: '', name: '', code: '', description: '', format: 'GENERIC' });
            } catch (error: any) {
                toast.current?.show({ severity: 'error', summary: 'Error', detail: error.message, life: 3000 });
            }
        }
    };

    const onInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>, name: string) => {
        const val = (e.target && e.target.value) || '';
        setBankType((prev) => ({ ...prev, [name]: val }));
    };

    const onFormatChange = (e: any) => {
        setBankType((prev) => ({ ...prev, format: e.value }));
    };

    const leftToolbarTemplate = () => {
        return (
            <React.Fragment>
                <div className="my-2">
                    <Button label="New" icon="pi pi-plus" severity="success" className="mr-2" onClick={openNew} />
                </div>
            </React.Fragment>
        );
    };

    const actionBodyTemplate = (rowData: any) => {
        return (
            <React.Fragment>
                <Button icon="pi pi-pencil" rounded outlined className="mr-2" onClick={() => editBankType(rowData)} />
            </React.Fragment>
        );
    };

    const bankTypeDialogFooter = (
        <React.Fragment>
            <Button label="Cancel" icon="pi pi-times" outlined onClick={hideDialog} />
            <Button label="Save" icon="pi pi-check" onClick={saveBankType} />
        </React.Fragment>
    );

    return (
        <div className="grid crud-demo">
            <div className="col-12">
                <div className="card">
                    <Toast ref={toast} />
                    <Toolbar className="mb-4" left={leftToolbarTemplate}></Toolbar>

                    <DataTable value={bankTypes} dataKey="id" paginator rows={10} rowsPerPageOptions={[5, 10, 25]}
                        paginatorTemplate="FirstPageLink PrevPageLink PageLinks NextPageLink LastPageLink CurrentPageReport RowsPerPageDropdown"
                        currentPageReportTemplate="Showing {first} to {last} of {totalRecords} bank types"
                        header="Bank Types" emptyMessage="No bank types found.">
                        <Column field="code" header="Code" sortable style={{ minWidth: '10rem' }}></Column>
                        <Column field="name" header="Name" sortable style={{ minWidth: '12rem' }}></Column>
                        <Column field="format" header="Format" sortable style={{ minWidth: '10rem' }}></Column>
                        <Column field="description" header="Description" sortable style={{ minWidth: '16rem' }}></Column>
                        <Column body={actionBodyTemplate} exportable={false} style={{ minWidth: '8rem' }}></Column>
                    </DataTable>

                    <Dialog visible={bankTypeDialog} style={{ width: '450px' }} header="Bank Type Details" modal className="p-fluid" footer={bankTypeDialogFooter} onHide={hideDialog}>
                        <div className="field">
                            <label htmlFor="code">Code</label>
                            <InputText id="code" value={bankType.code} onChange={(e) => onInputChange(e, 'code')} required autoFocus className={submitted && !bankType.code ? 'p-invalid' : ''} />
                            {submitted && !bankType.code && <small className="p-invalid">Code is required.</small>}
                        </div>
                        <div className="field">
                            <label htmlFor="name">Name</label>
                            <InputText id="name" value={bankType.name} onChange={(e) => onInputChange(e, 'name')} required className={submitted && !bankType.name ? 'p-invalid' : ''} />
                            {submitted && !bankType.name && <small className="p-invalid">Name is required.</small>}
                        </div>
                        <div className="field">
                            <label htmlFor="format">Format</label>
                            <Dropdown id="format" value={bankType.format} options={formatOptions} onChange={onFormatChange} placeholder="Select a Format" />
                        </div>
                        <div className="field">
                            <label htmlFor="description">Description</label>
                            <InputTextarea id="description" value={bankType.description} onChange={(e) => onInputChange(e, 'description')} required rows={3} cols={20} />
                        </div>
                    </Dialog>
                </div>
            </div>
        </div>
    );
};

export default BankTypesPage;

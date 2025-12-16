package controllers

import (
    "database/sql"
    "encoding/json"
    "net/http"
)

type CategoryController struct{ DB *sql.DB }

func (c CategoryController) CreateOrList(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodPost:
        var body struct{
            ID string `json:"id"`
            Type string `json:"type"`
            Name string `json:"name"`
            DefaultAccount string `json:"defaultAccount"`
            BusinessRules any `json:"businessRules"`
            TaxRules any `json:"taxRules"`
            BudgetRef string `json:"budgetRef"`
        }
        dec := json.NewDecoder(r.Body)
        dec.DisallowUnknownFields()
        if err := dec.Decode(&body); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
        if body.ID == "" || body.Type == "" || body.Name == "" { http.Error(w, "id, type, name are required", http.StatusBadRequest); return }
        if body.Type != "money_in" && body.Type != "money_out" { http.Error(w, "type must be money_in or money_out", http.StatusBadRequest); return }
        br, _ := json.Marshal(body.BusinessRules)
        tr, _ := json.Marshal(body.TaxRules)
        _, err := c.DB.Exec(`INSERT INTO categories (id, type, name, default_account, business_rules, tax_rules, budget_ref) VALUES (?, ?, ?, ?, ?, ?, ?)`, body.ID, body.Type, body.Name, body.DefaultAccount, string(br), string(tr), body.BudgetRef)
        if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(map[string]string{"status":"ok","id":body.ID})
    case http.MethodGet:
        rows, err := c.DB.Query(`SELECT id, type, name, default_account FROM categories WHERE deleted_at IS NULL ORDER BY name`)
        if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
        defer rows.Close()
        var list []map[string]string
        for rows.Next() {
            var id, typ, name, defAcc sql.NullString
            if err := rows.Scan(&id, &typ, &name, &defAcc); err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
            m := map[string]string{"id": id.String, "type": typ.String, "name": name.String}
            if defAcc.Valid { m["defaultAccount"] = defAcc.String }
            list = append(list, m)
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(list)
    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}


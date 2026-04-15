package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type ColumnMetaData struct {
	Field   string
	Type    string
	Null    string
	Key     string
	Default sql.NullString
	Extra   string
}

type Handler struct {
	DB     *sql.DB
	Schema map[string][]ColumnMetaData
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts:=strings.Split(strings.Trim(r.URL.Path,"/"),"/")
	switch len(parts) {
	case 1:
		tableName:=parts[0]
		if tableName==""{
			if r.Method == http.MethodGet {
				h.ListTables(w,r)
			}else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			return
		}
		if _, ok := h.Schema[tableName]; !ok {
			http.Error(w, "Unknown table", http.StatusNotFound)
			return
		}
	}

}

func (h *Handler) ListTables(w http.ResponseWriter, r *http.Request) {
	tables := make([]string, 0, len(h.Schema))
	for t := range h.Schema {
		tables = append(tables, t)
	}
	response := map[string]interface{}{
		"response": map[string]interface{}{
			"tables": tables,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode data", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetTableRecords(w http.ResponseWriter, r *http.Request, tableName string) {
	// Get parameters
	offsetStr:=r.FormValue("offset")
	limitStr:=r.FormValue("limit")
	var err error
	limitInt:=0
	if limitStr !=""{
		limitInt, err= strconv.Atoi(limitStr)
		if err != nil || limitInt <0 {
			http.Error(w, "Parameter 'limit' must be a non-negative integer", http.StatusBadRequest)
			return
		}
	}
	offsetInt:=0
	if offsetStr !=""{
		offsetInt, err= strconv.Atoi(offsetStr)
		if err != nil || offsetInt < 0 {
			http.Error(w, "Parameter 'offset' must be a non-negative integer", http.StatusBadRequest)
			return
		}
	}
	row:=h.DB.QueryRow("SELECT 	* FROM ")

}

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные
func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	h := &Handler{
		DB:     db,
		Schema: make(map[string][]ColumnMetaData),
	}
	rows, err := h.DB.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		colRows, err := h.DB.Query(fmt.Sprintf("SHOW COLUMNS FROM `%s`", tableName))
		if err != nil {
			return nil, err
		}
		cols := []ColumnMetaData{}
		for colRows.Next() {
			c := ColumnMetaData{}
			if err := colRows.Scan(&c.Field, &c.Type, &c.Null, &c.Key, &c.Default, &c.Extra); err != nil {
				return nil, err
			}
			cols = append(cols, c)
		}
		colRows.Close()
		h.Schema[tableName] = cols
	}
	return h, nil
}

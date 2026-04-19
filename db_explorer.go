package main

import (
	// "context"
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

func respondWithError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": msg,
	})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	tableName := parts[0]
	switch len(parts) {
	case 1:
		if tableName == "" {
			if r.Method == http.MethodGet {
				h.ListTables(w, r)
			} else {
				respondWithError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			return
		}
		if _, ok := h.Schema[tableName]; !ok {
			respondWithError(w, "unknown table", http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.GetTableRecords(w, r, tableName)
		case http.MethodPut:
			h.CreateRecord(w, r, tableName)
		}
	case 2:
		id := parts[1]
		if _, ok := h.Schema[tableName]; !ok {
			respondWithError(w, "unknown table", http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.GetRecordById(w, r, tableName, id)
		case http.MethodPost:
			h.UpdateRecord(w, r, tableName, id)
		case http.MethodDelete:
			h.DeleteRecord(w, r, tableName, id)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	offsetStr := r.FormValue("offset")
	limitStr := r.FormValue("limit")
	var err error
	limitInt := 5
	if limitStr != "" {
		val, err := strconv.Atoi(limitStr)
		if err == nil {
			limitInt = val
		}
	}
	offsetInt := 0
	if offsetStr != "" {
		val, err := strconv.Atoi(offsetStr)
		if err == nil {
			offsetInt = val
		}
	}
	q := fmt.Sprintf("SELECT * FROM %s LIMIT ? OFFSET ?", tableName)
	rows, err := h.DB.Query(q, limitInt, offsetInt)
	if err != nil {
		respondWithError(w, "Failed to execute query", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		respondWithError(w, "Failed to get columns", http.StatusInternalServerError)
		return
	}

	records := []map[string]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valueParts := make([]interface{}, len(columns))
		for i := range values {
			valueParts[i] = &values[i]
		}
		if err := rows.Scan(valueParts...); err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			return
		}
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val == nil {
				row[col] = nil
			} else if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}

		}
		records = append(records, row)
	}
	if err = rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := map[string]interface{}{
		"response": map[string]interface{}{
			"records": records,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) GetRecordById(w http.ResponseWriter, r *http.Request, tableName string, strId string) {
	id, err := strconv.Atoi(strId)
	if err != nil {
		respondWithError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var pkColumn string
	for _, col := range h.Schema[tableName] {
		if col.Key == "PRI" {
			pkColumn = col.Field
			break
		}
	}
	if pkColumn == "" {
		http.Error(w, "Table has no primary key", http.StatusInternalServerError)
		return
	}

	query := fmt.Sprintf("SELECT * FROM `%s` WHERE `%s` = ?", tableName, pkColumn)
	rows, err := h.DB.Query(query, id)
	if err != nil {
		respondWithError(w, "Failed to execute query", http.StatusInternalServerError)
	}
	defer rows.Close()
	if !rows.Next() {
		respondWithError(w, "record not found", http.StatusNotFound)
		return
	}
	columns, err := rows.Columns()
	if err != nil {
		respondWithError(w, "Failed to get columns", http.StatusInternalServerError)
		return
	}
	values := make([]interface{}, len(columns))
	valueParts := make([]interface{}, len(columns))
	for i := range values {
		valueParts[i] = &values[i]
	}
	if err := rows.Scan(valueParts...); err != nil {
		http.Error(w, "Failed to scan row", http.StatusInternalServerError)
		return
	}
	row := make(map[string]interface{})
	for i, col := range columns {
		val := values[i]
		if val == nil {
			row[col] = nil
		} else if b, ok := val.([]byte); ok {
			row[col] = string(b)
		} else {
			row[col] = val
		}

	}
	response := map[string]interface{}{
		"response": map[string]interface{}{
			"record": row,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) CreateRecord(w http.ResponseWriter, r *http.Request, tableName string) {
	var data map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		respondWithError(w, "invalid json", http.StatusBadRequest)
		return
	}

	columns := []string{}
	placeholders := []string{}
	args := []interface{}{}
	var pkName string

	for _, col := range h.Schema[tableName] {

		if col.Key == "PRI" {
			pkName = col.Field
			continue
		}

		val, provided := data[col.Field]

		if provided {
			// 1. Валидация типов при наличии значения
			sqlType := strings.ToLower(col.Type)
			if val != nil {
				if strings.Contains(sqlType, "int") {
					if _, ok := val.(float64); !ok {
						respondWithError(w, fmt.Sprintf("field %s have invalid type", col.Field), http.StatusBadRequest)
						return
					}
				} else if strings.Contains(sqlType, "varchar") || strings.Contains(sqlType, "text") {
					if _, ok := val.(string); !ok {
						respondWithError(w, fmt.Sprintf("field %s have invalid type", col.Field), http.StatusBadRequest)
						return
					}
				}
			}
			columns = append(columns, fmt.Sprintf("`%s`", col.Field))
			placeholders = append(placeholders, "?")
			args = append(args, val)

		} else if col.Null == "NO" {
			columns = append(columns, fmt.Sprintf("`%s`", col.Field))
			placeholders = append(placeholders, "?")

			sqlType := strings.ToLower(col.Type)
			if strings.Contains(sqlType, "int") {
				args = append(args, 0)
			} else {
				args = append(args, "")
			}
		}
	}

	query := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	res, err := h.DB.Exec(query, args...)
	if err != nil {
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lastId, _ := res.LastInsertId()

	response := map[string]interface{}{
		"response": map[string]interface{}{
			pkName: lastId,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) UpdateRecord(w http.ResponseWriter, r *http.Request, tableName string, strId string) {
	id, err := strconv.Atoi(strId)
	if err != nil {
		respondWithError(w, "invalid id", http.StatusBadRequest)
		return
	}

	// Decode JSON body
	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		respondWithError(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Find primary key column
	var pkColumn string
	for _, col := range h.Schema[tableName] {
		if col.Key == "PRI" {
			pkColumn = col.Field
			break
		}
	}
	if pkColumn == "" {
		respondWithError(w, "Table has no primary key", http.StatusInternalServerError)
		return
	}

	// Build SET clause dynamically
	setClauses := []string{}
	args := []interface{}{}

	for _, col := range h.Schema[tableName] {
		// Primary key cannot be updated
		if col.Key == "PRI" {
			if _, ok := data[col.Field]; ok {
				respondWithError(w, fmt.Sprintf("field %s have invalid type", col.Field), http.StatusBadRequest)
				return
			}
			continue
		}

		val, provided := data[col.Field]

		// Not provided → skip
		if !provided {
			continue
		}

		// Handle explicit NULL
		if val == nil {
			// Only allowed if column is nullable
			if col.Null == "NO" {
				respondWithError(w, fmt.Sprintf("field %s have invalid type", col.Field), http.StatusBadRequest)
				return
			}
			setClauses = append(setClauses, fmt.Sprintf("`%s` = NULL", col.Field))
			continue
		}

		// Type validation
		sqlType := strings.ToLower(col.Type)
		if strings.Contains(sqlType, "int") {
			switch val.(type) {
			case float64, int, int64:
				// ok
			default:
				respondWithError(w, fmt.Sprintf("field %s have invalid type", col.Field), http.StatusBadRequest)
				return
			}
		} else if strings.Contains(sqlType, "varchar") || strings.Contains(sqlType, "text") || strings.Contains(sqlType, "char") {
			if _, ok := val.(string); !ok {
				respondWithError(w, fmt.Sprintf("field %s have invalid type", col.Field), http.StatusBadRequest)
				return
			}
		}
		setClauses = append(setClauses, fmt.Sprintf("`%s` = ?", col.Field))
		args = append(args, val)
	}

	if len(setClauses) == 0 {
		respondWithError(w, "no fields to update", http.StatusBadRequest)
		return
	}

	// Build and execute UPDATE
	query := fmt.Sprintf("UPDATE `%s` SET %s WHERE `%s` = ?", tableName, strings.Join(setClauses, ", "), pkColumn)
	args = append(args, id)

	result, err := h.DB.Exec(query, args...)
	if err != nil {
		respondWithError(w, "Failed to update record", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, "record not found", http.StatusNotFound)
		return
	}

	// Response format expected by tests
	response := map[string]interface{}{
		"response": map[string]interface{}{
			"updated": rowsAffected,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) DeleteRecord(w http.ResponseWriter, r *http.Request, tableName string, strId string) {
	// Validate and parse ID
	id, err := strconv.Atoi(strId)
	if err != nil {
		respondWithError(w, "invalid id", http.StatusBadRequest)
		return
	}

	// Find primary key column
	var pkColumn string
	for _, col := range h.Schema[tableName] {
		if col.Key == "PRI" {
			pkColumn = col.Field
			break
		}
	}
	if pkColumn == "" {
		respondWithError(w, "Table has no primary key", http.StatusInternalServerError)
		return
	}

	// Execute DELETE query
	query := fmt.Sprintf("DELETE FROM `%s` WHERE `%s` = ?", tableName, pkColumn)
	result, err := h.DB.Exec(query, id)
	if err != nil {
		respondWithError(w, "Failed to delete record", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()

	// Return JSON with deleted count (test expects 1 or 0, not an error for 0)
	response := map[string]interface{}{
		"response": map[string]interface{}{
			"deleted": rowsAffected,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные
func NewDbExplorer(db *sql.DB) (http.Handler, error) {

	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	var tableNames []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tableNames = append(tableNames, name)
	}
	rows.Close()

	schema := make(map[string][]ColumnMetaData)

	for _, t := range tableNames {
		colRows, err := db.Query(fmt.Sprintf("SHOW COLUMNS FROM `%s`", t))
		if err != nil {
			return nil, err
		}
		cols := []ColumnMetaData{}
		for colRows.Next() {
			c := ColumnMetaData{}
			if err := colRows.Scan(&c.Field, &c.Type, &c.Null, &c.Key, &c.Default, &c.Extra); err != nil {
				colRows.Close()
				return nil, err
			}
			cols = append(cols, c)
		}
		colRows.Close()
		schema[t] = cols
	}

	return &Handler{
		DB:     db,
		Schema: schema,
	}, nil
}

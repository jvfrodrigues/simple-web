package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/marcboeker/go-duckdb"
	_ "github.com/marcboeker/go-duckdb"
)

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("duckdb", "")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/query", queryHandler)
	http.HandleFunc("/data", dataHandler)
	http.HandleFunc("/", indexHandler)

	port, _ := os.LookupEnv("PORT")
	if port == "" {
		port = "8486"
	}
	fmt.Println("Server listening on port " + port)
	log.Fatal(http.ListenAndServe(": "+port, nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Convert rows to JSON
	results := []map[string]interface{}{}
	for rows.Next() {
		var data duckdb.Map
		if err := rows.Scan(&data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		genericMap := make(map[string]interface{})
		for k, v := range data {
			stringK, ok := k.(string)
			if !ok {
				http.Error(w, "Data error", http.StatusInternalServerError)
				return
			}
			genericMap[stringK] = v
		}
		result := map[string]interface{}{
			"data": genericMap,
		}
		results = append(results, result)
	}
	jsonBytes, err := json.Marshal(results)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonBytes)
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	table := r.URL.Query().Get("table")
	if table == "" {
		http.Error(w, "Table name is required", http.StatusBadRequest)
		return
	}

	var data map[string]string
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	createTableSQL := fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s (
            data MAP(VARCHAR, VARCHAR)
        );
    `, table)
	_, err = db.Exec(createTableSQL)
	if err != nil {
		fmt.Println("err1")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	parsedData := make([]string, 0)
	for key, value := range data {
		parsedData = append(parsedData, "('"+string(key)+"', '"+string(value)+"')")
	}

	insertSQL := fmt.Sprintf("INSERT INTO %s (data) VALUES (map_from_entries([%s]));", table, strings.Join(parsedData, ", "))
	_, err = db.Exec(insertSQL)
	if err != nil {
		fmt.Println("err2")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

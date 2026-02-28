package main

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Record struct {
	ID         int    `json:"id"`
	DeviceName string `json:"device_name"`
	Voltage    int    `json:"voltage"`
	RecordDate string `json:"record_date"`
}

var db *sql.DB

func initDB() {
	var err error
	// УВАГА: Заміни "твій_пароль" на пароль від MySQL!
	dsn := "root:1111@tcp(127.0.0.1:3306)/surge_protection"
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Помилка БД:", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("БД не відповідає:", err)
	}
	log.Println("Підключення до MySQL успішне.")
}

// УСКЛАДНЕННЯ 1: Middleware для логування (Error-handling & Logging)
func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next(w, r)
		log.Printf("[%s] %s %s - оброблено за %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	}
}

// УСКЛАДНЕННЯ 2: Базова авторизація (Basic Auth)
func basicAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		// Логін: admin, Пароль: secret
		if !ok || user != "admin" || pass != "secret" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted Access"`)
			http.Error(w, "Неавторизований доступ", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// Функція для отримання даних з БД (використовується і для UI, і для API)
func getRecordsFromDB() ([]Record, error) {
	rows, err := db.Query("SELECT id, device_name, voltage, record_date FROM voltage_logs ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var rec Record
		if err := rows.Scan(&rec.ID, &rec.DeviceName, &rec.Voltage, &rec.RecordDate); err == nil {
			records = append(records, rec)
		}
	}
	return records, nil
}

// Обробник веб-інтерфейсу (HTML)
func uiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		name := r.FormValue("deviceName")
		voltage := r.FormValue("voltage")
		date := r.FormValue("date")

		_, err := db.Exec("INSERT INTO voltage_logs (device_name, voltage, record_date) VALUES (?, ?, ?)", name, voltage, date)
		if err != nil {
			log.Println("Помилка запису в БД:", err)
			http.Error(w, "Помилка збереження", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	records, err := getRecordsFromDB()
	if err != nil {
		http.Error(w, "Помилка БД", http.StatusInternalServerError)
		return
	}

	tmpl, _ := template.ParseFiles("templates/index.html")
	tmpl.Execute(w, records)
}

// Обробник REST API (JSON)
func apiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		records, err := getRecordsFromDB()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Помилка БД"})
			return
		}
		json.NewEncoder(w).Encode(records)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Метод не підтримується. Використовуйте GET."})
	}
}

func main() {
	initDB()
	defer db.Close()

	// Захищаємо головну сторінку авторизацією та додаємо логування
	http.HandleFunc("/", loggingMiddleware(basicAuthMiddleware(uiHandler)))

	// API залишаємо відкритим для інших систем (тільки логування)
	http.HandleFunc("/api/devices", loggingMiddleware(apiHandler))

	port := ":8080"
	log.Printf("MVP Застосунок запущено на http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

// SubscribeRequest 定義前端送來的 JSON 結構
type SubscribeRequest struct {
	UserID        string              `json:"userId"`
	Subscriptions map[string][]string `json:"subscriptions"`
}

// Response 用來回應前端的訊息
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

var db *sql.DB

func main() {
	// 載入.env檔案
	err := godotenv.Load()
	if err != nil {
		log.Printf("【ERROR】Error loading .env file: %v", err)
	}

	// ----------------------
	// ---    mariadb     ---
	// ----------------------
	dbMysql := os.Getenv("MADB_MYSQL")
	dbUsername := os.Getenv("MADB_USERNAME")
	dbPassword := os.Getenv("MADB_PASSWORD")
	dbRequest := os.Getenv("MADB_REQUEST")
	dbDatabase := os.Getenv("MADB_DATABASE")

	if dbMysql == "" || dbUsername == "" || dbPassword == "" || dbRequest == "" || dbDatabase == "" {
		log.Fatalf("【FATAL】缺少DATABASE必要的參數")
	}

	// ----- maridb連線 -----
	//
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", dbUsername, dbPassword, dbRequest, dbDatabase)
	db, err := sql.Open(dbMysql, dsn)

	// 測試資料庫連線
	if err != nil {
		log.Printf("【FATAL】Failed to connect to database: %v", err)
	}
	defer db.Close()

	fmt.Println("Connected to MariaDB successfully")

	// 註冊 HTTP 路徑與處理函式
	http.HandleFunc("/subscribeAll", subscribeHandler)

	// 設定伺服器監聽的 port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// subscribeHandler 處理 /subscribeAll POST 請求
func subscribeHandler(w http.ResponseWriter, r *http.Request) {
	// 只接受 POST 方法
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析 JSON 請求
	var req SubscribeRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 開始交易
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 準備 INSERT 語句
	stmt, err := tx.Prepare("INSERT INTO subscriptions (user_id, county, town, created_at) VALUES (?, ?, ?, ?)")
	if err != nil {
		tx.Rollback()
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	now := time.Now()

	// 針對每個縣市與其鄉鎮逐筆寫入資料庫
	for county, towns := range req.Subscriptions {
		for _, town := range towns {
			_, err := stmt.Exec(req.UserID, county, town, now)
			if err != nil {
				tx.Rollback()
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}
	}

	// 提交交易
	if err := tx.Commit(); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 回傳成功訊息
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Success: true})
}

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
	//_ "github.com/lib/pq" // Поддержка PostgreSQL
)

var DB *sql.DB

func InitDatabase() {
	dbType := os.Getenv("DB_TYPE")
	dbPath := os.Getenv("DB_PATH")

	var err error

	if dbType == "sqlite" {
		DB, err = sql.Open("sqlite", dbPath)
	} else if dbType == "postgres" {
		DB, err = sql.Open("postgres", dbPath) // dbPath тут = строка подключения к PostgreSQL
	} else {
		log.Fatal(" Неизвестный тип БД. Укажи DB_TYPE=sqlite или postgres")
	}

	if err != nil {
		log.Fatal("Не удалось подключиться к базе:", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS requests (
		id SERIAL PRIMARY KEY,
		username TEXT,
		timestamp TEXT,
		user_text TEXT,
		gpt_response TEXT,
		status TEXT
	)
	`

	_, err = DB.Exec(createTableSQL)
	if err != nil {
		log.Fatal("Ошибка при создании таблицы:", err)
	}

	fmt.Println(" База данных подключена и готова")
}

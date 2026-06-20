package db

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

var DB *pgx.Conn

func Connect(){
	connStr := "postgres://admin:admin123@localhost:5432/paymentdb"

	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Fatal("Unable to connect to database:", err)
	}

	err = conn.Ping(context.Background())
	if err != nil {
		log.Fatal("Database did not respond:", err)
	}

	DB = conn
	fmt.Println("Connected to database successfully")
}
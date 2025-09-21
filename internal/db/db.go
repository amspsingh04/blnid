package db

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func Connect(connStr string) {
	var err error
	Pool, err = pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatal("Unable to connect to database:", err)
	}

	err = Pool.Ping(context.Background())
	if err != nil {
		log.Fatal("Cannot ping database:", err)
	}

	log.Println("Connected to Postgres successfully!")
}

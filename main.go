package main

import (
	"MCAuth-server/internal/database"
	minecraft2 "MCAuth-server/internal/minecraft"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	favicon, err := os.ReadFile("./server-icon.png")
	if err == nil && len(favicon) > 0 {
		minecraft2.Favicon = base64.StdEncoding.EncodeToString(favicon)
	}

	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "25565")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		getEnv("POSTGRES_HOST", "localhost"),
		getEnv("POSTGRES_PORT", "5432"),
		os.Getenv("POSTGRES_DATABASE"),
	))
	if err != nil {
		log.Fatal("Failed to connect to PostgreSQL: ", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("PostgreSQL ping failed: ", err)
	}

	log.Println("Connected to PostgreSQL")

	queries := database.New(pool)

	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		redisDB = 0
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       redisDB,
	})
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis ping failed: ", err)
	}

	log.Println("Connected to Redis")
	log.Println("Starting Minecraft authentication server")

	minecraft2.NewMinecraftServer(func(client *minecraft2.Client, profile minecraft2.GameProfile, disconnect func(string)) {
		handlePlayer(client, profile, disconnect, queries, rdb)
	})
}

func handlePlayer(client *minecraft2.Client, profile minecraft2.GameProfile, disconnect func(string), queries *database.Queries, rdb *redis.Client) {
	disconnect("not implemented")
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

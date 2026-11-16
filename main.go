package main

import (
	"MCAuth-server/internal/database"
	"MCAuth-server/internal/helpers"
	minecraft2 "MCAuth-server/internal/minecraft"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgtype"
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
	addr := client.ServerAddress
	if before, ok := strings.CutSuffix(addr, ".mcauth.net"); ok {
		token := before

		ctx, cancel := helpers.GetContext()
		defer cancel()

		exists, err := rdb.Exists(ctx, fmt.Sprintf("token:%s", token)).Result()
		if err != nil {
			panic(err)
		}
		if exists > 0 {
			err := rdb.Set(ctx, fmt.Sprintf("token:%s", token), "true", time.Minute).Err()
			if err != nil {
				panic(err)
			}
			u, err := uuid.Parse(profile.ID)
			if err != nil {
				panic(err)
			}
			id, err := queries.CreateAccount(ctx, pgtype.UUID{
				Bytes: u,
				Valid: true,
			})
			if err != nil {
				id, err = queries.CheckAccount(ctx, pgtype.UUID{
					Bytes: u,
					Valid: true,
				})
				if err != nil {
					panic(err)
				}
			}
			ip, err := netip.ParseAddr(strings.Split(client.Conn.RemoteAddr().String(), ":")[0])
			if err != nil {
				panic(err)
			}
			_, err = queries.CreateLogin(ctx, database.CreateLoginParams{
				AccountID: id,
				Uuid: pgtype.UUID{
					Bytes: u,
					Valid: true,
				},
				Token:           token,
				Ip:              ip,
				ProtocolVersion: int16(client.Protocol),
				Brand: pgtype.Text{
					String: "",
					Valid:  false,
				},
				Lang: pgtype.Text{
					String: "",
					Valid:  false,
				},
			})
			if err == nil {
				disconnect("successfully connected")
			}
		}
	}

	disconnect("not implemented")
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

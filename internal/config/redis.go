package config

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var (
	Ctx   = context.Background()
	Redis *redis.Client
)

func InitRedis() {
	db, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		db = 0
	}

	Redis = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	})

	if err := Redis.Ping(Ctx).Err(); err != nil {
		log.Fatal("Redis tidak nyambung:", err)
	}

	log.Println("Redis connected (DB", db, ")")
}

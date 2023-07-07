package gredis

import "github.com/go-redis/redis/v8"

var MainClient *redis.Client

func SetUp() {
	MainClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
}

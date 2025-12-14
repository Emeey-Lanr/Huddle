package connection

import (
	"github.com/redis/go-redis/v9"
	
	"context"
	"log"
	
)



var ctx = context.Background()

func ConnectToRedis ()  *redis.Client{
 rdb := redis.NewClient(&redis.Options{
	Addr:"localhost:6379",
	Password:"",
	DB:0,

 })

 value, err := rdb.Ping(ctx).Result()
 if err != nil{
	panic(err)
 }

 log.Println(value, "--")

 return  rdb

}
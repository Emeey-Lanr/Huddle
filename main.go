package main

import (
	"huddle/db/connection"
	"huddle/pkg/router"

	"github.com/joho/godotenv"
	"log"

	
)





func main (){
 
 err := godotenv.Load()	
 if err != nil{
	log.Println(err, "unable to load env")
 }


db :=	connection.ConnectToDb()
rdb := connection.ConnectToRedis()

r :=  router.RoutesSetup(db, rdb)

r.Run(":8080")
}
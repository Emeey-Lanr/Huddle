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

r :=  router.RoutesSetup(db)

r.Run(":8080")
}
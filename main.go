package main

import (
	"huddle/db/connection"
	"huddle/setup"
)





func main (){
 
db :=	connection.ConnectToDb()

setup.RoutesSetup(db)
}
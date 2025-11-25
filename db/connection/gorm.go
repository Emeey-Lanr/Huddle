package connection

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"gorm.io/driver/postgres"
)



func ConnectToDb () *gorm.DB {

host := os.Getenv("DB_HOST") 
user := os.Getenv("DB_USER")
password:= os.Getenv("DB_PASSWORD")
dbName:= os.Getenv("DB_NAME")
port := os.Getenv("DB_PORT")
dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", host, user, password, dbName, port )
	
log.Println(dsn)
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
	NamingStrategy: schema.NamingStrategy{
		SingularTable: true, //cancels pularization
	},
	PrepareStmt: true,
	NowFunc: func() time.Time {return time.Now().UTC()},
})

if err != nil{
	panic("Unable to connect to db"+ err.Error())
}

log.Println("DB connected")

return db
}
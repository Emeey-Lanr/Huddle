package connection

import (
	"os"
	"time"
	"log"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"gorm.io/driver/postgres"
)



func ConnectToDb () *gorm.DB {
dsn :=  os.Getenv("")
	
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
	NamingStrategy: schema.NamingStrategy{
		SingularTable: true, //cancels pularization
	},
	PrepareStmt: true,
	NowFunc: func() time.Time {return time.Now().UTC()},
})

if err != nil{
	log.Println("Unable to connect to db")
}


return db
}
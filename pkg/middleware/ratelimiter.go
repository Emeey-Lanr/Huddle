package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"log"
)



type RateData struct{
	Time time.Time
	Retry int
}
var users  = make(map[string]*RateData)

func RateLimiter(c *gin.Context) {
   clientIp := c.ClientIP()

   log.Println(clientIp)

   _, exist := users[clientIp]

   if !exist {
     users[clientIp] = &RateData{Time: time.Now(), Retry: 1}
	 c.Next()	  
   }else {
        users[clientIp].Retry++
         log.Println(time.Since(users[clientIp].Time), "kkk")

     if time.Since(users[clientIp].Time) >= 1 * time.Minute{
        if users[clientIp].Retry >= 4{

			time.Sleep(10 * time.Second)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message":"too many request"})
			return
		}else{
			delete(users, clientIp)
			c.Next()
		}

	 }

   }

}

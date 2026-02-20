package main

import (
	"fmt"
	"time"

	rl "github.com/rishavvajpayee/go-ratelimiter/ratelimiter"
)

func main() {
	fmt.Println("Go in-memory Rate limiter")

	rl := rl.NewRateLimiter()
	rl.StartCleanUp(1*time.Minute, 5*time.Minute)
	userID := int64(1)

	for i := range 12 {
		allowed := rl.AllowRequest(userID)
		fmt.Println("Request :", i+1, "Allowed :", allowed)
	}

}

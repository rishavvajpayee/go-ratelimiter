package main

import (
	"fmt"
	"sync"
	"time"
)

const N = 10              // number of requests
const T = 1 * time.Minute // minutes

const WINDOW = 20 * time.Minute

type Queue []int64

func (q *Queue) Enqueue(value int64) {
	*q = append(*q, value)
}

func (q *Queue) Dequeue() (int64, error) {
	if len(*q) == 0 {
		return 0, fmt.Errorf("Queue is empty")
	}

	value := (*q)[0]
	(*q)[0] = 0
	*q = (*q)[1:]

	return value, nil
}

const ShardCount = 64

type Shard struct {
	mu    sync.Mutex
	users map[int64]*UserLimiter
}

type UserLimiter struct {
	mu       sync.Mutex
	queue    Queue
	lastSeen time.Time
}

type RateLimiter struct {
	shards [ShardCount]*Shard
}

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{}

	for i := 0; i < ShardCount; i++ {
		rl.shards[i] = &Shard{
			users: make(map[int64]*UserLimiter),
		}
	}

	return rl
}

func (rl *RateLimiter) getShard(userID int64) *Shard {
	return rl.shards[userID%ShardCount]
}

func (rl *RateLimiter) StartCleanUp(interval time.Duration, idleDuration time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for now := range ticker.C {
			for _, s := range rl.shards {
				s.mu.Lock()
				for userID, ul := range s.users {
					ul.mu.Lock()
					cutoff := now.Add(-WINDOW).UnixNano()
					for len(ul.queue) > 0 && ul.queue[0] < cutoff {
						ul.queue.Dequeue()
					}
					idle := now.Sub(ul.lastSeen) > idleDuration
					empty := len(ul.queue) == 0

					if idle && empty {
						delete(s.users, userID)
						ul.mu.Unlock()
						continue
					}
					ul.mu.Unlock()
				}
				s.mu.Unlock()
			}
		}
	}()
}

func (rl *RateLimiter) GetUserLimiter(userID int64) *UserLimiter {
	s := rl.getShard(userID)

	s.mu.Lock()
	ul, exists := s.users[userID]
	s.mu.Unlock()
	if exists {
		return ul
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if ul, exists = s.users[userID]; exists {
		return ul
	}

	ul = &UserLimiter{
		queue: make(Queue, 0),
	}

	s.users[userID] = ul
	return ul
}

func (rl *RateLimiter) AllowRequest(userID int64) bool {
	now := time.Now().UnixNano()
	cutOff := time.Now().Add(-T).UnixNano()

	ul := rl.GetUserLimiter(userID)
	ul.lastSeen = time.Now()
	ul.mu.Lock()
	defer ul.mu.Unlock()

	for len(ul.queue) > 0 && ul.queue[0] < cutOff {
		_, e := ul.queue.Dequeue()
		if e != nil {
			fmt.Println("Something went wrong while dequeuing")
			return false
		}
	}

	if len(ul.queue) >= N {
		return false
	}

	ul.queue.Enqueue(now)
	return true
}

func main() {
	fmt.Println("Hello Rate limiter")

	rl := NewRateLimiter()
	rl.StartCleanUp(1*time.Minute, 5*time.Minute)
	userID := int64(1)

	for i := range 12 {
		allowed := rl.AllowRequest(userID)
		fmt.Println("Request :", i+1, "Allowed :", allowed)
	}

}

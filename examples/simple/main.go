package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/KronusRodion/cbreaker"
	"github.com/KronusRodion/cbreaker/domain"
)

type user struct {
	name string
	age  int
}

func main() {
	// Must add resources like db or brokers, before use it
	cbreaker.AddResources("postgres", "redis")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	getUser := func(ctx context.Context) (user, error) {
		v := rand.Intn(100)
		if v < 80 {
			return user{}, errors.New("connection close")
		}
		return user{name: "Rodni", age: 11}, nil
	}

	var wg sync.WaitGroup

	errStat := atomic.Int32{}
	rejectStat := atomic.Int32{}
	successStat := atomic.Int32{}

	// use func Call from packet github.com/KronusRodion/cbreaker
	for range 100 {
		wg.Go(func() {
			_, err := cbreaker.Call(ctx, "postgres", getUser)
			if err != nil {
				switch {
				case errors.Is(err, domain.ErrCircuitOpen):
					rejectStat.Add(1)
				default:
					errStat.Add(1)
				}
			} else {
				successStat.Add(1)
			}
		})
	}

	wg.Wait()

	log.Println("Errors: ", errStat.Load())
	log.Println("Rejected: ", rejectStat.Load())
	log.Println("Success: ", successStat.Load())

}

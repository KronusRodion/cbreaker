package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"sync"
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

	// use func Call from packet github.com/KronusRodion/cbreaker
	for range 100 {
		wg.Go(func() {
			user, err := cbreaker.Call(ctx, "postgres", getUser)
			if err != nil {
				switch {
				case errors.Is(err, domain.ErrCircuitOpen):
					log.Println("The service is unavailable")
				default:
					log.Println("undefined error: ", err)
				}
			} else {
				log.Println(user)
			}
		})
	}

	wg.Wait()

}

package main

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/KronusRodion/cbreaker"
	"github.com/KronusRodion/cbreaker/domain"
)

type user struct {
	id   string
	name string
	age  int
}

type repository struct{}

func (r repository) GetUserByID(ctx context.Context, id string) (user, error) {
	v := rand.Intn(100)
	if v < 80 {
		return user{}, errors.New("connection close")
	}
	return user{id: id, name: "Rodni", age: 11}, nil
}

type usecase struct {
	repo repository
}

func (u usecase) GetUserByID(ctx context.Context, id string) (user, error) {
	us, err := cbreaker.Call[user](ctx, "sqlite", func(ctx context.Context) (user, error) {
		return u.repo.GetUserByID(ctx, id)
	})

	// as example of err handle, you can just return err or try to retry with backoff
	if errors.Is(err, domain.ErrCircuitOpen) {
		timer := time.NewTimer(cbreaker.CurrentConfig().Timeout)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return user{}, errors.New("timeout error")
		case <-timer.C:
			// we try to retry after cfg timeout
			us, err = cbreaker.Call[user](ctx, "sqlite", func(ctx context.Context) (user, error) {
				return u.repo.GetUserByID(ctx, id)
			})
		}
	}

	return us, err
}

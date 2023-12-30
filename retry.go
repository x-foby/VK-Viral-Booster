package main

import (
	"time"

	"github.com/SevereCloud/vksdk/v2/api"
)

type executor interface {
	Execute(string, any) error
}

func executeWithRetries(e executor, data string, target any) (err error) {
	const maxRetries = 10

	for i := 0; i < maxRetries; i++ {
		err = e.Execute(data, target)
		if err == nil {
			return nil
		}

		time.Sleep(time.Second)
	}

	return
}

type requestFunc[T any] func(api.Params) (T, error)

func requestWithRetries[T any](f requestFunc[T], params api.Params) (resp T, err error) {
	const maxRetries = 10

	for i := 0; i < maxRetries; i++ {
		resp, err = f(params)
		if err == nil {
			return resp, nil
		}

		time.Sleep(time.Second)
	}

	return
}

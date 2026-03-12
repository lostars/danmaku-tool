package test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
)

func Test(t *testing.T) {
	url := "http://localhost:8089/api/v1/1/search/anime?keyword=仙剑"
	wg := sync.WaitGroup{}
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := http.Get(url)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(resp.Status)
		}(w)
	}
	wg.Wait()
}

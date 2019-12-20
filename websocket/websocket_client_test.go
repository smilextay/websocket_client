package websocket

import (
	"log"
	"sync"
	"testing"
)

func Test_main(t *testing.T) {
	var wg sync.WaitGroup

	client, err := NewClient(&Config{
		URL:              "ws://127.0.0.1:8080/echo",
		Origin:           "http://127.0.0.1:8080/",
		EvtMessagePrefix: []byte("iris-websocket-message:"),
	})
	if err != nil {
		log.Println(err)
		return
	}
	wg.Add(1)
	client.On("chat", func(msg string) {

		t.Log(msg)
		wg.Done()
	})
	client.Emit(`chat`, "hello")
	wg.Wait()
}

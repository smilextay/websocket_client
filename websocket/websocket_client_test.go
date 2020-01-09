package websocket

import (
	"log"
	"sync"
	"testing"
)

func Test_main(t *testing.T) {
	var wg sync.WaitGroup

	client, err := NewClient(&Config{
		URL:              "ws://127.0.0.1/connect",
		Origin:           "http://localhost/connect/",
		EvtMessagePrefix: []byte("xiaojia365-websocket-protocol:"),
	})
	if err != nil {
		log.Println(err)
		return
	}
	client.OnDisconnect(func() {
		log.Print("lost")
	})
	wg.Add(1)
	client.On("login", func(msg interface{}) {

		t.Log(msg)
		wg.Done()
	})
	client.On("login", func(msg interface{}) {

		t.Log(msg)
		wg.Done()
	})
	client.On("getstate", func(msg interface{}) {

		t.Log(msg)
		wg.Done()
	})
	client.Emit(`chat`, "hello")
	wg.Wait()
}

package websocket

import (
	"bytes"
	"context"
	"log"
	"net"
	"strconv"
	"sync"

	"golang.org/x/net/websocket"
)

type (
	//DebugFunc Function for debug
	//Return the source data of every commnit
	DebugFunc func([]byte)
	// DisconnectFunc is the callback which is fired when a client/connection closed
	DisconnectFunc func()
	// ErrorFunc is the callback which fires whenever an error occurs
	ErrorFunc (func(error))
	// PingFunc is the callback which fires each ping
	PingFunc func()
	// PongFunc is the callback which fires on pong message received
	PongFunc func()
	// NativeMessageFunc is the callback for native websocket messages, receives one []byte parameter which is the raw client's message
	NativeMessageFunc func([]byte)
	// MessageFunc is the second argument to the Emitter's Emit functions.
	// A callback which should receives one parameter of type string, int, bool or any valid JSON/Go struct
	MessageFunc interface{}
	//Config ws 配置信息
	Config struct {
		// A WebSocket server address.
		URL string

		// A Websocket client origin.
		Origin string

		// WebSocket subprotocols.
		Protocol string

		//EvtMessagePrefix  custom event prefix
		EvtMessagePrefix []byte

		//ReadBufferSize  the buff size for the connection reader
		//Defaul value is 4096
		ReadBufferSize int
	}
	// WsClient is the front-end API that you will use to communicate with the server side
	WsClient interface {

		// Write writes a raw websocket message with a specific type to the client
		// used by ping messages and any CloseMessage types.
		Write(websocketMessageType int, data []byte) error

		// Context returns the (upgraded) context.Context of this connection
		// avoid using it, you normally don't need it,
		// websocket has everything you need to authenticate the user BUT if it's necessary
		// then  you use it to receive user information, for example: from headers
		Context() context.Context

		// OnDisconnect registers a callback which is fired when this connection is closed by an error or manual
		OnDisconnect(DisconnectFunc)
		// OnError registers a callback which fires when this connection occurs an error
		OnError(ErrorFunc)
		// OnPing  registers a callback which fires on each ping
		OnPing(PingFunc)
		// OnPong  registers a callback which fires on pong message received
		OnPong(PongFunc)
		OnDebug(DebugFunc)
		// FireOnError can be used to send a custom error message to the connection
		//
		// It does nothing more than firing the OnError listeners. It doesn't send anything to the client.
		FireOnError(err error)

		// OnMessage registers a callback which fires when native websocket message received
		OnMessage(NativeMessageFunc)
		// On registers a callback to a particular event which is fired when a message to this event is received
		On(string, ...MessageFunc)
		Emit(event string, message interface{}) error
		// Disconnect disconnects the client, close the underline websocket conn and removes it from the conn list
		// returns the error, if any, from the underline connection
		Disconnect() error
	}
	//Client WebSocket Client
	Client struct {
		*websocket.Conn
		config *Config
		// onConnectionListeners    []ConnectionFunc
		onNativeMessageListeners []NativeMessageFunc
		onDisconnectListeners    []DisconnectFunc
		onErrorListeners         []ErrorFunc
		onPingListeners          []PingFunc
		onPongListeners          []PongFunc
		onEventListeners         sync.Map
		messageSerializer        *messageSerializer
		onDebugListeners         []DebugFunc
	}

	//EventLister 事件绑定
	EventLister struct {
		Type        string
		MessageFunc interface{}
	}
)

var (
	defaultEvtMessagePrefix = []byte("Ws_golang")

	defaultReadBufferSize = 4096
)

//NewClient 创建一新的websocket客户端
func NewClient(conf *Config, event ...EventLister) (WsClient, error) {

	if conf.EvtMessagePrefix == nil {
		conf.EvtMessagePrefix = defaultEvtMessagePrefix
	}
	config := &Config{
		URL:      conf.URL,
		Protocol: conf.Protocol,
		Origin:   conf.Origin,
	}
	if conf.ReadBufferSize == 0 {
		config.ReadBufferSize = defaultReadBufferSize
	} else {
		config.ReadBufferSize = conf.ReadBufferSize
	}

	if conf.EvtMessagePrefix == nil {
		config.EvtMessagePrefix = defaultEvtMessagePrefix
	} else {
		config.EvtMessagePrefix = conf.EvtMessagePrefix
	}
	ws, err := websocket.Dial(conf.URL, conf.Protocol, conf.Origin)
	if err != nil {
		return nil, err
	}
	client := &Client{
		Conn:   ws,
		config: config,
		// onEventListeners:  map[string][]MessageFunc{},
		messageSerializer: newMessageSerializer(config.EvtMessagePrefix),
	}
	for _, v := range event {
		client.On(v.Type, v.MessageFunc)
	}
	go client.startReader()
	return client, nil
}

//Write ...
func (c *Client) Write(websocketMessageType int, data []byte) error {
	return nil
}

//Context ...
func (c *Client) Context() context.Context {
	return c.Conn.Request().Context()
}

// OnDisconnect registers a callback which is fired when this connection is closed by an error or manual
func (c *Client) OnDisconnect(cb DisconnectFunc) {
	c.onDisconnectListeners = append(c.onDisconnectListeners, cb)
}

// OnError registers a callback which fires when this connection occurs an error
func (c *Client) OnError(cb ErrorFunc) {
	c.onErrorListeners = append(c.onErrorListeners, cb)
}

// OnPing  registers a callback which fires on each ping
func (c *Client) OnPing(cb PingFunc) {
	c.onPingListeners = append(c.onPingListeners, cb)
}

// OnPong  registers a callback which fires on pong message received
func (c *Client) OnPong(cb PongFunc) {
	c.onPongListeners = append(c.onPongListeners, cb)
}

//OnDebug method for debug
//registers a acllback whitch fires on every send and receive
func (c *Client) OnDebug(cb DebugFunc) {
	c.onDebugListeners = append(c.onDebugListeners, cb)
}

// FireOnError can be used to send a custom error message to the connection
//
// It does nothing more than firing the OnError listeners. It doesn't send anything to the client.
func (c *Client) FireOnError(err error) {
	for _, cb := range c.onErrorListeners {
		cb(err)
	}
}

// OnMessage registers a callback which fires when native websocket message received
func (c *Client) OnMessage(cb NativeMessageFunc) {
	c.onNativeMessageListeners = append(c.onNativeMessageListeners, cb)
}

// On registers a callback to a particular event which is fired when a message to this event is received
func (c *Client) On(event string, cb ...MessageFunc) {
	if cb == nil {
		return
	}
	ls, ok := c.onEventListeners.LoadOrStore(event, cb)
	if ok {
		//	value was loaded
		return
	}
	cbs, ok := ls.([]MessageFunc)
	if !ok {
		c.onEventListeners.Store(event, cb)
		return
	}
	cbs = append(cbs, cb)
	c.onEventListeners.Store(event, cbs)
}

// Disconnect disconnects the client, close the underline websocket conn and removes it from the conn list
// returns the error, if any, from the underline connection
func (c *Client) Disconnect() error {
	if c.Conn == nil {
		return nil
	}
	c.fireDisconnect()
	return c.Conn.Close()
}

//Emit ...
func (c *Client) Emit(event string, message interface{}) error {
	msg, err := c.messageSerializer.serialize(event, message)
	if err != nil {
		return err
	}
	for _, v := range c.onDebugListeners {
		v(msg)
	}
	_, err = c.Conn.Write(msg)
	if err != nil {
		er, ok := err.(*net.OpError)
		if ok {
			log.Print(er)
			c.Disconnect()
		} else {
			c.FireOnError(err)
		}
	}
	return nil
}

func (c *Client) startReader() {

	defer func() {
		c.Disconnect()
	}()
	var tmp []byte
	data := make([]byte, c.config.ReadBufferSize)
	for {

		// if hasReadTimeout {
		// 	// set the read deadline based on the configuration
		// 	conn.SetReadDeadline(time.Now().Add(c.server.config.ReadTimeout))
		// }

		count, err := c.Conn.Read(data)
		if err != nil {
			if IsUnexpectedCloseError(err, CloseGoingAway) {
				c.FireOnError(err)
			}
			break
		} else {
			//拆包

			if count >= 4092 && len(tmp) == 0 {
				tmp = data[:count]
				continue
			}

			c.messageReceived(append(tmp[:], data[:count]...))
			if len(tmp) == 0 {
				tmp = []byte{}
			}
		}
	}

}

// IsUnexpectedCloseError returns boolean indicating whether the error is a
// *CloseError with a code not in the list of expected codes.
func IsUnexpectedCloseError(err error, expectedCodes ...int) bool {
	if e, ok := err.(*CloseError); ok {
		for _, code := range expectedCodes {
			if e.Code == code {
				return false
			}
		}
		return true
	}
	return false
}

// messageReceived checks the incoming message and fire the nativeMessage listeners or the event listeners (ws custom message)
func (c *Client) messageReceived(data []byte) {
	for _, v := range c.onDebugListeners {
		v(data)
	}
	if bytes.HasPrefix(data, c.config.EvtMessagePrefix) {
		//it's a custom ws message
		receivedEvt := c.messageSerializer.getWebsocketCustomEvent(data)
		value, ok := c.onEventListeners.Load(string(receivedEvt))
		if !ok || value == nil {
			return
		}

		listeners, ok := value.([]MessageFunc)
		if !ok || len(listeners) == 0 {
			return // if not listeners for this event exit from here
		}

		customMessage, err := c.messageSerializer.deserialize(receivedEvt, data)
		if customMessage == nil || err != nil {
			return
		}

		for i := range listeners {
			if fn, ok := listeners[i].(func()); ok { // its a simple func(){} callback
				fn()
			} else if fnString, ok := listeners[i].(func(string)); ok {

				if msgString, is := customMessage.(string); is {
					fnString(msgString)
				} else if msgInt, is := customMessage.(int); is {
					// here if server side waiting for string but client side sent an int, just convert this int to a string
					fnString(strconv.Itoa(msgInt))
				}

			} else if fnInt, ok := listeners[i].(func(int)); ok {
				fnInt(customMessage.(int))
			} else if fnBool, ok := listeners[i].(func(bool)); ok {
				fnBool(customMessage.(bool))
			} else if fnBytes, ok := listeners[i].(func([]byte)); ok {
				fnBytes(customMessage.([]byte))
			} else {
				listeners[i].(func(interface{}))(customMessage)
			}

		}
	} else {
		// it's native websocket message
		for i := range c.onNativeMessageListeners {
			c.onNativeMessageListeners[i](data)
		}
	}

}
func (c *Client) fireDisconnect() {
	for i := range c.onDisconnectListeners {
		c.onDisconnectListeners[i]()
	}
}

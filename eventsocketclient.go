package eventsocketclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var d websocket.Dialer

func init() {
	d = websocket.Dialer{
		NetDial:          nil,
		TLSClientConfig:  nil,
		HandshakeTimeout: time.Second * 5,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}
}

// Client is the main eventsocket client
type Client struct {
	// The client Id, provided by the server
	Id string `json:"Id"`

	// channels for receiving messages
	RecvBroadcast chan *Received `json:"-"`
	RecvRequest   chan *Received `json:"-"`
	RecvError     chan error     `json:"-"`

	// the URL to the server
	url string

	// the actual websocket connection, after we've connected
	ws *websocket.Conn

	// a mapping of all requests that are currently inflight
	liveRequests map[string]chan *Received

	// a mapping of the subscription return channels
	subscriptions map[string]chan *Received
}

// NewClient registers a new client with the server
func NewClient(url string) (*Client, error) {
	resp, err := http.Post(fmt.Sprintf("http://%s/v1/clients", url), "application/json", strings.NewReader(""))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	client := Client{
		url:           url,
		liveRequests:  make(map[string]chan *Received),
		subscriptions: make(map[string]chan *Received),
		RecvBroadcast: make(chan *Received),
		RecvRequest:   make(chan *Received),
		RecvError:     make(chan error),
	}

	// unmarshal the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(body, &client)

	return &client, nil
}

// DialWs will dial the websocket server, and get a websocket connection in return
func (client *Client) DialWs() error {
	headers := http.Header{}
	ws, _, err := d.Dial(fmt.Sprintf("ws://%s/v1/clients/%s/ws", client.url, client.Id), headers)
	if err != nil {
		return err
	}

	client.ws = ws

	return nil
}

// Reconnect attempts a reconnect with the server
func (client *Client) Reconnect() error {
	client.ws.Close()

	return client.DialWs()
}

// Close closes the connections
func (client *Client) Close() {
	client.ws.Close()
}

// Recv reads from the socket
func (client *Client) Recv() error {
	for {
		m := &Message{}
		if err := client.ws.ReadJSON(m); err != nil {
			// TODO: something other than panic
			client.RecvError <- err
			continue
		}

		r := &Received{
			Message: m,
			Err:     nil,
		}

		switch m.MessageType {
		case MESSAGE_TYPE_BROADCAST:
			client.RecvBroadcast <- r
		case MESSAGE_TYPE_STANDARD:
			client.subscriptions[r.Message.Event] <- r
		case MESSAGE_TYPE_REQUEST:
			client.RecvRequest <- r
		case MESSAGE_TYPE_REPLY:
			client.liveRequests[m.RequestId] <- r
			close(client.liveRequests[m.RequestId])
			delete(client.liveRequests, m.RequestId)
		}
	}

	return nil
}

// Broadcast a payload
func (client *Client) Broadcast(p *Payload) error {
	m := &Message{
		MessageType: MESSAGE_TYPE_BROADCAST,
		Payload:     p,
	}

	return client.write(m)
}

// Emit an event
func (client *Client) Emit(event string, p *Payload) error {
	m := &Message{
		MessageType: MESSAGE_TYPE_STANDARD,
		Event:       event,
		Payload:     p,
	}

	return client.write(m)
}

// Suscribe to an event(s)
func (client *Client) Suscribe(events ...string) (<-chan *Received, error) {
	p := NewPayload()
	p["Events"] = events

	m := &Message{
		MessageType: MESSAGE_TYPE_SUSCRIBE,
		Payload:     &p,
	}

	if err := client.write(m); err != nil {
		return nil, err
	}

	// create the return channel
	c := make(chan *Received)
	for _, event := range events {
		client.subscriptions[event] = c
	}

	return c, nil
}

// Request will execute a request against a client
func (client *Client) Request(id string, p *Payload) (<-chan *Received, error) {
	requestID := makeUuid()
	m := &Message{
		MessageType:     MESSAGE_TYPE_REQUEST,
		RequestId:       requestID,
		RequestClientId: id,
		Payload:         p,
	}

	if err := client.write(m); err != nil {
		return nil, err
	}

	c := make(chan *Received)
	client.liveRequests[requestID] = c
	return c, nil
}

// Reply sends a reply to a request
func (client *Client) Reply(requestID, cid string, p *Payload) error {
	m := &Message{
		MessageType:   MESSAGE_TYPE_REPLY,
		ReplyClientId: cid,
		RequestId:     requestID,
		Payload:       p,
	}

	return client.write(m)

}

// write a message to the socket
func (client *Client) write(m *Message) error {
	return client.ws.WriteJSON(m)
}

// SetMaxMessageSize sets the max message size. Note: this is not the max frame size
func (client *Client) SetMaxMessageSize(limit int64) {
	client.ws.SetReadLimit(limit)
}

// SetReadDeadline sets the deadline to receive a response on the socket
func (client *Client) SetReadDeadline(t time.Duration) {
	client.ws.SetReadDeadline(time.Now().Add(t))
}

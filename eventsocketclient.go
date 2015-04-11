package eventsocketclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/dronemill/eventsocket-client-go/Godeps/_workspace/src/github.com/gorilla/websocket"
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

type Client struct {
	// The client Id, provided by the server
	Id string `json:"Id"`

	RecvBroadcast chan *Received
	RecvStandard  chan *Received
	RecvRequest   chan *Received

	// the URL to the server
	url string `json:-`

	// the actual websocket connection, after we've connected
	ws *websocket.Conn

	// a mapping of all requests that are currently inflight
	liveRequests map[string]chan *Received
}

// register a new client with the server
func NewClient(url string) (*Client, error) {
	resp, err := http.Post(fmt.Sprintf("http://%s/v1/clients", url), "application/json", strings.NewReader(""))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	client := Client{
		url:           url,
		liveRequests:  make(map[string]chan *Received),
		RecvBroadcast: make(chan *Received),
		RecvStandard:  make(chan *Received),
		RecvRequest:   make(chan *Received),
	}

	// unmarshal the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(body, &client)

	return &client, nil
}

// dial the websocket server, and get a websocket connection in return
func (client *Client) DialWs() error {
	headers := http.Header{}
	ws, _, err := d.Dial(fmt.Sprintf("ws://%s/v1/clients/%s/ws", client.url, client.Id), headers)
	if err != nil {
		return err
	}

	client.ws = ws

	return nil
}

// Receive from the socket
func (client *Client) Recv() error {
	for {
		m := &Message{}
		if err := client.ws.ReadJSON(m); err != nil {
			// TODO: something other than panic
			panic(err)
		}

		r := &Received{
			Message: m,
			Err:     nil,
		}

		switch m.MessageType {
		case MESSAGE_TYPE_BROADCAST:
			client.RecvBroadcast <- r
		case MESSAGE_TYPE_STANDARD:
			client.RecvStandard <- r
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

// broadcast a payload
func (client *Client) Broadcast(p *Payload) error {
	m := &Message{
		MessageType: MESSAGE_TYPE_BROADCAST,
		Payload:     p,
	}

	return client.write(m)
}

// emit an event
func (client *Client) Emit(event string, p *Payload) error {
	m := &Message{
		MessageType: MESSAGE_TYPE_STANDARD,
		Event:       event,
		Payload:     p,
	}

	return client.write(m)
}

// suscribe to an event(s)
func (client *Client) Suscribe(events ...string) error {
	p := NewPayload()
	p["Events"] = events

	m := &Message{
		MessageType: MESSAGE_TYPE_SUSCRIBE,
		Payload:     &p,
	}

	return client.write(m)
}

// Execute a request against a client
func (client *Client) Request(id string, p *Payload) (chan *Received, error) {
	requestId := makeUuid()
	m := &Message{
		MessageType:     MESSAGE_TYPE_REQUEST,
		RequestId:       requestId,
		RequestClientId: id,
		Payload:         p,
	}

	if err := client.write(m); err != nil {
		return nil, err
	}

	c := make(chan *Received)
	client.liveRequests[requestId] = c
	return c, nil
}

// Send a reply to a request
func (client *Client) Reply(requestId, cid string, p *Payload) error {
	m := &Message{
		MessageType:   MESSAGE_TYPE_REPLY,
		ReplyClientId: cid,
		RequestId:     requestId,
		Payload:       p,
	}

	return client.write(m)

}

// write a message to the socket
func (client *Client) write(m *Message) error {
	return client.ws.WriteJSON(m)
}

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

type Client struct {
	// The client Id, provided by the server
	Id string `json:"Id"`

	// the URL to the server
	url string `json:-`

	// the actual websocket connection, after we've connected
	ws *websocket.Conn
}

// register a new client with the server
func NewClient(url string) (*Client, error) {
	resp, err := http.Post(fmt.Sprintf("http://%s/v1/clients", url), "application/json", strings.NewReader(""))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	client := Client{
		url: url,
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

// read a message from the webscoket
func (client *Client) ReadMessage() (int, []byte, error) {
	return client.ws.ReadMessage()
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

// write a message to the socket
func (client *Client) write(m *Message) error {
	return client.ws.WriteJSON(m)
}

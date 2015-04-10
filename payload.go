package eventsocketclient

type Payload map[string]interface{}

func NewPayload() Payload {
	p := make(Payload)

	return p
}

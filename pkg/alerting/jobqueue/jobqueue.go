package jobqueue

type Message struct {
	RoutingKey string
	Payload    []byte
}

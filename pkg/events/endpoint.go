package events

import (
	"encoding/json"
	"time"

	m "github.com/raintank/worldping-api/pkg/models"
)

type EndpointCreated struct {
	Ts      time.Time
	Payload *m.EndpointDTO
}

func (a *EndpointCreated) Type() string {
	return "Endpoint.created"
}

func (a *EndpointCreated) Timestamp() time.Time {
	return a.Ts
}

func (a *EndpointCreated) Body() ([]byte, error) {
	return json.Marshal(a.Payload)
}

type EndpointDeleted struct {
	Ts      time.Time
	Payload *m.EndpointDTO
}

func (a *EndpointDeleted) Type() string {
	return "Endpoint.deleted"
}

func (a *EndpointDeleted) Timestamp() time.Time {
	return a.Ts
}

func (a *EndpointDeleted) Body() ([]byte, error) {
	return json.Marshal(a.Payload)
}

type EndpointUpdated struct {
	Ts      time.Time
	Payload struct {
		Last    *m.EndpointDTO `json:"last"`
		Current *m.EndpointDTO `json:"current"`
	}
}

func (a *EndpointUpdated) Type() string {
	return "Endpoint.updated"
}

func (a *EndpointUpdated) Timestamp() time.Time {
	return a.Ts
}

func (a *EndpointUpdated) Body() ([]byte, error) {
	return json.Marshal(a.Payload)
}

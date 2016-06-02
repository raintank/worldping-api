package events

import (
	"encoding/json"
	"time"

	m "github.com/raintank/worldping-api/pkg/models"
)

type ProbeCreated struct {
	Ts      time.Time
	Payload *m.ProbeDTO
}

func (a *ProbeCreated) Type() string {
	return "Probe.created"
}

func (a *ProbeCreated) Timestamp() time.Time {
	return a.Ts
}

func (a *ProbeCreated) Body() ([]byte, error) {
	return json.Marshal(a.Payload)
}

type ProbeDeleted struct {
	Ts      time.Time
	Payload *m.ProbeDTO
}

func (a *ProbeDeleted) Type() string {
	return "Probe.deleted"
}

func (a *ProbeDeleted) Timestamp() time.Time {
	return a.Ts
}

func (a *ProbeDeleted) Body() ([]byte, error) {
	return json.Marshal(a.Payload)
}

type ProbeUpdated struct {
	Ts      time.Time
	Payload struct {
		Last    *m.ProbeDTO `json:"last"`
		Current *m.ProbeDTO `json:"current"`
	}
}

func (a *ProbeUpdated) Type() string {
	return "Probe.updated"
}

func (a *ProbeUpdated) Timestamp() time.Time {
	return a.Ts
}

func (a *ProbeUpdated) Body() ([]byte, error) {
	return json.Marshal(a.Payload)
}

type ProbeOnline struct {
	Ts      time.Time
	Payload *m.ProbeDTO
}

func (a *ProbeOnline) Type() string {
	return "Probe.online"
}

func (a *ProbeOnline) Timestamp() time.Time {
	return a.Ts
}

func (a *ProbeOnline) Body() ([]byte, error) {
	return json.Marshal(a.Payload)
}

type ProbeOffline struct {
	Ts      time.Time
	Payload *m.ProbeDTO
}

func (a *ProbeOffline) Type() string {
	return "Probe.offline"
}

func (a *ProbeOffline) Timestamp() time.Time {
	return a.Ts
}

func (a *ProbeOffline) Body() ([]byte, error) {
	return json.Marshal(a.Payload)
}

type ProbeSessionCreated struct {
	Ts      time.Time
	Payload *m.ProbeSession
}

func (a *ProbeSessionCreated) Type() string {
	return "ProbeSession.created"
}

func (a *ProbeSessionCreated) Timestamp() time.Time {
	return a.Ts
}

func (a *ProbeSessionCreated) Body() ([]byte, error) {
	return json.Marshal(a.Payload)
}

type ProbeSessionDeleted struct {
	Ts      time.Time
	Payload *m.ProbeSession
}

func (a *ProbeSessionDeleted) Type() string {
	return "ProbeSession.deleted"
}

func (a *ProbeSessionDeleted) Timestamp() time.Time {
	return a.Ts
}

func (a *ProbeSessionDeleted) Body() ([]byte, error) {
	return json.Marshal(a.Payload)
}

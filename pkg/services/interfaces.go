package services

import (
	"gopkg.in/raintank/schema.v1"
)

type MetricsPublisher interface {
	Add([]*schema.MetricData)
}

type EventsPublisher interface {
	AddEvent(*schema.ProbeEvent)
}

type MetricsEventsPublisher interface {
	MetricsPublisher
	EventsPublisher
}

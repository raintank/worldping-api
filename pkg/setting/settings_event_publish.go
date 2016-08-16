package setting

type EventPublishSettings struct {
	Enabled     bool
	Topic       string
	Broker      string
	Compression string
}

func readEventPublishSettings() {
	sec := Cfg.Section("event_publisher")
	EventPublish.Enabled = sec.Key("enabled").MustBool(false)
	EventPublish.Topic = sec.Key("topic").MustString("events")
	EventPublish.Broker = sec.Key("broker").MustString("localhost:9092")
	EventPublish.Compression = sec.Key("compression").MustString("none")
}

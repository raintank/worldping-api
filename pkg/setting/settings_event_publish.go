package setting

type EventPublishSettings struct {
	Enabled     bool
	Topic       string
	Broker      string
	Compression string
}

func readEventPublishSettings() {
	sec := Cfg.Section("metric_publisher")
	MetricPublish.Enabled = sec.Key("enabled").MustBool(false)
	MetricPublish.Topic = sec.Key("topic").MustString("events")
	MetricPublish.Broker = sec.Key("broker").MustString("localhost:9092")
	MetricPublish.Compression = sec.Key("compression").MustString("none")
}

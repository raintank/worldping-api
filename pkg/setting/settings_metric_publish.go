package setting

type MetricPublishSettings struct {
	Enabled     bool
	Topic       string
	Broker      string
	Compression string
}

func readMetricPublishSettings() {
	sec := Cfg.Section("metric_publisher")
	MetricPublish.Enabled = sec.Key("enabled").MustBool(false)
	MetricPublish.Topic = sec.Key("topic").MustString("mdm")
	MetricPublish.Broker = sec.Key("broker").MustString("localhost:9092")
	MetricPublish.Compression = sec.Key("compression").MustString("none")
}

package setting

type RabbitmqSettings struct {
	Enabled        bool
	Url            string
	EventsExchange string
	AlertsExchange string
}

func readRabbitmqSettings() {
	sec := Cfg.Section("rabbitmq")
	Rabbitmq.Enabled = sec.Key("enabled").MustBool(false)
	Rabbitmq.Url = sec.Key("url").String()
	Rabbitmq.EventsExchange = sec.Key("events_exchange").String()
	Rabbitmq.AlertsExchange = sec.Key("alerts_exchange").String()
}

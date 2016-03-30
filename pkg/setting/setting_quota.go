package setting

import (
	"reflect"
)

type OrgQuota struct {
	Endpoint  int64 `target:"endpoint"`
	Collector int64 `target:"collector"`
}

type GlobalQuota struct {
	Endpoint  int64 `target:"endpoint"`
	Collector int64 `target:"collector"`
}

func (q *OrgQuota) ToMap() map[string]int64 {
	return quotaToMap(*q)
}

func (q *GlobalQuota) ToMap() map[string]int64 {
	return quotaToMap(*q)
}

func quotaToMap(q interface{}) map[string]int64 {
	qMap := make(map[string]int64)
	typ := reflect.TypeOf(q)
	val := reflect.ValueOf(q)

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := field.Tag.Get("target")
		if name == "" {
			name = field.Name
		}
		if name == "-" {
			continue
		}
		value := val.Field(i)
		qMap[name] = value.Int()
	}
	return qMap
}

type QuotaSettings struct {
	Enabled bool
	Org     *OrgQuota
	Global  *GlobalQuota
}

func readQuotaSettings() {
	// set global defaults.
	quota := Cfg.Section("quota")
	Quota.Enabled = quota.Key("enabled").MustBool(false)

	// per ORG Limits
	Quota.Org = &OrgQuota{
		Endpoint:  quota.Key("org_endpoint").MustInt64(10),
		Collector: quota.Key("org_collector").MustInt64(10),
	}

	// Global Limits
	Quota.Global = &GlobalQuota{
		Endpoint:  quota.Key("global_endpoint").MustInt64(10),
		Collector: quota.Key("global_collector").MustInt64(10),
	}

}

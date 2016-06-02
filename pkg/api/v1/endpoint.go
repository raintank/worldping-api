package api

import (
	"fmt"
	"time"

	"github.com/grafana/grafana/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/endpointdiscovery"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
)

func GetEndpointById(c *middleware.Context) {
	id := c.ParamsInt64(":id")

	endpoint, err := sqlstore.GetEndpointById(c.OrgId, id)
	if err != nil {
		c.JSON(404, "Endpoint not found")
		return
	}

	c.JSON(200, endpoint)
	return
}

func GetEndpoints(c *middleware.Context, query m.GetEndpointsQuery) {
	query.OrgId = c.OrgId

	endpoints, err := sqlstore.GetEndpoints(&query)
	if err != nil {
		c.JSON(500, fmt.Sprintf("Failed to query endpoints. %s", err))
		return
	}
	c.JSON(200, endpoints)
}

func DeleteEndpoint(c *middleware.Context) {
	id := c.ParamsInt64(":id")

	err := sqlstore.DeleteEndpoint(c.OrgId, id)
	if err != nil {
		c.JSON(500, fmt.Sprintf("Failed to delete endpoint. %s", err))
		return
	}

	c.JSON(200, "endpoint deleted")
	return
}

func AddEndpoint(c *middleware.Context, cmd m.AddEndpointCommand) {
	cmd.OrgId = c.OrgId
	if cmd.Name == "" {
		c.JSON(400, fmt.Sprintf("Endpoint name not set.", nil))
		return
	}
	checks := make([]m.Check, len(cmd.Monitors))
	for i, mon := range cmd.Monitors {
		checks[i] = m.Check{
			OrgId:          c.OrgId,
			EndpointId:     0,
			Type:           m.MonitorTypeToCheckTypeMap[mon.MonitorTypeId-1],
			Frequency:      mon.Frequency,
			Enabled:        mon.Enabled,
			HealthSettings: mon.HealthSettings,
			Route:          &m.CheckRoute{},
			Settings:       m.MonitorSettingsDTO(mon.Settings).ToV2Setting(m.MonitorTypeToCheckTypeMap[mon.MonitorTypeId-1]),
		}
		if len(mon.CollectorTags) > 0 {
			checks[i].Route.Type = m.RouteByTags
			checks[i].Route.Config = map[string]interface{}{"tags": mon.CollectorTags}
		} else {
			checks[i].Route.Type = m.RouteByIds
			checks[i].Route.Config = map[string]interface{}{"ids": mon.CollectorIds}
		}
	}
	endpoint := m.EndpointDTO{
		OrgId:   cmd.OrgId,
		Name:    cmd.Name,
		Tags:    cmd.Tags,
		Created: time.Now(),
		Updated: time.Now(),
		Checks:  checks,
	}
	err := sqlstore.AddEndpoint(&endpoint)
	if err != nil {
		log.Error(3, "failed to add Endpoint. %s", err)
		c.JSON(500, fmt.Sprintf("failed to add endpoint. %s", err))
		return
	}

	c.JSON(200, endpoint)
}

func UpdateEndpoint(c *middleware.Context, cmd m.UpdateEndpointCommand) {
	cmd.OrgId = c.OrgId
	if cmd.Name == "" {
		c.JSON(400, fmt.Sprintf("Endpoint name not set.", nil))
		return
	}
	// get existing endpoint.
	endpoint, err := sqlstore.GetEndpointById(cmd.OrgId, cmd.Id)
	if err != nil {
		c.JSON(500, fmt.Sprintf("Failed to lookup existing endpoint. %s", err))
		return
	}
	if endpoint == nil {
		c.JSON(404, "Endpoint not found")
		return
	}

	endpoint.Name = cmd.Name
	endpoint.Tags = cmd.Tags

	err = sqlstore.UpdateEndpoint(endpoint)
	if err != nil {
		c.JSON(500, fmt.Sprintf("failed to update endpoint. %s", err))
		return
	}

	c.JSON(200, "Endpoint updated")
}

func DiscoverEndpoint(c *middleware.Context, cmd m.DiscoverEndpointCmd) {
	checks, err := endpointdiscovery.Discover(cmd.Name)
	if err != nil {
		c.JSON(500, fmt.Sprintf("Failed to discover endpoint. %s", err))
		return
	}
	// convert from checks to v1api SuggestedMonitor
	monitors := make([]m.SuggestedMonitor, len(checks))
	for i, check := range checks {
		monitors[i] = m.SuggestedMonitor{
			MonitorTypeId: checkTypeToId(check.Type),
			Settings:      checkSettingToMonitorSetting(check.Settings),
		}
	}
	c.JSON(200, monitors)
}

func checkTypeToId(t m.CheckType) int64 {
	lookup := map[m.CheckType]int64{
		m.HTTP_CHECK:  1,
		m.HTTPS_CHECK: 2,
		m.PING_CHECK:  3,
		m.DNS_CHECK:   4,
	}
	typeNum, exists := lookup[t]
	if !exists {
		return 0
	}
	return typeNum
}

func checkSettingToMonitorSetting(settings map[string]interface{}) []m.MonitorSettingDTO {
	monSetting := make([]m.MonitorSettingDTO, 0)
	for key, val := range settings {
		monSetting = append(monSetting, m.MonitorSettingDTO{
			Variable: key,
			Value:    fmt.Sprintf("%v", val),
		})
	}
	return monSetting
}

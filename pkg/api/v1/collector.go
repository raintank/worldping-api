package v1

import (
	"fmt"

	//"github.com/grafana/grafana/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
)

func GetCollectors(c *middleware.Context, query m.GetProbesQuery) {
	query.OrgId = c.OrgId
	probes, err := sqlstore.GetProbes(&query)
	if err != nil {
		c.JSON(500, fmt.Sprintf("Failed to query collectors. %s", err))
		return
	}
	c.JSON(200, probes)
	return
}

func GetCollectorLocations(c *middleware.Context) {
	query := m.GetProbesQuery{
		OrgId: c.OrgId,
	}

	probes, err := sqlstore.GetProbes(&query)
	if err != nil {
		c.JSON(500, fmt.Sprintf("Failed to query collectors. %s", err))
		return
	}

	locations := make([]m.ProbeLocationDTO, len(probes))
	for i, c := range probes {
		locations[i] = m.ProbeLocationDTO{
			Key:       c.Slug,
			Latitude:  c.Latitude,
			Longitude: c.Longitude,
			Name:      c.Name,
		}
	}

	c.JSON(200, locations)
	return
}

func GetCollectorById(c *middleware.Context) {
	id := c.ParamsInt64(":id")

	probe, err := sqlstore.GetProbeById(id, c.OrgId)
	if err != nil {
		c.JSON(404, "Collector not found")
		return
	}

	c.JSON(200, probe)
	return
}

func DeleteCollector(c *middleware.Context) {
	id := c.ParamsInt64(":id")

	err := sqlstore.DeleteProbe(id, c.OrgId)
	if err != nil {
		c.JSON(500, fmt.Sprintf("Failed to delete collector. %s", err))
		return
	}

	c.JSON(200, "collector deleted")
	return
}

func AddCollector(c *middleware.Context, probe m.ProbeDTO) {
	probe.OrgId = c.OrgId
	if probe.Id != 0 {
		c.JSON(400, "Id already set. Try update instead of create.")
		return
	}
	if probe.Name == "" {
		c.JSON(400, "Collector Name not set.")
		return
	}

	if err := sqlstore.AddProbe(&probe); err != nil {
		c.JSON(500, fmt.Sprintf("Failed to add collector", err))
		return
	}

	c.JSON(200, probe)
	return
}

func UpdateCollector(c *middleware.Context, probe m.ProbeDTO) {
	probe.OrgId = c.OrgId
	if probe.Name == "" {
		c.JSON(400, "Collector Name not set.")
		return
	}

	if probe.Public {
		if !c.IsAdmin {
			c.JSON(400, "Only admins can make public collectors")
			return
		}
	}

	if err := sqlstore.UpdateProbe(&probe); err != nil {
		c.JSON(500, fmt.Sprintf("Failed to add collector", err))
		return
	}

	c.JSON(200, probe)
	return
}

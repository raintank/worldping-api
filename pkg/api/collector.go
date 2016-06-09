package api

import (
	//"github.com/raintank/worldping-api/pkg/log"
	"github.com/raintank/worldping-api/pkg/middleware"
	m "github.com/raintank/worldping-api/pkg/models"
	"github.com/raintank/worldping-api/pkg/services/sqlstore"
)

func V1GetCollectors(c *middleware.Context, query m.GetProbesQuery) {
	query.OrgId = c.OrgId
	probes, err := sqlstore.GetProbes(&query)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(200, probes)
	return
}

func V1GetCollectorLocations(c *middleware.Context) {
	query := m.GetProbesQuery{
		OrgId: c.OrgId,
	}

	probes, err := sqlstore.GetProbes(&query)
	if err != nil {
		handleError(c, err)
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

func V1GetCollectorById(c *middleware.Context) {
	id := c.ParamsInt64(":id")

	probe, err := sqlstore.GetProbeById(id, c.OrgId)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(200, probe)
	return
}

func V1DeleteCollector(c *middleware.Context) {
	id := c.ParamsInt64(":id")

	err := sqlstore.DeleteProbe(id, c.OrgId)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(200, "collector deleted")
	return
}

func V1AddCollector(c *middleware.Context, probe m.ProbeDTO) {
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
		handleError(c, err)
		return
	}

	c.JSON(200, probe)
	return
}

func V1UpdateCollector(c *middleware.Context, probe m.ProbeDTO) {
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
		handleError(c, err)
		return
	}

	c.JSON(200, probe)
	return
}

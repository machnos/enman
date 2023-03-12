package persistency

import (
	"enman/internal/energysource"
	"fmt"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"time"
)

type influxRepository struct {
	Repository
	client    influxdb2.Client
	writeApis map[string]api.WriteAPI
}

func NewInfluxRepository(serverUrl string, token string) Repository {
	repo := &influxRepository{
		client:    influxdb2.NewClient(serverUrl, token),
		writeApis: make(map[string]api.WriteAPI),
	}
	repo.writeApis["grid"] = repo.client.WriteAPI("machnos-enman", "meters")
	repo.writeApis["pv"] = repo.client.WriteAPI("machnos-enman", "meters")
	return repo
}

func (i *influxRepository) StoreGridValues(grid energysource.Grid) {
	fields := map[string]interface{}{
		"total_current":         grid.TotalCurrent(),
		"total_power":           grid.TotalPower(),
		"total_energy_consumed": grid.TotalEnergyConsumed(),
		"total_energy_provided": grid.TotalEnergyProvided(),
	}
	for ix := uint8(0); ix < grid.Phases(); ix++ {
		fields[fmt.Sprintf("l%d_current", ix+1)] = grid.Current(ix)
		fields[fmt.Sprintf("l%d_power", ix+1)] = grid.Power(ix)
		fields[fmt.Sprintf("l%d_voltage", ix+1)] = grid.Voltage(ix)
		if grid.EnergyConsumed(ix) != 0 {
			fields[fmt.Sprintf("l%d_energy_consumed", ix+1)] = grid.EnergyConsumed(ix)
		}
		if grid.EnergyProvided(ix) != 0 {
			fields[fmt.Sprintf("l%d_energy_provided", ix+1)] = grid.EnergyProvided(ix)
		}
	}
	tags := map[string]string{
		"role": "grid",
	}
	if grid.Name() != "" {
		tags["name"] = grid.Name()
	}
	point := influxdb2.NewPoint(
		"energy_usage",
		tags,
		fields,
		time.Now())

	i.writeApis["grid"].WritePoint(point)
}

func (i *influxRepository) StorePvValues(pv energysource.Pv) {
	fields := map[string]interface{}{
		"total_current":         pv.TotalCurrent(),
		"total_power":           pv.TotalPower(),
		"total_energy_consumed": pv.TotalEnergyConsumed(),
		"total_energy_provided": pv.TotalEnergyProvided(),
	}
	for ix := uint8(0); ix < pv.Phases(); ix++ {
		fields[fmt.Sprintf("l%d_current", ix+1)] = pv.Current(ix)
		fields[fmt.Sprintf("l%d_power", ix+1)] = pv.Power(ix)
		fields[fmt.Sprintf("l%d_voltage", ix+1)] = pv.Voltage(ix)
		if pv.EnergyConsumed(ix) != 0 {
			fields[fmt.Sprintf("l%d_energy_consumed", ix+1)] = pv.EnergyConsumed(ix)
		}
		if pv.EnergyProvided(ix) != 0 {
			fields[fmt.Sprintf("l%d_energy_provided", ix+1)] = pv.EnergyProvided(ix)
		}
	}
	tags := map[string]string{
		"role": "pv",
	}
	if pv.Name() != "" {
		tags["name"] = pv.Name()
	}
	point := influxdb2.NewPoint(
		"energy_usage",
		tags,
		fields,
		time.Now())

	i.writeApis["pv"].WritePoint(point)
}

func (i *influxRepository) Close() {
	for _, value := range i.writeApis {
		value.Flush()
	}
	i.client.Close()
}

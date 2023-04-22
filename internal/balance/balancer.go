package balance

import (
	"enman/internal"
	"enman/internal/log"
	"enman/internal/persistency"
)

func StartUpdateLoop(updateChannels *internal.UpdateChannels, repository persistency.Repository) {
	log.Info("Starting balancer update loop...")
	go startGridUpdateLoop(updateChannels, repository)
	go startPvUpdateLoop(updateChannels, repository)
	log.Info("Balancer update loop started")
}

func startGridUpdateLoop(updateChannels *internal.UpdateChannels, repository persistency.Repository) {
	for {
		grid := <-updateChannels.GridUpdated()
		repository.StoreEnergyFlow(grid)
	}
}

func startPvUpdateLoop(updateChannels *internal.UpdateChannels, repository persistency.Repository) {
	for {
		pv := <-updateChannels.PvUpdated()
		repository.StoreEnergyFlow(pv)
	}
}

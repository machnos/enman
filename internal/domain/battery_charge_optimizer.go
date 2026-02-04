package domain

import (
	"context"
	"enman/internal/domain/battery"
	"enman/internal/domain/events"
	"enman/internal/domain/prices"
	"enman/internal/domain/repository"
	"enman/internal/log"
	"time"
)

type BatteryChargeOptimizer struct {
	priceRepo           repository.EnergyPrice
	providerName        string
	system              *System
	roundTripEfficiency float32
	minSoC              float32
	maxSoC              float32
	cancel              context.CancelFunc
}

func NewBatteryChargeOptimizer(priceRepo repository.EnergyPrice, providerName string, system *System) *BatteryChargeOptimizer {
	return &BatteryChargeOptimizer{
		priceRepo:           priceRepo,
		providerName:        providerName,
		system:              system,
		roundTripEfficiency: 0.9, // 90% round-trip efficiency
		minSoC:              15,  // 15% minimum SoC
		maxSoC:              100, // 100% maximum SoC
	}
}

// Start begins the optimization process that runs every 5 minutes
// Returns when context is cancelled (for errgroup integration)
func (b *BatteryChargeOptimizer) Start(ctx context.Context) {
	ctx, b.cancel = context.WithCancel(ctx)

	log.Info("BatteryChargeOptimizer started")

	// Run optimization immediately on start
	b.runOptimization()

	// Start a goroutine to run optimization periodically
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.runOptimization()
			}
		}
	}()
}

// Stop stops the optimization process
func (b *BatteryChargeOptimizer) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	log.Info("BatteryChargeOptimizer stopped")
}

// runOptimization performs the optimization and publishes schedule slots
func (b *BatteryChargeOptimizer) runOptimization() {
	if b.priceRepo == nil {
		log.Warning("BatteryChargeOptimizer: no price repository configured, skipping optimization")
		return
	}

	now := time.Now()
	from := now.Truncate(time.Hour)
	till := from.Add(24 * time.Hour)

	// Retrieve energy prices for the next 24 hours
	energyPrices, err := b.priceRepo.EnergyPrices(from, till, b.providerName, prices.EnergyTypeElectricity)
	if err != nil {
		log.Errorf("BatteryChargeOptimizer: failed to retrieve energy prices: %v", err)
		return
	}

	if len(energyPrices) == 0 {
		log.Warning("BatteryChargeOptimizer: no energy prices available for optimization")
		return
	}

	// Calculate optimal charge slots
	slots := b.calculateOptimalSlots(energyPrices)

	// Publish schedule slots via events
	for _, slot := range slots {
		event := events.NewBatteryScheduleSlotEvent(slot)
		events.BatteryScheduleSlots.Trigger(event)
	}

	if log.DebugEnabled() {
		log.Debugf("BatteryChargeOptimizer: published %d schedule slots", len(slots))
	}
}

// calculateOptimalSlots determines the best charging slots based on prices
func (b *BatteryChargeOptimizer) calculateOptimalSlots(energyPrices []*prices.EnergyPrice) []*battery.ScheduleSlot {
	if len(energyPrices) == 0 {
		return nil
	}

	// Get battery info from system
	batteries := b.system.Batteries()
	if len(batteries) == 0 {
		log.Warning("BatteryChargeOptimizer: no batteries configured in system")
		return nil
	}

	// Aggregate battery properties across all batteries
	var batteryBanksTotalStorageCapacityInKwh float32
	var batteryBanksMaxChargePower float32
	var batteryBanksAvailableStorageCapacityInKwh float32
	var batteryBanksUsedStorageCapacityInKwh float32

	for _, bat := range batteries {
		capacityKwh := bat.AvailableCapacity() * bat.Voltage() / 1000
		batteryBanksTotalStorageCapacityInKwh += capacityKwh
		batteryBanksMaxChargePower += bat.MaxChargePower()
		batteryBanksUsedStorageCapacityInKwh += bat.State().SoC() * capacityKwh
		batteryBanksAvailableStorageCapacityInKwh += capacityKwh - (bat.State().SoC() * capacityKwh)
	}

	return nil
}

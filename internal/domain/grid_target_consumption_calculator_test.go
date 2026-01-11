package domain

import (
	"context"
	"enman/internal/domain/constants"
	"enman/internal/domain/electricity"
	"enman/internal/domain/events"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func Test_GridTargetConsumptionCalculation(t *testing.T) {
	system := NewSystem(time.Now().Location())
	targetConsumption := 50
	percentageFromGrid := uint8(rand.Int31n(100))
	mockController := &MockGridController{}
	system.SetGrid("Grid", 230, 25, 3, targetConsumption, nil, mockController)
	system.AddAcLoad("EvCharger1", constants.EnergySourceRoleEvCharger, percentageFromGrid, nil)
	calculator, err := NewGridTargetConsumptionCalculator(system, nil, 90.0) // 90% round-trip efficiency
	if err != nil {
		t.Error(err)
	}

	// Create context for the calculator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start calculator in a goroutine
	go func() {
		_ = calculator.Start(ctx)
	}()

	// Give calculator time to start
	time.Sleep(100 * time.Millisecond)

	defer calculator.Stop()

	totalPower := 0
	loops := 10
	mockController.wg.Add(1)
	for i := 0; i < loops; i++ {
		power := min(int(rand.Int31n(math.MaxInt16)), int(system.Grid().MaxElectricityConsumption()))
		totalPower += power
		events.ElectricityMeterReadings.Trigger(events.NewElectricityMeterValues().
			SetName("EvCharger1").
			SetRole(constants.EnergySourceRoleEvCharger).SetState(electricity.NewState().SetPower(0, float32(power))))
	}
	mockController.wg.Wait()
	// Add 1 to the WaitGroup because the 'defer calculator.Stop() will also call the MockController'
	// This would result in a panic: sync: negative WaitGroup counter
	mockController.wg.Add(1)
	avg := totalPower / loops
	expected := (avg * int(percentageFromGrid) / 100) + targetConsumption
	if mockController.targetConsumption != expected {
		t.Errorf("target consumption expected = %d, got %d", expected, mockController.targetConsumption)
	}
}

type MockGridController struct {
	targetConsumption int
	wg                sync.WaitGroup
}

func (m *MockGridController) SetElectricityTargetConsumption(targetConsumption int) error {
	defer m.wg.Done()
	m.targetConsumption = targetConsumption
	return nil
}

func (m *MockGridController) GridConnected() (bool, error) {
	return true, nil
}

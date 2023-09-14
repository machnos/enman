package domain

import (
	"testing"
	"time"
)

func Test_ElectricityMeterReadingsEventsWithoutFilter(t *testing.T) {
	listener := &MockEnergyMeterListener{}
	ElectricityMeterReadings.Register(listener, nil)
	// Fire an event
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues())
	// Events are fired in separate go routines. Give the system some time to execute them.
	time.Sleep(time.Millisecond * 10)
	if listener.eventsReceived != 1 {
		t.Errorf("triggered events expected = %d, got %d", 1, listener.eventsReceived)
	}
}

func Test_ElectricityMeterReadingsEventsWithFilter(t *testing.T) {
	listener := &MockEnergyMeterListener{}
	ElectricityMeterReadings.Register(listener, func(values *ElectricityMeterValues) bool {
		return values.MeterBrand() == "ABB"
	})
	// Fire an event without a meter brand
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues())
	// Fire an event with the correct meter brand
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues().SetMeterBrand("ABB"))
	// Fire an event with a wrong meter brand
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues().SetMeterBrand("Carlo Gavazzi"))
	// Events are fired in separate go routines. Give the system some time to execute them.
	time.Sleep(time.Millisecond * 10)
	if listener.eventsReceived != 1 {
		t.Errorf("triggered events expected = %d, got %d", 1, listener.eventsReceived)
	}
}

func Test_DeregisterEventHandler(t *testing.T) {
	listener := &MockEnergyMeterListener{}
	ElectricityMeterReadings.Register(listener, nil)
	// Fire an event
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues())
	// Events are fired in separate go routines. Give the system some time to execute them.
	time.Sleep(time.Millisecond * 10)
	if listener.eventsReceived != 1 {
		t.Errorf("triggered events expected = %d, got %d", 1, listener.eventsReceived)
	}

	ElectricityMeterReadings.Deregister(listener)
	// Fire an event
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues())
	// Events are fired in separate go routines. Give the system some time to execute them.
	time.Sleep(time.Millisecond * 10)
	if listener.eventsReceived != 1 {
		t.Errorf("triggered events expected = %d, got %d", 1, listener.eventsReceived)
	}
}

type MockEnergyMeterListener struct {
	eventsReceived uint8
}

func (m *MockEnergyMeterListener) HandleEvent(*ElectricityMeterValues) {
	m.eventsReceived++
}

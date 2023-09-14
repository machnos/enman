package domain

import (
	"reflect"
	"testing"
	"time"
)

func Test_SetGrid(t *testing.T) {
	// Create a new system.
	system := NewSystem(time.Now().Location())
	gridName := "TestGrid"
	system.SetGrid(gridName, 230, 25, 3)

	currentUsage := system.Grid().ElectricityUsage()
	currentState := system.Grid().ElectricityState()

	state := NewElectricityState().
		SetCurrent(0, 1).
		SetCurrent(1, 2).
		SetCurrent(2, 3).
		SetPower(0, 100).
		SetPower(1, 200).
		SetPower(2, 300).
		SetVoltage(0, 229).
		SetVoltage(1, 230).
		SetVoltage(2, 231)
	usage := NewElectricityUsage().
		SetEnergyConsumed(0, 1000).
		SetEnergyConsumed(1, 1001).
		SetEnergyConsumed(2, 1002).
		SetEnergyProvided(2, 500).
		SetEnergyProvided(2, 501).
		SetEnergyProvided(2, 502)

	// Trigger an event with the wrong grid name
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues().
		SetName("WrongGridName").
		SetRole(RoleGrid).
		SetElectricityState(state).
		SetElectricityUsage(usage))

	// Events are fired in separate go routines. Give the system some time to execute them.
	time.Sleep(time.Millisecond * 10)
	if !reflect.DeepEqual(currentState, system.Grid().ElectricityState()) {
		t.Errorf("Electricity state picked up from meter event when it has another grid name")
	}
	if !reflect.DeepEqual(currentUsage, system.Grid().ElectricityUsage()) {
		t.Errorf("Electricity usage picked up from meter event when it has another grid name")
	}

	// Trigger an event with the wrong role
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues().
		SetName(gridName).
		SetRole(RolePv).
		SetElectricityState(state).
		SetElectricityUsage(usage))

	// Events are fired in separate go routines. Give the system some time to execute them.
	time.Sleep(time.Millisecond * 10)
	if !reflect.DeepEqual(currentState, system.Grid().ElectricityState()) {
		t.Errorf("Electricity state picked up from meter event when it has another grid name")
	}
	if !reflect.DeepEqual(currentUsage, system.Grid().ElectricityUsage()) {
		t.Errorf("Electricity usage picked up from meter event when it has another grid name")
	}

	// Trigger an event from an electricity meter
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues().
		SetName(gridName).
		SetRole(RoleGrid).
		SetElectricityState(state).
		SetElectricityUsage(usage))

	// Events are fired in separate go routines. Give the system some time to execute them.
	time.Sleep(time.Millisecond * 10)
	if !reflect.DeepEqual(state, system.Grid().ElectricityState()) {
		t.Errorf("Electricity state not picked up from meter event")
	}
	if !reflect.DeepEqual(usage, system.grid.ElectricityUsage()) {
		t.Errorf("Electricity usage not picked up from meter event")
	}
}

func Test_AddPv(t *testing.T) {
	// Create a new system.
	system := NewSystem(time.Now().Location())
	pvName1 := "TestPv1"
	pvName2 := "TestPv2"
	system.AddPv(pvName1).AddPv(pvName2)

	currentUsagePv1 := system.Pvs()[0].ElectricityUsage()
	currentStatePv1 := system.Pvs()[0].ElectricityState()
	currentUsagePv2 := system.Pvs()[1].ElectricityUsage()
	currentStatePv2 := system.Pvs()[1].ElectricityState()

	state := NewElectricityState().
		SetCurrent(0, 1).
		SetCurrent(1, 2).
		SetCurrent(2, 3).
		SetPower(0, 100).
		SetPower(1, 200).
		SetPower(2, 300).
		SetVoltage(0, 229).
		SetVoltage(1, 230).
		SetVoltage(2, 231)
	usage := NewElectricityUsage().
		SetEnergyConsumed(0, 1000).
		SetEnergyConsumed(1, 1001).
		SetEnergyConsumed(2, 1002).
		SetEnergyProvided(2, 500).
		SetEnergyProvided(2, 501).
		SetEnergyProvided(2, 502)

	// Trigger an event with the wrong role
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues().
		SetName(pvName1).
		SetRole(RoleGrid).
		SetElectricityState(state).
		SetElectricityUsage(usage))

	// Events are fired in separate go routines. Give the system some time to execute them.
	time.Sleep(time.Millisecond * 10)
	if !reflect.DeepEqual(currentStatePv1, system.Pvs()[0].ElectricityState()) {
		t.Errorf("Electricity state picked up from meter event when it has another pv name")
	}
	if !reflect.DeepEqual(currentUsagePv1, system.Pvs()[0].ElectricityUsage()) {
		t.Errorf("Electricity usage picked up from meter event when it has another pv name")
	}

	// Trigger an event from an electricity meter
	ElectricityMeterReadings.Trigger(NewElectricityMeterValues().
		SetName(pvName1).
		SetRole(RolePv).
		SetElectricityState(state).
		SetElectricityUsage(usage))

	// Events are fired in separate go routines. Give the system some time to execute them.
	time.Sleep(time.Millisecond * 10)
	if !reflect.DeepEqual(state, system.Pvs()[0].ElectricityState()) {
		t.Errorf("Electricity state not picked up from meter event")
	}
	if !reflect.DeepEqual(usage, system.Pvs()[0].ElectricityUsage()) {
		t.Errorf("Electricity usage not picked up from meter event")
	}
	// Make sure pv2 isn't affected
	if !reflect.DeepEqual(currentStatePv2, system.Pvs()[1].ElectricityState()) {
		t.Errorf("Electricity state picked up from meter event when it has another pv name")
	}
	if !reflect.DeepEqual(currentUsagePv2, system.Pvs()[1].ElectricityUsage()) {
		t.Errorf("Electricity usage picked up from meter event when it has another pv name")
	}
}

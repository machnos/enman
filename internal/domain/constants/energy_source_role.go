package constants

type EnergySourceRole string

const (
	EnergySourceRoleUndefined EnergySourceRole = ""
	EnergySourceRoleGrid      EnergySourceRole = "Grid"
	EnergySourceRolePv        EnergySourceRole = "Pv"
	EnergySourceRoleBattery   EnergySourceRole = "Battery"
	EnergySourceRoleEvCharger EnergySourceRole = "EvCharger"
)

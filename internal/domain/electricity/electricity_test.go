package electricity

import (
	"reflect"
	"testing"
)

func TestElectricityState_Current(t *testing.T) {
	type fields struct {
		current [MaxPhases]float32
	}
	type args struct {
		lineIx uint8
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   float32
	}{
		{"Test get current phase 1", fields{current: [3]float32{1, 2, 3}}, args{lineIx: 0}, float32(1)},
		{"Test get current phase 2", fields{current: [3]float32{1, 2, 3}}, args{lineIx: 1}, float32(2)},
		{"Test get current phase 3", fields{current: [3]float32{1, 2, 3}}, args{lineIx: 2}, float32(3)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &State{
				current: tt.fields.current,
			}
			if got := es.Current(tt.args.lineIx); got != tt.want {
				t.Errorf("Current() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityState_Power(t *testing.T) {
	type fields struct {
		power [MaxPhases]float32
	}
	type args struct {
		lineIx uint8
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   float32
	}{
		{"Test get power phase 1", fields{power: [3]float32{1, 2, 3}}, args{lineIx: 0}, float32(1)},
		{"Test get power phase 2", fields{power: [3]float32{1, 2, 3}}, args{lineIx: 1}, float32(2)},
		{"Test get power phase 3", fields{power: [3]float32{1, 2, 3}}, args{lineIx: 2}, float32(3)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &State{
				power: tt.fields.power,
			}
			if got := es.Power(tt.args.lineIx); got != tt.want {
				t.Errorf("Power() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestState_SetCurrent(t *testing.T) {
	type fields struct {
		current [MaxPhases]float32
	}
	type args struct {
		lineIx  uint8
		current float32
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *State
	}{
		{"Test set current phase 1", fields{current: [3]float32{}}, args{lineIx: 0, current: 16}, NewState().SetCurrent(0, 16)},
		{"Test set current phase 2", fields{current: [3]float32{}}, args{lineIx: 1, current: 32}, NewState().SetCurrent(1, 32)},
		{"Test set current phase 3", fields{current: [3]float32{}}, args{lineIx: 2, current: 64}, NewState().SetCurrent(2, 64)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &State{
				current: tt.fields.current,
			}
			if got := es.SetCurrent(tt.args.lineIx, tt.args.current); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetCurrent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityState_SetPower(t *testing.T) {
	type fields struct {
		power [MaxPhases]float32
	}
	type args struct {
		lineIx uint8
		power  float32
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *State
	}{
		{"Test set power phase 1", fields{power: [3]float32{}}, args{lineIx: 0, power: 1600}, NewState().SetPower(0, 1600)},
		{"Test set power phase 2", fields{power: [3]float32{}}, args{lineIx: 1, power: 1200}, NewState().SetPower(1, 1200)},
		{"Test set power phase 3", fields{power: [3]float32{}}, args{lineIx: 2, power: 5000}, NewState().SetPower(2, 5000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &State{
				power: tt.fields.power,
			}
			if got := es.SetPower(tt.args.lineIx, tt.args.power); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetPower() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityState_SetVoltage(t *testing.T) {
	type fields struct {
		voltage [MaxPhases]float32
	}
	type args struct {
		lineIx  uint8
		voltage float32
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *State
	}{
		{"Test set voltage phase 1", fields{voltage: [3]float32{}}, args{lineIx: 0, voltage: 110}, NewState().SetVoltage(0, 110)},
		{"Test set voltage phase 2", fields{voltage: [3]float32{}}, args{lineIx: 1, voltage: 230}, NewState().SetVoltage(1, 230)},
		{"Test set voltage phase 3", fields{voltage: [3]float32{}}, args{lineIx: 2, voltage: 400}, NewState().SetVoltage(2, 400)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &State{
				voltage: tt.fields.voltage,
			}
			if got := es.SetVoltage(tt.args.lineIx, tt.args.voltage); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetVoltage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityState_TotalCurrent(t *testing.T) {
	type fields struct {
		current [MaxPhases]float32
	}
	tests := []struct {
		name   string
		fields fields
		want   float32
	}{
		{"Test get total current", fields{current: [3]float32{1, 2, 3}}, float32(6)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &State{
				current: tt.fields.current,
			}
			if got := es.TotalCurrent(); got != tt.want {
				t.Errorf("TotalCurrent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityState_TotalPower(t *testing.T) {
	type fields struct {
		power [MaxPhases]float32
	}
	tests := []struct {
		name   string
		fields fields
		want   float32
	}{
		{"Test get total power", fields{power: [3]float32{1, 2, 3}}, float32(6)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &State{
				power: tt.fields.power,
			}
			if got := es.TotalPower(); got != tt.want {
				t.Errorf("TotalPower() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityState_Valid(t *testing.T) {
	type fields struct {
		power   [MaxPhases]float32
		voltage [MaxPhases]float32
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{
		{"Test valid power", fields{power: [3]float32{10, -10, 10}}, true, false},
		{"Test valid voltage", fields{voltage: [3]float32{110, 230, 400}}, true, false},
		{"Test invalid voltage", fields{voltage: [3]float32{-110, -230, -400}}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &State{
				power:   tt.fields.power,
				voltage: tt.fields.voltage,
			}
			got, err := es.Valid()
			if (err != nil) != tt.wantErr {
				t.Errorf("Valid() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Valid() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityState_Voltage(t *testing.T) {
	type fields struct {
		voltage [MaxPhases]float32
	}
	type args struct {
		lineIx uint8
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   float32
	}{
		{"Test get voltage phase 1", fields{voltage: [3]float32{110, 230, 400}}, args{lineIx: 0}, float32(110)},
		{"Test get voltage phase 2", fields{voltage: [3]float32{110, 230, 400}}, args{lineIx: 1}, float32(230)},
		{"Test get voltage phase 3", fields{voltage: [3]float32{110, 230, 400}}, args{lineIx: 2}, float32(400)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := &State{
				voltage: tt.fields.voltage,
			}
			if got := es.Voltage(tt.args.lineIx); got != tt.want {
				t.Errorf("Voltage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityUsage_EnergyConsumed(t *testing.T) {
	type fields struct {
		energyConsumed [MaxPhases]float64
	}
	type args struct {
		lineIx uint8
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   float64
	}{
		{"Test get energy consumed phase 1", fields{energyConsumed: [3]float64{1000, 2000, 3000}}, args{lineIx: 0}, float64(1000)},
		{"Test get energy consumed phase 2", fields{energyConsumed: [3]float64{1000, 2000, 3000}}, args{lineIx: 1}, float64(2000)},
		{"Test get energy consumed phase 3", fields{energyConsumed: [3]float64{1000, 2000, 3000}}, args{lineIx: 2}, float64(3000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eu := &Usage{
				energyConsumed: tt.fields.energyConsumed,
			}
			if got := eu.EnergyConsumed(tt.args.lineIx); got != tt.want {
				t.Errorf("EnergyConsumed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityUsage_EnergyProvided(t *testing.T) {
	type fields struct {
		energyProvided [MaxPhases]float64
	}
	type args struct {
		lineIx uint8
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   float64
	}{
		{"Test get energy provided phase 1", fields{energyProvided: [3]float64{1000, 2000, 3000}}, args{lineIx: 0}, float64(1000)},
		{"Test get energy provided phase 2", fields{energyProvided: [3]float64{1000, 2000, 3000}}, args{lineIx: 1}, float64(2000)},
		{"Test get energy provided phase 3", fields{energyProvided: [3]float64{1000, 2000, 3000}}, args{lineIx: 2}, float64(3000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eu := &Usage{
				energyProvided: tt.fields.energyProvided,
			}
			if got := eu.EnergyProvided(tt.args.lineIx); got != tt.want {
				t.Errorf("EnergyProvided() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityUsage_SetEnergyConsumed(t *testing.T) {
	type fields struct {
		energyConsumed [MaxPhases]float64
	}
	type args struct {
		lineIx         uint8
		energyConsumed float64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Usage
	}{
		{"Test set energy consumed phase 1", fields{energyConsumed: [3]float64{}}, args{lineIx: 0, energyConsumed: 1000}, NewUsage().SetEnergyConsumed(0, 1000)},
		{"Test set energy consumed phase 2", fields{energyConsumed: [3]float64{}}, args{lineIx: 1, energyConsumed: 2000}, NewUsage().SetEnergyConsumed(1, 2000)},
		{"Test set energy consumed phase 3", fields{energyConsumed: [3]float64{}}, args{lineIx: 2, energyConsumed: 3000}, NewUsage().SetEnergyConsumed(2, 3000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eu := &Usage{
				energyConsumed: tt.fields.energyConsumed,
			}
			if got := eu.SetEnergyConsumed(tt.args.lineIx, tt.args.energyConsumed); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetEnergyConsumed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityUsage_SetEnergyProvided(t *testing.T) {
	type fields struct {
		energyProvided [MaxPhases]float64
	}
	type args struct {
		lineIx         uint8
		energyProvided float64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Usage
	}{
		{"Test set energy provided phase 1", fields{energyProvided: [3]float64{}}, args{lineIx: 0, energyProvided: 1000}, NewUsage().SetEnergyProvided(0, 1000)},
		{"Test set energy provided phase 2", fields{energyProvided: [3]float64{}}, args{lineIx: 1, energyProvided: 2000}, NewUsage().SetEnergyProvided(1, 2000)},
		{"Test set energy provided phase 3", fields{energyProvided: [3]float64{}}, args{lineIx: 2, energyProvided: 3000}, NewUsage().SetEnergyProvided(2, 3000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eu := &Usage{
				energyProvided: tt.fields.energyProvided,
			}
			if got := eu.SetEnergyProvided(tt.args.lineIx, tt.args.energyProvided); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetEnergyProvided() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityUsage_SetTotalEnergyConsumed(t *testing.T) {
	type fields struct {
		totalEnergyConsumed float64
	}
	type args struct {
		totalEnergyConsumed float64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Usage
	}{
		{"Test set total energy consumed", fields{totalEnergyConsumed: 2500}, args{totalEnergyConsumed: 2500}, NewUsage().SetTotalEnergyConsumed(2500)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eu := &Usage{
				totalEnergyConsumed: tt.fields.totalEnergyConsumed,
			}
			if got := eu.SetTotalEnergyConsumed(tt.args.totalEnergyConsumed); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetTotalEnergyConsumed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityUsage_SetTotalEnergyProvided(t *testing.T) {
	type fields struct {
		totalEnergyProvided float64
	}
	type args struct {
		totalEnergyProvided float64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Usage
	}{
		{"Test set total energy provided", fields{totalEnergyProvided: 2500}, args{totalEnergyProvided: 2500}, NewUsage().SetTotalEnergyProvided(2500)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eu := &Usage{
				totalEnergyProvided: tt.fields.totalEnergyProvided,
			}
			if got := eu.SetTotalEnergyProvided(tt.args.totalEnergyProvided); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SetTotalEnergyProvided() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityUsage_TotalEnergyConsumed(t *testing.T) {
	type fields struct {
		energyConsumed      [MaxPhases]float64
		totalEnergyConsumed float64
	}
	tests := []struct {
		name   string
		fields fields
		want   float64
	}{
		{"Test get total energy consumed with override", fields{energyConsumed: [3]float64{100, 100, 100}, totalEnergyConsumed: 2500}, 2500},
		{"Test get total energy consumed without override", fields{energyConsumed: [3]float64{100, 100, 100}, totalEnergyConsumed: 0}, 300},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eu := &Usage{
				energyConsumed:      tt.fields.energyConsumed,
				totalEnergyConsumed: tt.fields.totalEnergyConsumed,
			}
			if got := eu.TotalEnergyConsumed(); got != tt.want {
				t.Errorf("TotalEnergyConsumed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityUsage_TotalEnergyProvided(t *testing.T) {
	type fields struct {
		energyProvided      [MaxPhases]float64
		totalEnergyProvided float64
	}
	tests := []struct {
		name   string
		fields fields
		want   float64
	}{
		{"Test get total energy provided with override", fields{energyProvided: [3]float64{100, 100, 100}, totalEnergyProvided: 2500}, 2500},
		{"Test get total energy provided without override", fields{energyProvided: [3]float64{100, 100, 100}, totalEnergyProvided: 300}, 300},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eu := &Usage{
				energyProvided:      tt.fields.energyProvided,
				totalEnergyProvided: tt.fields.totalEnergyProvided,
			}
			if got := eu.TotalEnergyProvided(); got != tt.want {
				t.Errorf("TotalEnergyProvided() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElectricityUsage_Valid(t *testing.T) {
	type fields struct {
		energyConsumed      [MaxPhases]float64
		totalEnergyConsumed float64
		energyProvided      [MaxPhases]float64
		totalEnergyProvided float64
	}
	tests := []struct {
		name    string
		fields  fields
		want    bool
		wantErr bool
	}{
		{"Test valid energyConsumed", fields{energyConsumed: [3]float64{10, 10, 10}}, true, false},
		{"Test invalid energyConsumed", fields{energyConsumed: [3]float64{-10, -10, -10}}, false, true},
		{"Test valid totalEnergyConsumer", fields{totalEnergyConsumed: 2000}, true, false},
		{"Test invalid totalEnergyConsumer", fields{totalEnergyConsumed: -2000}, false, true},
		{"Test valid energyProvided", fields{energyProvided: [3]float64{10, 10, 10}}, true, false},
		{"Test invalid energyProvided", fields{energyProvided: [3]float64{-10, -10, -10}}, false, true},
		{"Test valid totalEnergyProvided", fields{totalEnergyProvided: 2000}, true, false},
		{"Test invalid totalEnergyProvided", fields{totalEnergyProvided: -2000}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eu := &Usage{
				energyConsumed:      tt.fields.energyConsumed,
				totalEnergyConsumed: tt.fields.totalEnergyConsumed,
				energyProvided:      tt.fields.energyProvided,
				totalEnergyProvided: tt.fields.totalEnergyProvided,
			}
			got, err := eu.Valid()
			if (err != nil) != tt.wantErr {
				t.Errorf("Valid() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Valid() got = %v, want %v", got, tt.want)
			}
		})
	}
}

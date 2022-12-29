package energysource

import (
	"reflect"
	"testing"
)

func TestNewGrid(t *testing.T) {
	type args struct {
		gridConfig *GridConfig
	}
	gridConfig, _ := NewGridConfig(230, 25, 3)
	expected := NewGrid(gridConfig)
	tests := []struct {
		name string
		args args
		want *GridBase
	}{
		{"3Phases 230v 25amps", args{gridConfig}, expected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewGrid(tt.args.gridConfig)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewGrid() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGridConfig_Voltage(t *testing.T) {
	type fields struct {
		voltage float32
	}
	tests := []struct {
		name   string
		fields fields
		want   float32
	}{
		{"110v", fields{110}, 110},
		{"230v", fields{230}, 230},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GridConfig{
				voltage: tt.fields.voltage,
			}
			if got := g.Voltage(); got != tt.want {
				t.Errorf("Voltage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGridConfig_SetVoltage(t *testing.T) {
	type fields struct {
		voltage float32
	}
	type args struct {
		voltage float32
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Less than minimum", fields{0}, args{MinVoltage - 1}, true},
		{"110v", fields{0}, args{110}, false},
		{"230v", fields{0}, args{230}, false},
		{"More than maximum", fields{0}, args{MaxVoltage + 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GridConfig{
				voltage: tt.fields.voltage,
			}
			if err := g.SetVoltage(tt.args.voltage); (err != nil) != tt.wantErr {
				t.Errorf("setVoltage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGridConfig_MaxCurrentPerPhase(t *testing.T) {
	type fields struct {
		maxCurrentPerPhase float32
	}
	tests := []struct {
		name   string
		fields fields
		want   float32
	}{
		{"Max current per phase", fields{25}, 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GridConfig{
				maxCurrentPerPhase: tt.fields.maxCurrentPerPhase,
			}
			if got := g.MaxCurrentPerPhase(); got != tt.want {
				t.Errorf("MaxCurrentPerPhase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGridConfig_SetMaxCurrentPerPhase(t *testing.T) {
	type fields struct {
		maxCurrentPerPhase float32
	}
	type args struct {
		maxCurrentPerPhase float32
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Less than minimum", fields{0}, args{MinCurrentPerPhase - 1}, true},
		{"50amps", fields{0}, args{50}, false},
		{"More than maximum", fields{0}, args{MaxCurrentPerPhase + 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GridConfig{
				maxCurrentPerPhase: tt.fields.maxCurrentPerPhase,
			}
			if err := g.setMaxCurrentPerPhase(tt.args.maxCurrentPerPhase); (err != nil) != tt.wantErr {
				t.Errorf("setMaxCurrentPerPhase() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGridConfig_Phases(t *testing.T) {
	type fields struct {
		voltage            float32
		maxCurrentPerPhase float32
		phases             uint8
	}
	tests := []struct {
		name   string
		fields fields
		want   uint8
	}{
		{"1 Phase", fields{phases: 1}, 1},
		{"2 Phases", fields{phases: 2}, 2},
		{"3 Phases", fields{phases: 3}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &GridConfig{
				phases: tt.fields.phases,
			}
			if got := p.Phases(); got != tt.want {
				t.Errorf("Phases() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGridConfig_SetPhases(t *testing.T) {
	type fields struct {
		phases uint8
	}
	type args struct {
		phases uint8
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Less than minimum", fields{0}, args{phases: MinPhases - 1}, true},
		{"1 Phase", fields{0}, args{phases: 1}, false},
		{"2 Phases", fields{0}, args{phases: 2}, false},
		{"3 Phases", fields{0}, args{phases: 3}, false},
		{"More than maximum", fields{0}, args{phases: MaxPhases + 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &GridConfig{
				phases: tt.fields.phases,
			}
			if err := p.SetPhases(tt.args.phases); (err != nil) != tt.wantErr {
				t.Errorf("SetPhases() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGridConfig_MaxPowerPerPhase(t *testing.T) {
	type fields struct {
		voltage            float32
		maxCurrentPerPhase float32
	}
	tests := []struct {
		name   string
		fields fields
		want   uint32
	}{
		{"Max power 110v per phase", fields{110, 40}, 110 * 40},
		{"Max power 230v per phase", fields{230, 25}, 230 * 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GridConfig{
				voltage:            tt.fields.voltage,
				maxCurrentPerPhase: tt.fields.maxCurrentPerPhase,
			}
			if got := g.MaxPowerPerPhase(); got != tt.want {
				t.Errorf("MaxPowerPerPhase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGridConfig_MaxTotalPower(t *testing.T) {
	type fields struct {
		voltage            float32
		maxCurrentPerPhase float32
		phases             uint8
	}
	tests := []struct {
		name   string
		fields fields
		want   uint32
	}{
		{"Max power 110v all phases", fields{110, 40, 3}, 110 * 40 * 3},
		{"Max power 230v all phases", fields{230, 25, 3}, 230 * 25 * 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GridConfig{
				voltage:            tt.fields.voltage,
				maxCurrentPerPhase: tt.fields.maxCurrentPerPhase,
				phases:             tt.fields.phases,
			}
			if got := g.MaxTotalPower(); got != tt.want {
				t.Errorf("MaxTotalPower() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewGridConfig(t *testing.T) {
	type args struct {
		voltage            float32
		maxCurrentPerPhase float32
		phases             uint8
	}
	gridConfig, _ := NewGridConfig(110, 40, 1)
	tests := []struct {
		name    string
		args    args
		want    *GridConfig
		wantErr bool
	}{
		{"1Phase 110v 40amps", args{gridConfig.Voltage(), gridConfig.maxCurrentPerPhase, gridConfig.Phases()}, gridConfig, false},
		{"Invalid phases", args{MaxVoltage, MaxCurrentPerPhase, MaxPhases + 1}, nil, true},
		{"Invalid voltage", args{MinVoltage - 1, MinCurrentPerPhase, MaxPhases}, nil, true},
		{"Invalid current", args{MinVoltage, MinCurrentPerPhase - 1, MinPhases}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGridConfig(tt.args.voltage, tt.args.maxCurrentPerPhase, tt.args.phases)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPhaseHolder() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPhaseHolder() got = %v, want %v", got, tt.want)
			}
		})
	}
}

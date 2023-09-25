package meters_old

import (
	"enman/internal/domain"
)

type genericElectricityMeter struct {
	role        domain.EnergySourceRole
	lineIndices []uint8
	attributes  string
	phases      uint8
}

func (g *genericElectricityMeter) IsElectricityMeter() bool {
	return true
}

func (g *genericElectricityMeter) Role() domain.EnergySourceRole {
	return g.role
}

func (g *genericElectricityMeter) LineIndices() []uint8 {
	return g.lineIndices
}

func (g *genericElectricityMeter) Attributes() string {
	return g.attributes
}

func (g *genericElectricityMeter) Phases() uint8 {
	return g.phases
}

func (g *genericElectricityMeter) HasStateAttribute() bool {
	return "" == g.Attributes() || "state" == g.Attributes()
}

func (g *genericElectricityMeter) HasUsageAttribute() bool {
	return "" == g.Attributes() || "usage" == g.Attributes()
}

//type compoundElectricityMeter struct {
//	genericElectricityMeter
//	meters        []domain.ElectricityMeter
//	updateTicker  *time.Ticker
//	usageLastRead time.Time
//}
//
//func (cm *compoundElectricityMeter) WaitForInitialization() bool {
//	readableMeters := make([]domain.ElectricityMeter, 0)
//	for _, meter := range cm.meters {
//		if !meter.WaitForInitialization() {
//			log.Warningf("Unable to read values from electricity meter %s in role %v as it is an unknown device", cm.name, cm.role)
//			continue
//		}
//		readableMeters = append(readableMeters, meter)
//	}
//	cm.meters = readableMeters
//	if len(readableMeters) < 1 {
//		return false
//	}
//	return true
//}
//
//func (cm *compoundElectricityMeter) StartReading(ctx context.Context) {
//	if !cm.WaitForInitialization() {
//		log.Warningf("Unable to read values from compound electricity meter %s in role %v as it is an unknown device", cm.name, cm.role)
//		return
//	}
//	if len(cm.meters) == 0 || cm.updateTicker != nil {
//		// Nothing to do, or already started
//		return
//	}
//	modbusMeters := make([]*electricityModbusMeter, 0)
//	serialMeters := make([]*electricitySerialMeter, 0)
//	updateInterval := time.Nanosecond
//	for _, meter := range cm.meters {
//		modbusMeter, ok := meter.(*electricityModbusMeter)
//		if ok {
//			modbusMeters = append(modbusMeters, modbusMeter)
//			if modbusMeter.updateInterval > updateInterval {
//				updateInterval = modbusMeter.updateInterval
//			}
//		}
//		serialMeter, ok := meter.(*electricitySerialMeter)
//		if ok {
//			serialMeters = append(serialMeters, serialMeter)
//		}
//	}
//	if updateInterval == time.Nanosecond {
//		updateInterval = time.Millisecond * 500
//	}
//	cm.updateTicker = time.NewTicker(updateInterval)
//
//	go func() {
//		for {
//			select {
//			case <-ctx.Done():
//				if cm.updateTicker != nil {
//					cm.updateTicker.Stop()
//					cm.updateTicker = nil
//					log.Infof("Stop reading values from electricity meter %s in role %v", cm.name, cm.role)
//				}
//				for _, meter := range modbusMeters {
//					meter.shutdown()
//				}
//				for _, meter := range serialMeters {
//					meter.shutdown()
//				}
//				return
//			case _ = <-cm.updateTicker.C:
//				readWaitGroup := &sync.WaitGroup{}
//				state := domain.NewElectricityState()
//				usage := domain.NewElectricityUsage()
//				updateUsage := cm.shouldUpdateUsage()
//				for _, meter := range serialMeters {
//					readWaitGroup.Add(1)
//					var electricityState *domain.ElectricityState = nil
//					var electricityUsage *domain.ElectricityUsage = nil
//					if meter.HasStateAttribute() {
//						electricityState = state
//					}
//					if updateUsage && meter.HasUsageAttribute() {
//						electricityUsage = usage
//					}
//					go cm.readSerialValues(readWaitGroup, meter, electricityState, electricityUsage)
//				}
//				for _, meter := range modbusMeters {
//					readWaitGroup.Add(1)
//					var electricityState *domain.ElectricityState = nil
//					var electricityUsage *domain.ElectricityUsage = nil
//					if meter.HasStateAttribute() {
//						electricityState = state
//					}
//					if updateUsage && meter.HasUsageAttribute() {
//						electricityUsage = usage
//					}
//					go cm.readModbusValues(readWaitGroup, meter, electricityState, electricityUsage)
//				}
//				readWaitGroup.Wait()
//				if state.IsZero() {
//					state = nil
//				}
//				if !updateUsage || usage.IsZero() {
//					usage = nil
//				}
//				event := domain.NewElectricityMeterValues().
//					SetName(cm.name).
//					SetRole(cm.role).
//					SetReadLineIndices(cm.LineIndices()).
//					SetMeterPhases(uint8(len(cm.LineIndices()))).
//					SetElectricityState(state).
//					SetElectricityUsage(usage)
//				domain.ElectricityMeterReadings.Trigger(event)
//			}
//		}
//	}()
//}
//
//func (cm *compoundElectricityMeter) readSerialValues(waitGroup *sync.WaitGroup, meter *electricitySerialMeter, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
//	defer waitGroup.Done()
//	meter.readValues(meter, electricityState, electricityUsage)
//}
//
//func (cm *compoundElectricityMeter) readModbusValues(waitGroup *sync.WaitGroup, meter *electricityModbusMeter, electricityState *domain.ElectricityState, electricityUsage *domain.ElectricityUsage) {
//	defer waitGroup.Done()
//	meter.readValues(meter, electricityState, electricityUsage)
//}
//
//func (cm *compoundElectricityMeter) shouldUpdateUsage() bool {
//	if cm.usageLastRead.IsZero() || (time.Now().Sub(cm.usageLastRead) > meterUsageUpdateInterval) {
//		cm.usageLastRead = time.Now()
//		return true
//	}
//	return false
//}
//
//func NewCompoundElectricityMeter(name string, role domain.EnergySourceRole, meters []domain.ElectricityMeter) domain.ElectricityMeter {
//
//	genEnMet := &genericEnergyMeter{
//		name: name,
//	}
//
//	compoundMeter := &compoundElectricityMeter{
//		genericElectricityMeter: genericElectricityMeter{
//			genericEnergyMeter: genEnMet,
//			role: role,
//		},
//	}
//	lineIndices := make([]uint8, 0)
//	for _, m := range meters {
//		if m.Role() != role {
//			if log.WarningEnabled() {
//				log.Warningf("Meter %s has a different role %s compared to the compound role %s would like to be added. Skipping compoundMeter!", m.Name(), m.Role(), compoundMeter.Role())
//			}
//			continue
//		}
//		lineIndices = append(lineIndices, m.LineIndices()...)
//		compoundMeter.meters = append(compoundMeter.meters, m)
//	}
//	// Remove duplicates, which should never happen
//	lineIndicesMap := make(map[uint8]bool)
//	for _, element := range lineIndices {
//		lineIndicesMap[element] = true
//	}
//	lineIndices = make([]uint8, 0)
//	for lineIndex := range lineIndicesMap {
//		lineIndices = append(lineIndices, lineIndex)
//	}
//	// Sort the indices
//	sort.Slice(lineIndices, func(i, j int) bool { return lineIndices[i] < lineIndices[j] })
//	compoundMeter.lineIndices = lineIndices
//	return compoundMeter
//}

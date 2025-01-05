package gas

type Costs struct {
	consumptionUsage      float32
	consumptionPricePerM3 float32
	consumptionCosts      float32
}

func NewCosts() *Costs {
	return &Costs{}
}

func (c *Costs) ConsumptionUsage() float32 {
	return c.consumptionUsage
}

func (c *Costs) SetConsumptionUsage(consumptionUsage float32) *Costs {
	c.consumptionUsage = consumptionUsage
	return c
}

func (c *Costs) ConsumptionPricePerM3() float32 {
	return c.consumptionPricePerM3
}

func (c *Costs) SetConsumptionPricePerM3(consumptionPricePerM3 float32) *Costs {
	c.consumptionPricePerM3 = consumptionPricePerM3
	return c
}

func (c *Costs) ConsumptionCosts() float32 {
	return c.consumptionCosts
}

func (c *Costs) SetConsumptionCosts(consumptionCosts float32) *Costs {
	c.consumptionCosts = consumptionCosts
	return c
}

func (c *Costs) NetCosts() float32 {
	return c.consumptionCosts
}

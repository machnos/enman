package electricity

type Costs struct {
	consumptionEnergy      float32
	consumptionPricePerKwh float32
	consumptionCosts       float32
	feedbackEnergy         float32
	feedbackPricePerKwh    float32
	feedbackCosts          float32
}

func NewCosts() *Costs {
	return &Costs{}
}

func (c *Costs) ConsumptionEnergy() float32 {
	return c.consumptionEnergy
}

func (c *Costs) SetConsumptionEnergy(consumptionEnergy float32) *Costs {
	c.consumptionEnergy = consumptionEnergy
	return c
}

func (c *Costs) ConsumptionPricePerKwh() float32 {
	return c.consumptionPricePerKwh
}

func (c *Costs) SetConsumptionPricePerKwh(consumptionPricePerKwh float32) *Costs {
	c.consumptionPricePerKwh = consumptionPricePerKwh
	return c
}

func (c *Costs) ConsumptionCosts() float32 {
	return c.consumptionCosts
}

func (c *Costs) SetConsumptionCosts(consumptionCosts float32) *Costs {
	c.consumptionCosts = consumptionCosts
	return c
}

func (c *Costs) FeedbackEnergy() float32 {
	return c.feedbackEnergy
}

func (c *Costs) SetFeedbackEnergy(feedbackEnergy float32) *Costs {
	c.feedbackEnergy = feedbackEnergy
	return c
}

func (c *Costs) FeedbackPricePerKwh() float32 {
	return c.feedbackPricePerKwh
}

func (c *Costs) SetFeedbackPricePerKwh(feedbackPricePerKwh float32) *Costs {
	c.feedbackPricePerKwh = feedbackPricePerKwh
	return c
}

func (c *Costs) FeedbackCosts() float32 {
	return c.feedbackCosts
}

func (c *Costs) SetFeedbackCosts(feedbackCosts float32) *Costs {
	c.feedbackCosts = feedbackCosts
	return c
}

func (c *Costs) NetCosts() float32 {
	return c.consumptionCosts - c.FeedbackCosts()
}

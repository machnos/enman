package persistency

import "enman/pkg/energysource"

type Repository interface {
	StoreGridValues(energysource.Grid)
	StorePvValues(energysource.Pv)
	Close()
}

type NoopRepository struct {
	Repository
}

func (n *NoopRepository) StoreGridValues(grid energysource.Grid) {
}

func (n *NoopRepository) StorePvValues(energysource.Pv) {
}

func (n *NoopRepository) Close() {
}

func NewNoopRepository() *NoopRepository {
	return &NoopRepository{}
}

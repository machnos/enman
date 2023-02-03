package persistency

import (
	energysource2 "enman/internal/energysource"
)

type Repository interface {
	StoreGridValues(energysource2.Grid)
	StorePvValues(energysource2.Pv)
	Close()
}

type NoopRepository struct {
	Repository
}

func (n *NoopRepository) StoreGridValues(grid energysource2.Grid) {
}

func (n *NoopRepository) StorePvValues(energysource2.Pv) {
}

func (n *NoopRepository) Close() {
}

func NewNoopRepository() *NoopRepository {
	return &NoopRepository{}
}

package prices

import (
	"context"
	"time"
)

type PriceImporter interface {
	ImportPrices(ctx context.Context, startDate time.Time, endDate time.Time) error
}

package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/platform/events"
)

type Processor struct {
	outbox  events.Reader
	service *Service
}

func NewProcessor(outbox events.Reader, service *Service) *Processor {
	return &Processor{
		outbox:  outbox,
		service: service,
	}
}

func (p *Processor) ProcessPendingClosedOrders(ctx context.Context, limit int) (int, error) {
	if p.outbox == nil {
		return 0, fmt.Errorf("outbox reader is required")
	}
	if p.service == nil {
		return 0, fmt.Errorf("inventory service is required")
	}

	pending, err := p.outbox.ListUnpublished(ctx, contracts.ClosedOrderEventName, limit)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, event := range pending {
		var order contracts.ClosedOrder
		if err := json.Unmarshal(event.Payload, &order); err != nil {
			return processed, err
		}
		if order.LocationID == "" {
			order.LocationID = event.LocationID
		}
		if _, err := p.service.RecordDepletion(ctx, order); err != nil {
			return processed, err
		}
		if err := p.outbox.MarkPublished(ctx, event.ID, time.Now().UTC()); err != nil && !errors.Is(err, context.Canceled) {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

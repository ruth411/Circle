package diner

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
		return 0, fmt.Errorf("diner service is required")
	}

	pending, err := p.outbox.ListUnpublished(ctx, outboxConsumer, contracts.ClosedOrderEventName, limit)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, event := range pending {
		var order contracts.ClosedOrder
		if err := json.Unmarshal(event.Payload, &order); err != nil {
			if err := markInvalidEventDelivered(ctx, p.outbox, event.ID, "invalid_json", err); err != nil {
				return processed, err
			}
			continue
		}
		if order.LocationID == "" {
			order.LocationID = event.LocationID
		}
		if _, err := p.service.IssueToken(ctx, order); err != nil {
			if errors.Is(err, ErrInvalidTokenData) {
				if err := markInvalidEventDelivered(ctx, p.outbox, event.ID, "invalid_closed_order", err); err != nil {
					return processed, err
				}
				continue
			}
			return processed, err
		}
		if err := p.outbox.MarkPublished(ctx, outboxConsumer, event.ID, time.Now().UTC()); err != nil && !errors.Is(err, context.Canceled) {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func markInvalidEventDelivered(ctx context.Context, outbox events.Reader, eventID string, failureKind string, cause error) error {
	if err := outbox.MarkInvalid(ctx, outboxConsumer, eventID, failureKind, cause.Error(), time.Now().UTC()); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

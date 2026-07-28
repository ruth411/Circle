package ordering

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ruth411/circle/internal/core/recipe"
)

type memoryRepository struct {
	mu      sync.Mutex
	orders  map[string]Order
	tenders map[string]memoryTender
}

type memoryTender struct {
	OrderID string
	CheckID string
	Status  string
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		orders:  map[string]Order{},
		tenders: map[string]memoryTender{},
	}
}

func (r *memoryRepository) Get(_ context.Context, locationID string, orderID string) (Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return Order{}, ErrOrderNotFound
	}
	return cloneOrder(order), nil
}

func (r *memoryRepository) Create(_ context.Context, order Order) (Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.orders[order.ID]; ok {
		if existing.CheckID == order.CheckID &&
			existing.LocationID == order.LocationID &&
			existing.SnapshotID == order.SnapshotID &&
			existing.SnapshotVersion == order.SnapshotVersion &&
			existing.BusinessDate == order.BusinessDate {
			return cloneOrder(existing), nil
		}
		return Order{}, fmt.Errorf("%w: order %s already exists with different attributes", ErrInvalidOrder, order.ID)
	}

	order.TotalMinor = 0
	order.Currency = ""
	r.orders[order.ID] = order
	return cloneOrder(order), nil
}

func (r *memoryRepository) AddLine(_ context.Context, locationID string, orderID string, line OrderLine) (OrderLine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return OrderLine{}, ErrOrderNotFound
	}
	if order.Status != OrderStatusOpen {
		return OrderLine{}, fmt.Errorf("%w: order %s is not editable", ErrOrderNotEditable, orderID)
	}

	if line.LineID == "" {
		line.LineID = fmt.Sprintf("%s-%d", order.ID, len(order.Lines)+1)
	}
	for _, existing := range order.Lines {
		if existing.LineID == line.LineID {
			return OrderLine{}, fmt.Errorf("%w: line id %s already exists", ErrInvalidOrder, line.LineID)
		}
	}
	if order.Currency != "" && order.Currency != line.Currency {
		return OrderLine{}, fmt.Errorf("%w: line currency %s does not match order currency %s", ErrInvalidOrder, line.Currency, order.Currency)
	}
	if order.Currency == "" {
		order.Currency = line.Currency
	}
	order.TotalMinor += line.ResolvedPriceMinor
	order.Lines = append(order.Lines, line)
	r.orders[order.ID] = order
	return cloneLine(line), nil
}

func (r *memoryRepository) StartClose(_ context.Context, locationID string, orderID string, tender Tender) (Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return Order{}, ErrOrderNotFound
	}
	if order.Status == OrderStatusClosed {
		return cloneOrder(order), nil
	}
	if order.Status == OrderStatusClosing {
		return Order{}, ErrOrderAlreadyClosing
	}
	if tender.CheckID != order.CheckID {
		return Order{}, fmt.Errorf("%w: tender check id %s does not match order check id %s", ErrInvalidOrder, tender.CheckID, order.CheckID)
	}
	if order.Currency != "" && tender.Currency != order.Currency {
		return Order{}, fmt.Errorf("%w: tender currency %s does not match order currency %s", ErrInvalidOrder, tender.Currency, order.Currency)
	}
	if tender.AmountMinor < order.TotalMinor {
		return Order{}, fmt.Errorf("%w: tender amount %d is less than order total %d", ErrUnderpaidTender, tender.AmountMinor, order.TotalMinor)
	}

	order.Status = OrderStatusClosing
	r.orders[order.ID] = order
	r.tenderKey(locationID, tender.ID)
	r.tenders[r.tenderKey(locationID, tender.ID)] = memoryTender{
		OrderID: order.ID,
		CheckID: order.CheckID,
		Status:  "pending",
	}
	return cloneOrder(order), nil
}

func (r *memoryRepository) MarkTenderSucceeded(_ context.Context, locationID string, orderID string, tenderID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return ErrOrderNotFound
	}
	tender, ok := r.tenders[r.tenderKey(locationID, tenderID)]
	if !ok || tender.OrderID != orderID || tender.CheckID != order.CheckID {
		return fmt.Errorf("%w: tender %s is not pending for check %s", ErrInvalidOrder, tenderID, order.CheckID)
	}
	switch tender.Status {
	case "pending":
		tender.Status = "succeeded"
		r.tenders[r.tenderKey(locationID, tenderID)] = tender
		return nil
	case "succeeded":
		return nil
	default:
		return fmt.Errorf("%w: tender %s is not pending for check %s", ErrInvalidOrder, tenderID, order.CheckID)
	}
}

func (r *memoryRepository) FailClose(_ context.Context, locationID string, orderID string, tenderID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return ErrOrderNotFound
	}
	if tenderID == "" {
		return fmt.Errorf("%w: tender id is required", ErrInvalidOrder)
	}
	if order.Status == OrderStatusClosing {
		order.Status = OrderStatusOpen
		r.orders[order.ID] = order
	}
	if tender, ok := r.tenders[r.tenderKey(locationID, tenderID)]; ok {
		tender.Status = "failed"
		r.tenders[r.tenderKey(locationID, tenderID)] = tender
	}
	return nil
}

func (r *memoryRepository) FinishClose(_ context.Context, locationID string, orderID string, tenderID string, closedAt time.Time) (Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return Order{}, ErrOrderNotFound
	}
	if tenderID == "" {
		return Order{}, fmt.Errorf("%w: tender id is required", ErrInvalidOrder)
	}
	if order.Status == OrderStatusClosed {
		return cloneOrder(order), nil
	}
	if order.Status != OrderStatusClosing {
		return Order{}, fmt.Errorf("%w: order %s is not ready to close", ErrInvalidOrder, order.ID)
	}
	tender, ok := r.tenders[r.tenderKey(locationID, tenderID)]
	if !ok || tender.OrderID != orderID || tender.CheckID != order.CheckID || tender.Status != "succeeded" {
		return Order{}, fmt.Errorf("%w: tender %s is not succeeded for check %s", ErrInvalidOrder, tenderID, order.CheckID)
	}

	closedAt = closedAt.UTC()
	order.Status = OrderStatusClosed
	order.ClosedAt = &closedAt
	r.orders[order.ID] = order
	return cloneOrder(order), nil
}

func (r *memoryRepository) tenderKey(locationID string, tenderID string) string {
	return locationID + "|" + tenderID
}

type memorySnapshotStore struct {
	mu        sync.Mutex
	snapshots map[string]recipe.MenuSnapshot
}

func newMemorySnapshotStore() *memorySnapshotStore {
	return &memorySnapshotStore{
		snapshots: map[string]recipe.MenuSnapshot{},
	}
}

func (s *memorySnapshotStore) Register(snapshot recipe.MenuSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.ID] = snapshot
}

func (s *memorySnapshotStore) GetSnapshot(_ context.Context, locationID string, snapshotID string) (recipe.MenuSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.snapshots[snapshotID]
	if !ok || snapshot.LocationID != locationID {
		return recipe.MenuSnapshot{}, recipe.ErrSnapshotNotFound
	}
	return snapshot, nil
}

package ordering

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
)

type OrderStatus string

const (
	OrderStatusOpen   OrderStatus = "open"
	OrderStatusClosed OrderStatus = "closed"
)

type PaymentProvider interface {
	Process(context.Context, Tender) error
}

type MockProvider struct {
	Err error
}

func (m MockProvider) Process(_ context.Context, _ Tender) error {
	return m.Err
}

type Tender struct {
	ID          string
	CheckID     string
	AmountMinor int64
	Currency    string
	Kind        string
}

type OrderLine struct {
	LineID             string
	MenuItemID         string
	Name               string
	Quantity           int
	ResolvedPriceMinor int64
	Currency           string
	ResolvedMacros     ingredient.MacroValues
	IngredientUsage    map[string]float64
	SelectedModifiers  []recipe.SnapshotModifier
}

type Order struct {
	ID              string
	CheckID         string
	LocationID      string
	SnapshotID      string
	SnapshotVersion int
	BusinessDate    time.Time
	Status          OrderStatus
	Lines           []OrderLine
	ClosedAt        *time.Time
}

type CreateOrderInput struct {
	OrderID      string
	CheckID      string
	LocationID   string
	SnapshotID   string
	BusinessDate time.Time
}

type AddLineInput struct {
	OrderID     string
	LineID      string
	MenuItemID  string
	ModifierIDs []string
	Quantity    int
}

type CloseCheckInput struct {
	OrderID string
	Tender  Tender
}

type Service struct {
	mu        sync.Mutex
	payment   PaymentProvider
	orders    map[string]Order
	snapshots map[string]recipe.MenuSnapshot
}

func NewService(payment PaymentProvider) *Service {
	return &Service{
		payment:   payment,
		orders:    map[string]Order{},
		snapshots: map[string]recipe.MenuSnapshot{},
	}
}

func (s *Service) RegisterSnapshot(snapshot recipe.MenuSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.ID] = snapshot
}

func (s *Service) CreateOrder(input CreateOrderInput) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, ok := s.snapshots[input.SnapshotID]
	if !ok {
		return Order{}, fmt.Errorf("snapshot %s not found", input.SnapshotID)
	}

	if existing, ok := s.orders[input.OrderID]; ok {
		if existing.SnapshotID == input.SnapshotID && existing.LocationID == input.LocationID {
			return cloneOrder(existing), nil
		}
		return Order{}, fmt.Errorf("order %s already exists with different attributes", input.OrderID)
	}

	checkID := input.CheckID
	if checkID == "" {
		checkID = input.OrderID
	}

	order := Order{
		ID:              input.OrderID,
		CheckID:         checkID,
		LocationID:      input.LocationID,
		SnapshotID:      snapshot.ID,
		SnapshotVersion: snapshot.Version,
		BusinessDate:    input.BusinessDate.UTC(),
		Status:          OrderStatusOpen,
	}

	s.orders[order.ID] = order
	return cloneOrder(order), nil
}

func (s *Service) AddLine(input AddLineInput) (OrderLine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.Quantity <= 0 {
		return OrderLine{}, fmt.Errorf("quantity must be positive")
	}

	order, ok := s.orders[input.OrderID]
	if !ok {
		return OrderLine{}, fmt.Errorf("order %s not found", input.OrderID)
	}
	if order.Status == OrderStatusClosed {
		return OrderLine{}, fmt.Errorf("order %s is closed", input.OrderID)
	}

	snapshot, ok := s.snapshots[order.SnapshotID]
	if !ok {
		return OrderLine{}, fmt.Errorf("snapshot %s not found", order.SnapshotID)
	}

	item, err := findSnapshotItem(snapshot, input.MenuItemID)
	if err != nil {
		return OrderLine{}, err
	}

	selectedModifiers, err := selectModifiers(item, input.ModifierIDs)
	if err != nil {
		return OrderLine{}, err
	}

	lineID := input.LineID
	if lineID == "" {
		lineID = fmt.Sprintf("%s-%d", order.ID, len(order.Lines)+1)
	}

	unitPrice := item.PriceMinor
	macros := item.Macros
	usage := cloneUsage(item.IngredientUsage)

	for _, modifier := range selectedModifiers {
		unitPrice += modifier.PriceDeltaMinor
		macros = macros.Add(modifier.MacroDelta)
		mergeUsage(usage, modifier.IngredientUsage, 1)
	}

	line := OrderLine{
		LineID:             lineID,
		MenuItemID:         item.MenuItemID,
		Name:               item.Name,
		Quantity:           input.Quantity,
		ResolvedPriceMinor: unitPrice * int64(input.Quantity),
		Currency:           item.Currency,
		ResolvedMacros:     macros.Scale(float64(input.Quantity)),
		IngredientUsage:    scaleUsage(usage, float64(input.Quantity)),
		SelectedModifiers:  selectedModifiers,
	}

	order.Lines = append(order.Lines, line)
	s.orders[order.ID] = order
	return cloneLine(line), nil
}

func (s *Service) CloseCheck(ctx context.Context, input CloseCheckInput) (Order, error) {
	s.mu.Lock()
	order, ok := s.orders[input.OrderID]
	if !ok {
		s.mu.Unlock()
		return Order{}, fmt.Errorf("order %s not found", input.OrderID)
	}
	if order.Status == OrderStatusClosed {
		s.mu.Unlock()
		return cloneOrder(order), nil
	}

	total := orderTotal(order)
	if input.Tender.AmountMinor < total {
		s.mu.Unlock()
		return Order{}, fmt.Errorf("tender amount %d is less than order total %d", input.Tender.AmountMinor, total)
	}
	s.mu.Unlock()

	if err := s.payment.Process(ctx, input.Tender); err != nil {
		return Order{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	order = s.orders[input.OrderID]
	now := time.Now().UTC()
	order.Status = OrderStatusClosed
	order.ClosedAt = &now
	s.orders[order.ID] = order
	return cloneOrder(order), nil
}

func findSnapshotItem(snapshot recipe.MenuSnapshot, menuItemID string) (recipe.SnapshotItem, error) {
	for _, item := range snapshot.Items {
		if item.MenuItemID == menuItemID {
			return item, nil
		}
	}

	return recipe.SnapshotItem{}, fmt.Errorf("menu item %s not found in snapshot %s", menuItemID, snapshot.ID)
}

func selectModifiers(item recipe.SnapshotItem, selectedIDs []string) ([]recipe.SnapshotModifier, error) {
	selected := map[string]bool{}
	for _, id := range selectedIDs {
		if selected[id] {
			return nil, fmt.Errorf("modifier %s selected multiple times", id)
		}
		selected[id] = true
	}

	var out []recipe.SnapshotModifier
	seen := map[string]bool{}
	for _, group := range item.ModifierGroups {
		count := 0
		for _, modifier := range group.Modifiers {
			if !selected[modifier.ModifierID] {
				continue
			}
			count++
			seen[modifier.ModifierID] = true
			out = append(out, cloneModifier(modifier))
		}

		if group.Required && count == 0 {
			return nil, fmt.Errorf("modifier group %s requires a selection", group.GroupID)
		}
		if count < group.SelectionMin {
			return nil, fmt.Errorf("modifier group %s requires at least %d selections", group.GroupID, group.SelectionMin)
		}
		if group.SelectionMax > 0 && count > group.SelectionMax {
			return nil, fmt.Errorf("modifier group %s allows at most %d selections", group.GroupID, group.SelectionMax)
		}
		if group.Exclusive && count > 1 {
			return nil, fmt.Errorf("modifier group %s is exclusive", group.GroupID)
		}
	}

	for _, id := range selectedIDs {
		if !seen[id] {
			return nil, fmt.Errorf("modifier %s not found for menu item %s", id, item.MenuItemID)
		}
	}

	return out, nil
}

func orderTotal(order Order) int64 {
	var total int64
	for _, line := range order.Lines {
		total += line.ResolvedPriceMinor
	}
	return total
}

func cloneOrder(order Order) Order {
	out := order
	out.Lines = make([]OrderLine, len(order.Lines))
	for i, line := range order.Lines {
		out.Lines[i] = cloneLine(line)
	}
	return out
}

func cloneLine(line OrderLine) OrderLine {
	out := line
	out.IngredientUsage = cloneUsage(line.IngredientUsage)
	out.SelectedModifiers = make([]recipe.SnapshotModifier, len(line.SelectedModifiers))
	for i, modifier := range line.SelectedModifiers {
		out.SelectedModifiers[i] = cloneModifier(modifier)
	}
	return out
}

func cloneModifier(modifier recipe.SnapshotModifier) recipe.SnapshotModifier {
	out := modifier
	out.IngredientUsage = cloneUsage(modifier.IngredientUsage)
	return out
}

func cloneUsage(usage map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(usage))
	for ingredientID, qty := range usage {
		out[ingredientID] = qty
	}
	return out
}

func mergeUsage(dst map[string]float64, src map[string]float64, multiplier float64) {
	for ingredientID, qty := range src {
		dst[ingredientID] += qty * multiplier
	}
}

func scaleUsage(src map[string]float64, multiplier float64) map[string]float64 {
	out := make(map[string]float64, len(src))
	for ingredientID, qty := range src {
		out[ingredientID] = qty * multiplier
	}
	return out
}

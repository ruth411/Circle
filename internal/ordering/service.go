package ordering

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/platform/biztime"
)

var (
	ErrOrderNotFound       = errors.New("order not found")
	ErrInvalidOrder        = errors.New("invalid order")
	ErrOrderNotEditable    = errors.New("order not editable")
	ErrOrderAlreadyClosing = errors.New("order is already closing")
	ErrUnderpaidTender     = errors.New("underpaid tender")
	ErrPaymentFailed       = errors.New("payment failed")
)

type OrderStatus string

const (
	OrderStatusOpen    OrderStatus = "open"
	OrderStatusClosing OrderStatus = "closing"
	OrderStatusClosed  OrderStatus = "closed"
)

type PaymentProvider interface {
	Process(context.Context, Tender) error
}

type SnapshotLookup interface {
	GetSnapshot(context.Context, string, string) (recipe.MenuSnapshot, error)
}

type Repository interface {
	Get(context.Context, string, string) (Order, error)
	Create(context.Context, Order) (Order, error)
	AddLine(context.Context, string, string, OrderLine) (OrderLine, error)
	StartClose(context.Context, string, string, Tender) (Order, error)
	MarkTenderSucceeded(context.Context, string, string, string) error
	FailClose(context.Context, string, string, string) error
	FinishClose(context.Context, string, string, string, time.Time) (Order, error)
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
	IngredientUnits    map[string]ingredient.Unit
	SelectedModifiers  []recipe.SnapshotModifier
}

type Order struct {
	ID              string
	CheckID         string
	LocationID      string
	SnapshotID      string
	SnapshotVersion int
	BusinessDate    biztime.BusinessDate
	Status          OrderStatus
	TotalMinor      int64
	Currency        string
	Lines           []OrderLine
	ClosedAt        *time.Time
}

type CreateOrderInput struct {
	OrderID      string
	CheckID      string
	LocationID   string
	SnapshotID   string
	BusinessDate biztime.BusinessDate
}

type AddLineInput struct {
	LocationID  string
	OrderID     string
	LineID      string
	MenuItemID  string
	ModifierIDs []string
	Quantity    int
}

type CloseCheckInput struct {
	LocationID string
	OrderID    string
	Tender     Tender
}

type Service struct {
	repo      Repository
	snapshots SnapshotLookup
	payment   PaymentProvider
	mu        sync.Mutex
	inFlight  map[string]string
}

func NewService(payment PaymentProvider) *Service {
	return NewServiceWithDependencies(newMemoryRepository(), newMemorySnapshotStore(), payment)
}

func NewServiceWithDependencies(repo Repository, snapshots SnapshotLookup, payment PaymentProvider) *Service {
	if payment == nil {
		payment = MockProvider{}
	}
	return &Service{
		repo:      repo,
		snapshots: snapshots,
		payment:   payment,
		inFlight:  map[string]string{},
	}
}

func (s *Service) RegisterSnapshot(snapshot recipe.MenuSnapshot) error {
	if store, ok := s.snapshots.(*memorySnapshotStore); ok {
		store.Register(snapshot)
		return nil
	}

	return fmt.Errorf("snapshot registration is only supported by the in-memory snapshot store")
}

func (s *Service) GetOrder(ctx context.Context, locationID string, orderID string) (Order, error) {
	if s.repo == nil {
		return Order{}, fmt.Errorf("order repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	orderID = strings.TrimSpace(orderID)
	if locationID == "" {
		return Order{}, fmt.Errorf("%w: location id is required", ErrInvalidOrder)
	}
	if orderID == "" {
		return Order{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	return s.repo.Get(ctx, locationID, orderID)
}

func (s *Service) CreateOrder(ctx context.Context, input CreateOrderInput) (Order, error) {
	if s.repo == nil {
		return Order{}, fmt.Errorf("order repository is required")
	}
	if s.snapshots == nil {
		return Order{}, fmt.Errorf("snapshot lookup is required")
	}

	orderID := strings.TrimSpace(input.OrderID)
	locationID := strings.TrimSpace(input.LocationID)
	snapshotID := strings.TrimSpace(input.SnapshotID)
	checkID := strings.TrimSpace(input.CheckID)

	if orderID == "" {
		return Order{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	if locationID == "" {
		return Order{}, fmt.Errorf("%w: location id is required", ErrInvalidOrder)
	}
	if snapshotID == "" {
		return Order{}, fmt.Errorf("%w: snapshot id is required", ErrInvalidOrder)
	}
	if err := input.BusinessDate.Valid(); err != nil {
		return Order{}, fmt.Errorf("%w: %v", ErrInvalidOrder, err)
	}
	if checkID == "" {
		checkID = orderID
	}

	snapshot, err := s.snapshots.GetSnapshot(ctx, locationID, snapshotID)
	if err != nil {
		return Order{}, err
	}

	return s.repo.Create(ctx, Order{
		ID:              orderID,
		CheckID:         checkID,
		LocationID:      locationID,
		SnapshotID:      snapshot.ID,
		SnapshotVersion: snapshot.Version,
		BusinessDate:    input.BusinessDate,
		Status:          OrderStatusOpen,
	})
}

func (s *Service) AddLine(ctx context.Context, input AddLineInput) (OrderLine, error) {
	if s.repo == nil {
		return OrderLine{}, fmt.Errorf("order repository is required")
	}
	if s.snapshots == nil {
		return OrderLine{}, fmt.Errorf("snapshot lookup is required")
	}

	locationID := strings.TrimSpace(input.LocationID)
	orderID := strings.TrimSpace(input.OrderID)
	menuItemID := strings.TrimSpace(input.MenuItemID)
	lineID := strings.TrimSpace(input.LineID)
	if locationID == "" {
		return OrderLine{}, fmt.Errorf("%w: location id is required", ErrInvalidOrder)
	}
	if orderID == "" {
		return OrderLine{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}
	if menuItemID == "" {
		return OrderLine{}, fmt.Errorf("%w: menu item id is required", ErrInvalidOrder)
	}
	if input.Quantity <= 0 {
		return OrderLine{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidOrder)
	}

	order, err := s.repo.Get(ctx, locationID, orderID)
	if err != nil {
		return OrderLine{}, err
	}
	if order.Status != OrderStatusOpen {
		return OrderLine{}, fmt.Errorf("%w: order %s is not editable", ErrOrderNotEditable, order.ID)
	}

	snapshot, err := s.snapshots.GetSnapshot(ctx, order.LocationID, order.SnapshotID)
	if err != nil {
		return OrderLine{}, err
	}

	item, err := findSnapshotItem(snapshot, menuItemID)
	if err != nil {
		return OrderLine{}, fmt.Errorf("%w: %v", ErrInvalidOrder, err)
	}
	selectedModifiers, err := selectModifiers(item, input.ModifierIDs)
	if err != nil {
		return OrderLine{}, fmt.Errorf("%w: %v", ErrInvalidOrder, err)
	}

	unitPrice := item.PriceMinor
	macros := item.Macros
	usage := cloneUsage(item.IngredientUsage)
	units := cloneUnits(item.IngredientUnits)
	if units == nil {
		units = map[string]ingredient.Unit{}
	}

	for _, modifier := range selectedModifiers {
		unitPrice += modifier.PriceDeltaMinor
		macros = macros.Add(modifier.MacroDelta)
		mergeUsage(usage, modifier.IngredientUsage, 1)
		mergeUnits(units, modifier.IngredientUnits)
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
		IngredientUnits:    units,
		SelectedModifiers:  selectedModifiers,
	}

	return s.repo.AddLine(ctx, locationID, order.ID, line)
}

func (s *Service) CloseCheck(ctx context.Context, input CloseCheckInput) (Order, error) {
	if s.repo == nil {
		return Order{}, fmt.Errorf("order repository is required")
	}

	locationID := strings.TrimSpace(input.LocationID)
	orderID := strings.TrimSpace(input.OrderID)
	if locationID == "" {
		return Order{}, fmt.Errorf("%w: location id is required", ErrInvalidOrder)
	}
	if orderID == "" {
		return Order{}, fmt.Errorf("%w: order id is required", ErrInvalidOrder)
	}

	current, err := s.repo.Get(ctx, locationID, orderID)
	if err != nil {
		return Order{}, err
	}
	if current.Status == OrderStatusClosed {
		return current, nil
	}

	tender, err := normalizeTender(current, input.Tender)
	if err != nil {
		return Order{}, err
	}
	if current.Status == OrderStatusClosing {
		if s.paymentInFlight(locationID, orderID, tender.ID) {
			return Order{}, ErrOrderAlreadyClosing
		}
		return s.finalizeClosingOrder(ctx, locationID, orderID, tender)
	}

	order, err := s.repo.StartClose(ctx, locationID, orderID, tender)
	if err != nil {
		return Order{}, err
	}
	if order.Status == OrderStatusClosed {
		return order, nil
	}

	s.setPaymentInFlight(locationID, orderID, tender.ID, true)
	defer s.setPaymentInFlight(locationID, orderID, tender.ID, false)

	if err := s.payment.Process(ctx, tender); err != nil {
		paymentErr := fmt.Errorf("%w: %v", ErrPaymentFailed, err)
		if rollbackErr := s.repo.FailClose(backgroundCloseContext(ctx), locationID, orderID, tender.ID); rollbackErr != nil {
			return Order{}, errors.Join(paymentErr, fmt.Errorf("reopen order after payment failure: %w", rollbackErr))
		}
		return Order{}, paymentErr
	}

	return s.finalizeClosingOrder(ctx, locationID, orderID, tender)
}

func backgroundCloseContext(ctx context.Context) context.Context {
	closeCtx, _ := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	return closeCtx
}

func (s *Service) paymentInFlight(locationID string, orderID string, tenderID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.inFlight[locationID+"|"+orderID] == tenderID
}

func (s *Service) setPaymentInFlight(locationID string, orderID string, tenderID string, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := locationID + "|" + orderID
	if active {
		s.inFlight[key] = tenderID
		return
	}
	delete(s.inFlight, key)
}

func (s *Service) finalizeClosingOrder(ctx context.Context, locationID string, orderID string, tender Tender) (Order, error) {
	closeCtx := backgroundCloseContext(ctx)
	closed, err := s.repo.FinishClose(closeCtx, locationID, orderID, tender.ID, time.Now().UTC())
	if err == nil {
		return closed, nil
	}
	if !errors.Is(err, ErrInvalidOrder) {
		return Order{}, err
	}
	if err := s.repo.MarkTenderSucceeded(closeCtx, locationID, orderID, tender.ID); err != nil {
		if errors.Is(err, ErrInvalidOrder) {
			return Order{}, ErrOrderAlreadyClosing
		}
		return Order{}, err
	}
	return s.repo.FinishClose(closeCtx, locationID, orderID, tender.ID, time.Now().UTC())
}

func ToClosedOrder(order Order) (contracts.ClosedOrder, error) {
	if order.Status != OrderStatusClosed || order.ClosedAt == nil {
		return contracts.ClosedOrder{}, fmt.Errorf("order %s is not closed", order.ID)
	}

	lines := make([]contracts.ClosedOrderLine, len(order.Lines))
	for i, line := range order.Lines {
		lines[i] = contracts.ClosedOrderLine{
			LineID:          line.LineID,
			Name:            line.Name,
			Quantity:        line.Quantity,
			ResolvedMacros:  line.ResolvedMacros,
			IngredientUsage: cloneUsage(line.IngredientUsage),
			IngredientUnits: cloneUnits(line.IngredientUnits),
		}
	}

	return contracts.ClosedOrder{
		OrderID:    order.ID,
		LocationID: order.LocationID,
		ClosedAt:   order.ClosedAt.UTC(),
		Lines:      lines,
	}, nil
}

func normalizeTender(order Order, input Tender) (Tender, error) {
	tender := Tender{
		ID:          strings.TrimSpace(input.ID),
		CheckID:     strings.TrimSpace(input.CheckID),
		AmountMinor: input.AmountMinor,
		Currency:    strings.ToUpper(strings.TrimSpace(input.Currency)),
		Kind:        strings.TrimSpace(input.Kind),
	}

	if tender.ID == "" {
		return Tender{}, fmt.Errorf("%w: tender id is required", ErrInvalidOrder)
	}
	if tender.Kind == "" {
		return Tender{}, fmt.Errorf("%w: tender kind is required", ErrInvalidOrder)
	}
	if tender.AmountMinor < 0 {
		return Tender{}, fmt.Errorf("%w: tender amount must be non-negative", ErrInvalidOrder)
	}
	if tender.CheckID == "" {
		tender.CheckID = order.CheckID
	}
	if tender.CheckID != order.CheckID {
		return Tender{}, fmt.Errorf("%w: tender check id %s does not match order check id %s", ErrInvalidOrder, tender.CheckID, order.CheckID)
	}
	if tender.Currency == "" {
		tender.Currency = order.Currency
	}
	if order.Currency != "" && tender.Currency != order.Currency {
		return Tender{}, fmt.Errorf("%w: tender currency %s does not match order currency %s", ErrInvalidOrder, tender.Currency, order.Currency)
	}

	return tender, nil
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
	for _, rawID := range selectedIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return nil, fmt.Errorf("modifier id is required")
		}
		if selected[id] {
			return nil, fmt.Errorf("modifier %s selected multiple times", id)
		}
		selected[id] = true
	}

	var out []recipe.SnapshotModifier
	seen := map[string]bool{}
	for _, group := range item.ModifierGroups {
		count := 0
		groupSelected := map[string]bool{}
		for _, defaultID := range group.DefaultModifierIDs {
			groupSelected[defaultID] = true
		}
		for _, modifier := range group.Modifiers {
			if selected[modifier.ModifierID] {
				groupSelected[modifier.ModifierID] = true
			}
			if !groupSelected[modifier.ModifierID] {
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

	for id := range selected {
		if !seen[id] {
			return nil, fmt.Errorf("modifier %s not found for menu item %s", id, item.MenuItemID)
		}
	}

	return out, nil
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
	out.IngredientUnits = cloneUnits(line.IngredientUnits)
	out.SelectedModifiers = make([]recipe.SnapshotModifier, len(line.SelectedModifiers))
	for i, modifier := range line.SelectedModifiers {
		out.SelectedModifiers[i] = cloneModifier(modifier)
	}
	return out
}

func cloneModifier(modifier recipe.SnapshotModifier) recipe.SnapshotModifier {
	out := modifier
	out.IngredientUsage = cloneUsage(modifier.IngredientUsage)
	out.IngredientUnits = cloneUnits(modifier.IngredientUnits)
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

func cloneUnits(units map[string]ingredient.Unit) map[string]ingredient.Unit {
	if len(units) == 0 {
		return nil
	}

	out := make(map[string]ingredient.Unit, len(units))
	for ingredientID, unit := range units {
		out[ingredientID] = unit
	}
	return out
}

func mergeUnits(dst map[string]ingredient.Unit, src map[string]ingredient.Unit) {
	if len(src) == 0 {
		return
	}
	for ingredientID, unit := range src {
		dst[ingredientID] = unit
	}
}

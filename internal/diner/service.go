package diner

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

const TokenTTL = 24 * time.Hour

type PublicOrderItem struct {
	ItemID string
	LineID string
	Name   string
	Macros ingredient.MacroValues
}

type ReceiptToken struct {
	Token     string
	OrderID   string
	ExpiresAt time.Time
	Items     []PublicOrderItem
}

type Claim struct {
	ID              string
	Token           string
	SelectedItemIDs []string
	Totals          ingredient.MacroValues
	UpdatedAt       time.Time
}

type Service struct {
	mu     sync.Mutex
	tokens map[string]ReceiptToken
	claims map[string]Claim
	now    func() time.Time
	rand   io.Reader
}

func NewService() *Service {
	return &Service{
		tokens: map[string]ReceiptToken{},
		claims: map[string]Claim{},
		now:    func() time.Time { return time.Now().UTC() },
		rand:   rand.Reader,
	}
}

func (s *Service) IssueToken(order contracts.ClosedOrder) (ReceiptToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw := make([]byte, 18)
	if _, err := io.ReadFull(s.rand, raw); err != nil {
		return ReceiptToken{}, err
	}

	var items []PublicOrderItem
	for _, line := range order.Lines {
		if line.Quantity <= 0 {
			return ReceiptToken{}, fmt.Errorf("line %s has invalid quantity %d", line.LineID, line.Quantity)
		}

		perItemMacros := line.ResolvedMacros.Scale(1 / float64(line.Quantity))
		for i := 1; i <= line.Quantity; i++ {
			itemID := line.LineID
			if line.Quantity > 1 {
				itemID = fmt.Sprintf("%s#%d", line.LineID, i)
			}

			items = append(items, PublicOrderItem{
				ItemID: itemID,
				LineID: line.LineID,
				Name:   line.Name,
				Macros: perItemMacros,
			})
		}
	}

	token := ReceiptToken{
		Token:     base64.RawURLEncoding.EncodeToString(raw),
		OrderID:   order.OrderID,
		ExpiresAt: s.now().Add(TokenTTL),
		Items:     items,
	}
	s.tokens[token.Token] = token
	return token, nil
}

func (s *Service) ResolveToken(token string) (ReceiptToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.tokens[token]
	if !ok {
		return ReceiptToken{}, fmt.Errorf("token not found")
	}
	if s.now().After(stored.ExpiresAt) {
		return ReceiptToken{}, fmt.Errorf("token expired")
	}
	return cloneToken(stored), nil
}

func (s *Service) SubmitClaim(claimID string, token string, selectedItemIDs []string) (Claim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.tokens[token]
	if !ok {
		return Claim{}, fmt.Errorf("token not found")
	}
	if s.now().After(stored.ExpiresAt) {
		return Claim{}, fmt.Errorf("token expired")
	}

	selected := map[string]bool{}
	totals := ingredient.MacroValues{}
	for _, itemID := range selectedItemIDs {
		if selected[itemID] {
			return Claim{}, fmt.Errorf("item %s selected multiple times", itemID)
		}
		selected[itemID] = true

		found := false
		for _, item := range stored.Items {
			if item.ItemID != itemID {
				continue
			}
			totals = totals.Add(item.Macros)
			found = true
			break
		}
		if !found {
			return Claim{}, fmt.Errorf("item %s not found for token", itemID)
		}
	}

	if existing, ok := s.claims[claimID]; ok && existing.Token != token {
		return Claim{}, fmt.Errorf("claim %s belongs to a different token", claimID)
	}

	claim := Claim{
		ID:              claimID,
		Token:           token,
		SelectedItemIDs: append([]string(nil), selectedItemIDs...),
		Totals:          totals,
		UpdatedAt:       s.now(),
	}
	s.claims[claimID] = claim
	return claim, nil
}

func cloneToken(token ReceiptToken) ReceiptToken {
	out := token
	out.Items = append([]PublicOrderItem(nil), token.Items...)
	return out
}

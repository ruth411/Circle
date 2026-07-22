package diner

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/ordering"
)

const TokenTTL = 24 * time.Hour

type PublicOrderItem struct {
	LineID   string
	Name     string
	Quantity int
	Macros   ingredient.MacroValues
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
	SelectedLineIDs []string
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

func (s *Service) IssueToken(order ordering.Order) (ReceiptToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if order.Status != ordering.OrderStatusClosed {
		return ReceiptToken{}, fmt.Errorf("order %s must be closed before issuing a token", order.ID)
	}

	raw := make([]byte, 18)
	if _, err := io.ReadFull(s.rand, raw); err != nil {
		return ReceiptToken{}, err
	}

	items := make([]PublicOrderItem, len(order.Lines))
	for i, line := range order.Lines {
		items[i] = PublicOrderItem{
			LineID:   line.LineID,
			Name:     line.Name,
			Quantity: line.Quantity,
			Macros:   line.ResolvedMacros,
		}
	}

	token := ReceiptToken{
		Token:     base64.RawURLEncoding.EncodeToString(raw),
		OrderID:   order.ID,
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

func (s *Service) SubmitClaim(claimID string, token string, selectedLineIDs []string) (Claim, error) {
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
	for _, lineID := range selectedLineIDs {
		if selected[lineID] {
			return Claim{}, fmt.Errorf("line %s selected multiple times", lineID)
		}
		selected[lineID] = true

		found := false
		for _, item := range stored.Items {
			if item.LineID != lineID {
				continue
			}
			totals = totals.Add(item.Macros)
			found = true
			break
		}
		if !found {
			return Claim{}, fmt.Errorf("line %s not found for token", lineID)
		}
	}

	if existing, ok := s.claims[claimID]; ok && existing.Token != token {
		return Claim{}, fmt.Errorf("claim %s belongs to a different token", claimID)
	}

	claim := Claim{
		ID:              claimID,
		Token:           token,
		SelectedLineIDs: append([]string(nil), selectedLineIDs...),
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

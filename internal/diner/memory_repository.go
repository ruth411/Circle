package diner

import (
	"context"
	"sync"
)

type memoryRepository struct {
	mu            sync.Mutex
	tokensByValue map[string]ReceiptToken
	orderTokens   map[string]string
	claims        map[string]Claim
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		tokensByValue: map[string]ReceiptToken{},
		orderTokens:   map[string]string{},
		claims:        map[string]Claim{},
	}
}

func (r *memoryRepository) GetToken(_ context.Context, token string) (ReceiptToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, ok := r.tokensByValue[token]
	if !ok {
		return ReceiptToken{}, ErrTokenNotFound
	}
	return cloneToken(stored), nil
}

func (r *memoryRepository) GetTokenByOrder(_ context.Context, locationID string, orderID string) (ReceiptToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tokenValue, ok := r.orderTokens[locationID+"|"+orderID]
	if !ok {
		return ReceiptToken{}, ErrTokenNotFound
	}
	return cloneToken(r.tokensByValue[tokenValue]), nil
}

func (r *memoryRepository) CreateToken(_ context.Context, token ReceiptToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	orderKey := token.LocationID + "|" + token.OrderID
	if existingToken, ok := r.orderTokens[orderKey]; ok {
		if existingToken == token.Token {
			return nil
		}
		return ErrTokenAlreadyExists
	}
	if _, ok := r.tokensByValue[token.Token]; ok {
		return ErrTokenAlreadyExists
	}

	r.tokensByValue[token.Token] = cloneToken(token)
	r.orderTokens[orderKey] = token.Token
	return nil
}

func (r *memoryRepository) GetClaim(_ context.Context, claimID string) (Claim, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, ok := r.claims[claimID]
	if !ok {
		return Claim{}, ErrClaimNotFound
	}
	return cloneClaim(stored), nil
}

func (r *memoryRepository) CreateClaim(_ context.Context, claim Claim) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.claims[claim.ID]; ok {
		return ErrClaimAlreadyExists
	}
	r.claims[claim.ID] = cloneClaim(claim)
	return nil
}

func (r *memoryRepository) UpdateClaim(_ context.Context, claim Claim) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.claims[claim.ID]; !ok {
		return ErrClaimNotFound
	}
	r.claims[claim.ID] = cloneClaim(claim)
	return nil
}

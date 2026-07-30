package diner

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapClaimItemInsertErrorTranslatesOnlyTokenItemFK(t *testing.T) {
	err := mapClaimItemInsertError(&pgconn.PgError{
		Code:           "23503",
		ConstraintName: "diner_claim_items_token_item_fk",
	}, "item-1")
	if !errors.Is(err, ErrInvalidClaim) {
		t.Fatalf("err = %v, want ErrInvalidClaim", err)
	}
}

func TestMapClaimItemInsertErrorLeavesOtherErrorsAlone(t *testing.T) {
	other := errors.New("db unavailable")
	if got := mapClaimItemInsertError(other, "item-1"); !errors.Is(got, other) {
		t.Fatalf("err = %v, want original error", got)
	}

	err := mapClaimItemInsertError(&pgconn.PgError{
		Code:           "23503",
		ConstraintName: "some_other_fk",
	}, "item-1")
	if errors.Is(err, ErrInvalidClaim) {
		t.Fatalf("err = %v, did not want ErrInvalidClaim", err)
	}
}

package diner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) GetToken(ctx context.Context, token string) (ReceiptToken, error) {
	return r.loadToken(ctx, `
SELECT
    token,
    order_id,
    location_id,
    closed_at,
    expires_at
FROM diner.receipt_tokens
WHERE token = $1;
`, token)
}

func (r *SQLRepository) GetTokenByOrder(ctx context.Context, locationID string, orderID string) (ReceiptToken, error) {
	return r.loadToken(ctx, `
SELECT
    token,
    order_id,
    location_id,
    closed_at,
    expires_at
FROM diner.receipt_tokens
WHERE location_id = $1
  AND order_id = $2;
`, locationID, orderID)
}

func (r *SQLRepository) loadToken(ctx context.Context, query string, args ...any) (ReceiptToken, error) {
	var token ReceiptToken
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&token.Token,
		&token.OrderID,
		&token.LocationID,
		&token.ClosedAt,
		&token.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ReceiptToken{}, ErrTokenNotFound
	}
	if err != nil {
		return ReceiptToken{}, err
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT
    item_id,
    line_id,
    name,
    calories,
    protein_grams,
    carbs_grams,
    fat_grams
FROM diner.receipt_token_items
WHERE token = $1
ORDER BY ordinal, item_id;
`, token.Token)
	if err != nil {
		return ReceiptToken{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var item PublicOrderItem
		if err := rows.Scan(
			&item.ItemID,
			&item.LineID,
			&item.Name,
			&item.Macros.Calories,
			&item.Macros.ProteinGrams,
			&item.Macros.CarbsGrams,
			&item.Macros.FatGrams,
		); err != nil {
			return ReceiptToken{}, err
		}
		token.Items = append(token.Items, item)
	}
	if err := rows.Err(); err != nil {
		return ReceiptToken{}, err
	}
	return token, nil
}

func (r *SQLRepository) CreateToken(ctx context.Context, token ReceiptToken) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx, `
INSERT INTO diner.receipt_tokens (
    token,
    order_id,
    location_id,
    closed_at,
    expires_at,
    created_at
)
VALUES ($1, $2, $3, $4, $5, $6);
`, token.Token, token.OrderID, token.LocationID, token.ClosedAt.UTC(), token.ExpiresAt.UTC(), time.Now().UTC())
	if err != nil {
		if isUniqueViolation(err) {
			return ErrTokenAlreadyExists
		}
		return err
	}

	for ordinal, item := range token.Items {
		_, err := tx.ExecContext(ctx, `
INSERT INTO diner.receipt_token_items (
    location_id,
    token,
    item_id,
    line_id,
    ordinal,
    name,
    calories,
    protein_grams,
    carbs_grams,
    fat_grams,
    created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
`, token.LocationID, token.Token, item.ItemID, item.LineID, ordinal+1, item.Name, item.Macros.Calories, item.Macros.ProteinGrams, item.Macros.CarbsGrams, item.Macros.FatGrams, time.Now().UTC())
		if err != nil {
			if isUniqueViolation(err) {
				return ErrTokenAlreadyExists
			}
			return err
		}
	}

	return tx.Commit()
}

func (r *SQLRepository) GetClaim(ctx context.Context, claimID string) (Claim, error) {
	var claim Claim
	err := r.db.QueryRowContext(ctx, `
SELECT
    id,
    token,
    location_id,
    total_calories,
    total_protein_grams,
    total_carbs_grams,
    total_fat_grams,
    created_at,
    updated_at
FROM diner.claims
WHERE id = $1;
`, claimID).Scan(
		&claim.ID,
		&claim.Token,
		&claim.LocationID,
		&claim.Totals.Calories,
		&claim.Totals.ProteinGrams,
		&claim.Totals.CarbsGrams,
		&claim.Totals.FatGrams,
		&claim.CreatedAt,
		&claim.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Claim{}, ErrClaimNotFound
	}
	if err != nil {
		return Claim{}, err
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT item_id
FROM diner.claim_items
WHERE claim_id = $1
ORDER BY created_at, item_id;
`, claimID)
	if err != nil {
		return Claim{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			return Claim{}, err
		}
		claim.SelectedItemIDs = append(claim.SelectedItemIDs, itemID)
	}
	if err := rows.Err(); err != nil {
		return Claim{}, err
	}
	return claim, nil
}

func (r *SQLRepository) CreateClaim(ctx context.Context, claim Claim) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := upsertClaimRows(ctx, tx, claim, true); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLRepository) UpdateClaim(ctx context.Context, claim Claim) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM diner.claim_items WHERE claim_id = $1;`, claim.ID); err != nil {
		return err
	}
	if err := upsertClaimRows(ctx, tx, claim, false); err != nil {
		return err
	}
	return tx.Commit()
}

func upsertClaimRows(ctx context.Context, tx *sql.Tx, claim Claim, create bool) error {
	now := time.Now().UTC()
	if create {
		_, err := tx.ExecContext(ctx, `
INSERT INTO diner.claims (
    id,
    token,
    location_id,
    total_calories,
    total_protein_grams,
    total_carbs_grams,
    total_fat_grams,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
`, claim.ID, claim.Token, claim.LocationID, claim.Totals.Calories, claim.Totals.ProteinGrams, claim.Totals.CarbsGrams, claim.Totals.FatGrams, claim.CreatedAt.UTC(), claim.UpdatedAt.UTC())
		if err != nil {
			if isUniqueViolation(err) {
				return ErrClaimAlreadyExists
			}
			return err
		}
	} else {
		result, err := tx.ExecContext(ctx, `
UPDATE diner.claims
SET token = $2,
    location_id = $3,
    total_calories = $4,
    total_protein_grams = $5,
    total_carbs_grams = $6,
    total_fat_grams = $7,
    updated_at = $8
WHERE id = $1;
`, claim.ID, claim.Token, claim.LocationID, claim.Totals.Calories, claim.Totals.ProteinGrams, claim.Totals.CarbsGrams, claim.Totals.FatGrams, claim.UpdatedAt.UTC())
		if err != nil {
			return err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return ErrClaimNotFound
		}
	}

	for _, itemID := range claim.SelectedItemIDs {
		_, err := tx.ExecContext(ctx, `
INSERT INTO diner.claim_items (
    location_id,
    claim_id,
    token,
    item_id,
    created_at
)
VALUES ($1, $2, $3, $4, $5);
`, claim.LocationID, claim.ID, claim.Token, itemID, now)
		if err != nil {
			return mapClaimItemInsertError(err, itemID)
		}
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func mapClaimItemInsertError(err error, itemID string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" && (pgErr.ConstraintName == "diner_claim_items_token_item_fk" || pgErr.ConstraintName == "diner_claim_items_location_token_item_fk") {
		return fmt.Errorf("%w: item %s does not belong to token", ErrInvalidClaim, itemID)
	}
	return err
}

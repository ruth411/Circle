package tenancy

import (
	"context"
	"database/sql"
	"errors"
)

type SQLOrganizationResolver struct {
	db *sql.DB
}

func NewSQLOrganizationResolver(db *sql.DB) *SQLOrganizationResolver {
	return &SQLOrganizationResolver{db: db}
}

func (r *SQLOrganizationResolver) OrganizationIDForLocation(ctx context.Context, locationID string) (string, error) {
	const query = `
SELECT restaurants.organization_id
FROM tenancy.locations AS locations
JOIN tenancy.restaurants AS restaurants
    ON restaurants.id = locations.restaurant_id
WHERE locations.id = $1;
`

	var organizationID string
	err := r.db.QueryRowContext(ctx, query, locationID).Scan(&organizationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrLocationNotFound
		}
		return "", err
	}

	return organizationID, nil
}

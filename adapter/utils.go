package adapter

import (
	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/jackc/pgx/v5"
)

// convertDBErr converts database errors to appropriate domain errors
func convertDBErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

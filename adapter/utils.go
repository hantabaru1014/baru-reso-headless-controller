package adapter

import (
	"encoding/hex"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// convertDBErr converts database errors to appropriate domain errors.
func convertDBErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}

	return err
}

// textFromPtr は *string を pgtype.Text に変換する (nil なら Valid=false).
func textFromPtr(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}

	return pgtype.Text{String: *p, Valid: true}
}

// parseUUID は canonical UUID 文字列 ("xxxxxxxx-xxxx-...") を pgtype.UUID に変換する.
func parseUUID(id string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(id); err != nil {
		return pgtype.UUID{}, errors.WrapPrefix(err, "invalid uuid", 0)
	}

	return u, nil
}

// canonicalUUIDLen は ハイフン付き UUID 文字列の長さ (8-4-4-4-12).
const canonicalUUIDLen = 36

// formatUUID は pgtype.UUID を canonical 文字列に整形する.
func formatUUID(u pgtype.UUID) (string, error) {
	if !u.Valid {
		return "", errors.New("invalid uuid")
	}

	src := u.Bytes[:]
	dst := make([]byte, canonicalUUIDLen)
	hex.Encode(dst[0:8], src[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], src[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], src[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], src[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], src[10:16])

	return string(dst), nil
}

package adapter

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ port.ResoniteVersionRepository = (*ResoniteVersionRepository)(nil)

type ResoniteVersionRepository struct {
	q *db.Queries
}

func NewResoniteVersionRepository(q *db.Queries) *ResoniteVersionRepository {
	return &ResoniteVersionRepository{q: q}
}

func (r *ResoniteVersionRepository) Upsert(ctx context.Context, params port.ResoniteVersionUpsertParams) (*entity.ResoniteVersion, error) {
	row, err := r.q.UpsertResoniteVersion(ctx, db.UpsertResoniteVersionParams{
		ManifestID:  params.ManifestID,
		Branch:      string(params.Branch),
		GameVersion: textFromPtr(params.GameVersion),
		ReleasedAt:  pgtype.Timestamptz{Time: params.ReleasedAt, Valid: true},
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "resonite_version", 0)
	}

	return resoniteVersionToEntity(row), nil
}

func (r *ResoniteVersionRepository) Get(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch) (*entity.ResoniteVersion, error) {
	row, err := r.q.GetResoniteVersion(ctx, db.GetResoniteVersionParams{
		ManifestID: manifestID,
		Branch:     string(branch),
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "resonite_version", 0)
	}

	return resoniteVersionToEntity(row), nil
}

func (r *ResoniteVersionRepository) GetByImageTag(ctx context.Context, imageTag string) (*entity.ResoniteVersion, error) {
	row, err := r.q.GetResoniteVersionByImageTag(ctx, pgtype.Text{String: imageTag, Valid: true})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "resonite_version", 0)
	}

	return resoniteVersionToEntity(row), nil
}

func (r *ResoniteVersionRepository) List(ctx context.Context, branch *entity.ResoniteVersionBranch) (entity.ResoniteVersionList, error) {
	var arg pgtype.Text
	if branch != nil {
		arg = pgtype.Text{String: string(*branch), Valid: true}
	}

	rows, err := r.q.ListResoniteVersions(ctx, arg)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "resonite_version", 0)
	}

	result := make(entity.ResoniteVersionList, 0, len(rows))
	for _, row := range rows {
		result = append(result, resoniteVersionToEntity(row))
	}

	return result, nil
}

func (r *ResoniteVersionRepository) GetLatestByBranch(ctx context.Context, branch entity.ResoniteVersionBranch) (*entity.ResoniteVersion, error) {
	row, err := r.q.GetLatestResoniteVersionByBranch(ctx, string(branch))
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "resonite_version", 0)
	}

	return resoniteVersionToEntity(row), nil
}

func (r *ResoniteVersionRepository) ListStaleBuilt(ctx context.Context, branches []entity.ResoniteVersionBranch, currentAppVersion string) (entity.ResoniteVersionList, error) {
	branchStrs := make([]string, 0, len(branches))
	for _, b := range branches {
		branchStrs = append(branchStrs, string(b))
	}

	rows, err := r.q.ListStaleBuiltResoniteVersions(ctx, db.ListStaleBuiltResoniteVersionsParams{
		Branches:          branchStrs,
		CurrentAppVersion: currentAppVersion,
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "resonite_version", 0)
	}

	result := make(entity.ResoniteVersionList, 0, len(rows))
	// CTE 経由なので sqlc は db.ResoniteVersion ではなく専用 Row 型を生成する.
	// カラムは同一なので詰め替えて共通の変換に渡す.
	for _, row := range rows {
		result = append(result, resoniteVersionToEntity(db.ResoniteVersion(row)))
	}

	return result, nil
}

func (r *ResoniteVersionRepository) SetBuilding(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch) error {
	if _, err := r.q.SetResoniteVersionBuilding(ctx, db.SetResoniteVersionBuildingParams{
		ManifestID: manifestID,
		Branch:     string(branch),
	}); err != nil {
		return errors.WrapPrefix(convertDBErr(err), "resonite_version", 0)
	}

	return nil
}

func (r *ResoniteVersionRepository) SetBuilt(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch, imageTag string, builtWithAppVersion string) error {
	if _, err := r.q.SetResoniteVersionBuilt(ctx, db.SetResoniteVersionBuiltParams{
		ManifestID:          manifestID,
		Branch:              string(branch),
		ImageTag:            imageTag,
		BuiltWithAppVersion: builtWithAppVersion,
	}); err != nil {
		return errors.WrapPrefix(convertDBErr(err), "resonite_version", 0)
	}

	return nil
}

func (r *ResoniteVersionRepository) SetFailed(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch, errMessage string) error {
	if _, err := r.q.SetResoniteVersionFailed(ctx, db.SetResoniteVersionFailedParams{
		ManifestID: manifestID,
		Branch:     string(branch),
		BuildError: errMessage,
	}); err != nil {
		return errors.WrapPrefix(convertDBErr(err), "resonite_version", 0)
	}

	return nil
}

func resoniteVersionToEntity(s db.ResoniteVersion) *entity.ResoniteVersion {
	return &entity.ResoniteVersion{
		ManifestID:          s.ManifestID,
		Branch:              entity.ResoniteVersionBranch(s.Branch),
		GameVersion:         ptrFromText(s.GameVersion),
		ReleasedAt:          s.ReleasedAt.Time,
		BuildStatus:         entity.ResoniteVersionBuildStatus(s.BuildStatus),
		ImageTag:            ptrFromText(s.ImageTag),
		BuiltWithAppVersion: ptrFromText(s.BuiltWithAppVersion),
		BuiltAt:             ptrFromTimestamptz(s.BuiltAt),
		BuildError:          ptrFromText(s.BuildError),
		CreatedAt:           s.CreatedAt.Time,
		UpdatedAt:           s.UpdatedAt.Time,
	}
}

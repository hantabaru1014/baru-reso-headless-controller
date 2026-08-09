package port

import (
	"context"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
)

type ResoniteVersionUpsertParams struct {
	ManifestID  string
	Branch      entity.ResoniteVersionBranch
	GameVersion *string
	ReleasedAt  time.Time
}

// ResoniteVersionRepository は resonite_versions テーブルの永続化を担う.
type ResoniteVersionRepository interface {
	// Upsert は versions.json の 1 エントリを流し込む. build_status / image_tag /
	// built_with_app_version 等の build 側の状態は保持する.
	Upsert(ctx context.Context, params ResoniteVersionUpsertParams) (*entity.ResoniteVersion, error)

	// Get は (manifest_id, branch) の複合キーで 1 件取る.
	Get(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch) (*entity.ResoniteVersion, error)

	// GetByImageTag は built 済みイメージタグから逆引きする. 未 built のタグは nil を返さない (ErrNotFound).
	GetByImageTag(ctx context.Context, imageTag string) (*entity.ResoniteVersion, error)

	// GetLatestByBranch はブランチの最新版を返す (build 状態は問わない).
	// game_version が null の行は除外.
	GetLatestByBranch(ctx context.Context, branch entity.ResoniteVersionBranch) (*entity.ResoniteVersion, error)

	// List は branch を指定して (nil なら全件) 新しい順に返す.
	List(ctx context.Context, branch *entity.ResoniteVersionBranch) (entity.ResoniteVersionList, error)

	// ListStaleBuilt は branches の各ブランチの最新 built 行のうち、built_with_app_version が
	// currentAppVersion と一致しないものを返す (最大 len(branches) 件).
	// branches は絞り込み対象 (通常 headless + prerelease).
	ListStaleBuilt(ctx context.Context, branches []entity.ResoniteVersionBranch, currentAppVersion string) (entity.ResoniteVersionList, error)

	// SetBuilding は build_status を building に遷移させ、build_error をクリアする.
	SetBuilding(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch) error

	// SetBuilt は build_status を built に遷移させ、image_tag / built_with_app_version を書き込む.
	SetBuilt(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch, imageTag string, builtWithAppVersion string) error

	// SetFailed は build_status を failed に遷移させ、build_error を書き込む.
	SetFailed(ctx context.Context, manifestID string, branch entity.ResoniteVersionBranch, errMessage string) error
}

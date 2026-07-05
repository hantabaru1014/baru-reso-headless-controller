package entity

import "time"

// ResoniteVersionBranch は Steam の Resonite における branch 名 (versions.json 由来).
type ResoniteVersionBranch string

const (
	ResoniteVersionBranch_Public         ResoniteVersionBranch = "public"
	ResoniteVersionBranch_Prerelease     ResoniteVersionBranch = "prerelease"
	ResoniteVersionBranch_Release        ResoniteVersionBranch = "release"
	ResoniteVersionBranch_Headless       ResoniteVersionBranch = "headless"
	ResoniteVersionBranch_PreSplittening ResoniteVersionBranch = "pre-splittening"
)

// AutoBuildBranches は auto-build 対象のブランチ集合.
var AutoBuildBranches = []ResoniteVersionBranch{
	ResoniteVersionBranch_Headless,
	ResoniteVersionBranch_Prerelease,
}

// ResoniteVersionBuildStatus は resonite_versions.build_status の値.
type ResoniteVersionBuildStatus string

const (
	ResoniteVersionBuildStatus_NotBuilt ResoniteVersionBuildStatus = "not_built"
	ResoniteVersionBuildStatus_Building ResoniteVersionBuildStatus = "building"
	ResoniteVersionBuildStatus_Built    ResoniteVersionBuildStatus = "built"
	ResoniteVersionBuildStatus_Failed   ResoniteVersionBuildStatus = "failed"
)

type ResoniteVersion struct {
	ManifestID          string
	Branch              ResoniteVersionBranch
	GameVersion         *string
	ReleasedAt          time.Time
	BuildStatus         ResoniteVersionBuildStatus
	ImageTag            *string
	BuiltWithAppVersion *string
	BuiltAt             *time.Time
	BuildError          *string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ResoniteVersionList []*ResoniteVersion

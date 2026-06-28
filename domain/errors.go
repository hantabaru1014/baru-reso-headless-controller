package domain

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrUnauthenticated  = errors.New("unauthenticated")
	ErrPermissionDenied = errors.New("permission denied")
)

// SystemUserID は CLI / 内部 worker が「特定の利用者を持たない操作」を行うときの
// 実行主体ユーザー ID. マイグレーション 20260628120000_seed_system_user で
// users / group_members に投入されている.
const SystemUserID = "system"

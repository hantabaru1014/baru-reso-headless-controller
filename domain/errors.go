package domain

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrUnauthenticated  = errors.New("unauthenticated")
	ErrPermissionDenied = errors.New("permission denied")
)

// DetailedError は「一行のエラーメッセージとは別に、長い詳細 (ビルドログ等) を
// 併せ持つエラー」. async_job worker が errors.As で拾い、詳細を async_job_logs に
// 保存して job 履歴のエラー詳細ダイアログから参照できるようにする.
type DetailedError interface {
	error
	// ErrorDetail は詳細本文を返す. 空文字なら詳細なしとして扱われる.
	ErrorDetail() string
}

// SystemUserID は CLI / 内部 worker が「特定の利用者を持たない操作」を行うときの
// 実行主体ユーザー ID. マイグレーション 20260628120000_seed_system_user で
// users / group_members に投入されている.
const SystemUserID = "system"

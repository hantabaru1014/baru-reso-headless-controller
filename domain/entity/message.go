package entity

import "time"

// Message は管理者から利用者へのお知らせメッセージ (掲示板).
// 本文は markdown. 既読管理やリアルタイム通知は行わない.
type Message struct {
	ID    string
	Title string
	// markdown 本文.
	Body string
	// GroupID が nil なら全員向け. 設定時はそのグループのメンバー向け.
	GroupID *string
	// GroupName は表示用に解決済みのグループ名 (GroupID 設定時のみ).
	GroupName *string
	// CreatedBy / LastUpdatedBy は nil の場合削除済みユーザー.
	CreatedBy     *string
	LastUpdatedBy *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type MessageList []*Message

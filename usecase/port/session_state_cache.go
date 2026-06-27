package port

import (
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

// SessionStateCache は container が権威の揮発な session 現在状態を
// controller プロセス内に保持する in-memory cache。DB は session の
// 永続情報 (status / lifecycle / startup_parameters) を持ち、
// CurrentState (UsersCount / WorldUrl / 起動経過時間 等の刻々と変化する
// snapshot) はこの cache が持つ。
//
// host から流れる SessionParametersChanged / WorldSaved / SessionEnded
// event で更新される。GetSession RPC で cache miss した場合は container
// から取り直して populate する。
type SessionStateCache interface {
	Get(sessionID string) (*headlessv1.Session, bool)
	Set(sessionID string, snapshot *headlessv1.Session)
	Delete(sessionID string)
}

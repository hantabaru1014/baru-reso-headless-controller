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
//
// 各 entry は hostID と一緒に保存する: HostEventWatcher の OutOfRange
// stream reset 時に SessionStateSyncHandler が PruneHost を呼んで、当該
// host から消えた session の cache entry をまとめて掃除できるようにする
// ため。session は host を跨いで移動しうるので、同じ sessionID に対し
// 後発の Set(hostID2, ...) が来たら所属 host も書き換わる。
//
// Pointer aliasing 契約: Set に渡した *Session pointer はそのまま保持
// され、Get もそれをそのまま返す。caller は受け取った snapshot を
// mutate してはならない (mutate したい場合は proto.Clone してから
// Set し直す)。
type SessionStateCache interface {
	Get(sessionID string) (*headlessv1.Session, bool)
	Set(hostID, sessionID string, snapshot *headlessv1.Session)
	Delete(sessionID string)
	// PruneHost は指定 host の cache entry のうち liveSessionIDs に含まれない
	// ものをまとめて削除する。stream reset 後の再構築フェーズで host から
	// 消えた session を一掃するために使う。liveSessionIDs が nil/空なら当該
	// host の全 entry を削除する。
	PruneHost(hostID string, liveSessionIDs map[string]struct{})
}

-- 当初 sessions.current_state JSONB 列を追加する予定だったが、CurrentState は
-- container が権威の揮発状態と再設計し、controller 側は in-memory cache に
-- 移行した。本マイグレーションは PR レビュー中に no-op 化したもの。実際の
-- 列ドロップは後続の 20260627085354_drop_current_state_from_sessions が
-- IF EXISTS で行う。
SELECT 1;

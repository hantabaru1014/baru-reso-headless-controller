-- 20260627085354 が IF EXISTS で同じ列を drop しているので二重 drop しても safe。
-- 両 down 後に必ず column が無い状態になるよう対称化。
ALTER TABLE sessions DROP COLUMN IF EXISTS current_state;

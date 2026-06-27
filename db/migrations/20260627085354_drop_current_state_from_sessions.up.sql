-- CurrentState は揮発な session 現在状態であり container が権威。controller 側は
-- in-memory cache に持つ設計に変更したため列を撤去する。IF EXISTS で再 apply 安全。
ALTER TABLE sessions DROP COLUMN IF EXISTS current_state;

-- registration_tokens に personal_role_id 列を追加.
-- 招待発行時に system-admin が指定したロールを永続化し、登録時にトークンと
-- 一緒に lookup する. URL クエリ経由で改竄される経路を塞ぐ.
-- NULL は「seed-admin (デフォルト)」と解釈する.
ALTER TABLE registration_tokens
    ADD COLUMN personal_role_id TEXT REFERENCES roles(id) ON DELETE SET NULL;

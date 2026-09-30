-- ヘッドレスアカウントの一意性を「全体で resonite_id が 1 件」から
-- 「グループごとに resonite_id が 1 件」へ変更する.
-- 同一の Resonite アカウントを複数グループに登録できるようになる.
-- hosts.account_id は (hosts.group_id, hosts.account_id) の組で
-- headless_accounts (group_id, resonite_id) を指す (同一グループ制約).
ALTER TABLE headless_accounts DROP CONSTRAINT headless_accounts_pkey;
ALTER TABLE headless_accounts ADD PRIMARY KEY (group_id, resonite_id);

-- group_id 単体の絞り込みは新しい PK (group_id 先頭) で賄えるため不要になる.
DROP INDEX idx_headless_accounts_group_id;
CREATE INDEX idx_headless_accounts_resonite_id ON headless_accounts(resonite_id);

-- 旧仕様 (resonite_id が全体で一意) では、アカウントを別グループへ登録し直すと、元のグループに
-- 残ったホストが別グループのアカウント登録を参照し続けていた. 新仕様ではホストは自グループの
-- 登録しか参照しないため、そうしたホストが引き続き再起動できるよう、ホストのグループにも
-- アカウント登録を複製する (この時点では resonite_id ごとに 1 行なので複製元は一意に決まる).
INSERT INTO headless_accounts (resonite_id, credential, password, last_display_name, last_icon_url, group_id, created_by)
SELECT DISTINCT a.resonite_id, a.credential, a.password, a.last_display_name, a.last_icon_url, h.group_id, a.created_by
FROM hosts h
INNER JOIN headless_accounts a ON a.resonite_id = h.account_id
WHERE h.group_id <> a.group_id
ON CONFLICT DO NOTHING;

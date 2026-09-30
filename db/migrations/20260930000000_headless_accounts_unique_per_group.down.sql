-- 同一 resonite_id が複数グループに登録されていると PK を resonite_id 単独へ戻せない
-- (up の複製や、適用後の運用で重複登録が生じうる). 重複を解消してから実行すること.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM headless_accounts GROUP BY resonite_id HAVING COUNT(*) > 1) THEN
        RAISE EXCEPTION 'cannot revert: some resonite_id is registered in multiple groups';
    END IF;
END $$;

DROP INDEX idx_headless_accounts_resonite_id;
CREATE INDEX idx_headless_accounts_group_id ON headless_accounts(group_id);

ALTER TABLE headless_accounts DROP CONSTRAINT headless_accounts_pkey;
ALTER TABLE headless_accounts ADD PRIMARY KEY (resonite_id);

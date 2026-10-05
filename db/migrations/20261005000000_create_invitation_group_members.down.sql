DROP TABLE IF EXISTS invitation_group_members;

ALTER TABLE registration_tokens DROP CONSTRAINT IF EXISTS registration_tokens_id_key;
ALTER TABLE registration_tokens DROP COLUMN IF EXISTS id;

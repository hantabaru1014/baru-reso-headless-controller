ALTER TABLE role_permissions DISABLE TRIGGER protect_builtin_role_permissions_trg;

DELETE FROM role_permissions
WHERE role_id = 'seed-system-admin' AND permission_key = 'system:message.manage';

ALTER TABLE role_permissions ENABLE TRIGGER protect_builtin_role_permissions_trg;

DROP TABLE messages;

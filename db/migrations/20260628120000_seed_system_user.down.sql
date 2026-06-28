DELETE FROM group_members WHERE group_id = 'system' AND user_id = 'system';
-- up.sql で password='' で seed しているため、ログイン可能な正規 user とは確実に区別できる.
-- pre-existing で id='system' な正規 user (password が bcrypt ハッシュ) があった場合に
-- 誤って消さないため password 条件を付ける.
DELETE FROM users WHERE id = 'system' AND password = '';

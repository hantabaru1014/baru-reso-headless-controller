-- system ユーザー投稿の「アプリの使い方」お知らせメッセージを seed する。

-- created_by の FK 先である system ユーザーを冪等に確保する。
-- (テスト DB 等で users が truncate された後に再適用しても失敗しないように、
--  20260628120000_seed_system_user.up.sql と同じ内容を ON CONFLICT 付きで再投入する)
INSERT INTO users (id, password, resonite_id, icon_url, created_at, updated_at)
VALUES ('system', '', NULL, NULL, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO messages (id, title, body, group_id, created_by, last_updated_by)
VALUES (
    'seed-usage-guide',
    'はじめに: セッションを建てるまでの流れ',
    E'このアプリでヘッドレスサーバーを立ち上げ、セッションを開くまでの流れを紹介します。\n\n## 1. ヘッドレスアカウントを追加する\n\nまず、セッションのホストに使う Resonite アカウントを [アカウント](/headlessAccounts) ページから登録します。\n\n- ヘッドレスアカウントにサブスクリプションは必須ではありません(メインのアカウントで権利を持っていればいいです)\n- 普段使いのアカウントとは別に、ヘッドレス専用のアカウントを新規作成しましょう\n\n## 2. ホストを作成する\n\n[ホスト](/hosts) ページから、登録したアカウントを指定してホストを起動します。\n\nホストは「通常のヘッドレスクライアント 1 プロセス」に相当する概念です。実体は Docker コンテナとして起動され、このアプリから停止・再起動・ログ閲覧などの管理ができます。\n\n## 3. セッションを建てる\n\n起動したホスト上でセッション (ワールド) を開始します。 [セッション](/sessions) ページからワールドを指定して開始してください。1 つのホストで複数のセッションを同時にホストできます。\n\n## グループについて\n\nホスト・アカウント・セッションはすべていずれかの **グループ** に所属します。グループは権限管理の単位で、同じグループのメンバーとリソースを共有できます。\n\n- 自分専用の personal グループが最初から用意されているので、個人利用ならそのままで OK です\n- 複数人で管理したい場合はグループを作成してメンバーを招待しましょう\n- メンバーごとにロール (admin / user / session-operator など) を割り当てて、できる操作を制御できます',
    NULL,
    'system',
    'system'
)
ON CONFLICT (id) DO NOTHING;

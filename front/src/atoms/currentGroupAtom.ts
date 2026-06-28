import { atomWithStorage } from "jotai/utils";
import { localStorageKeyPrefix } from "./shared";

/**
 * ヘッダーで選択中の「コンテキストグループ」.
 * 各リスト画面はこの値を group_id としてリクエストに含め、表示対象を絞り込む.
 *
 * - `null`: アクセス可能な全グループを対象 (リクエストに group_id を含めない)
 * - グループ ID: そのグループに所属するリソースのみ
 *
 * localStorage に永続化されるため、ページリロード後も維持される.
 */
export const currentGroupIdAtom = atomWithStorage<string | null>(
  `${localStorageKeyPrefix}currentGroupId`,
  null,
);

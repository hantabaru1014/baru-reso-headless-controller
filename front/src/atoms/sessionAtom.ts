import { atom } from "jotai";
import { atomWithStorage } from "jotai/utils";
import { localStorageKeyPrefix } from "./shared";

export interface UserInfo {
  name?: string;
  email?: string;
  image?: string;
}

export interface Session {
  token: string;
  user: UserInfo;
}

export const sessionAtom = atom<Session | null>(null);
export const sessionTokenAtom = atom((get) => get(sessionAtom)?.token ?? null);
export const sessionRefreshTokenAtom = atomWithStorage<string | null>(
  `${localStorageKeyPrefix}sessionRefreshToken`,
  null,
);

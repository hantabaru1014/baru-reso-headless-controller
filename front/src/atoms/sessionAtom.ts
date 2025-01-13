import { atom } from "jotai";

export interface Session {
  token: string;
  user: {
    name?: string;
    email?: string;
    image?: string;
  };
}

export const sessionAtom = atom<Session | null>(null);

import { atom } from "jotai";

export interface SelectedHost {
  id: string;
  name: string;
}

export const selectedHostAtom = atom<SelectedHost | null>(null);

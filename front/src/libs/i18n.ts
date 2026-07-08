import i18n from "i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import { initReactI18next } from "react-i18next";

import en from "@/locales/en.json";
import ja from "@/locales/ja.json";

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      ja: { translation: ja },
      en: { translation: en },
    },
    fallbackLng: "en",
    supportedLngs: ["ja", "en"],
    load: "languageOnly",
    interpolation: {
      // React が escape するので二重 escape を避ける
      escapeValue: false,
    },
    detection: {
      order: ["localStorage", "navigator"],
      caches: ["localStorage"],
    },
  });

i18n.on("languageChanged", (lng) => {
  document.documentElement.lang = lng;
});
// resources が同期ロードのため初回の languageChanged は上の登録より先に発火する
if (i18n.resolvedLanguage) {
  document.documentElement.lang = i18n.resolvedLanguage;
}

export default i18n;

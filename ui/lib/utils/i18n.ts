import 'dayjs/locale/zh'

import dayjs from 'dayjs'
import i18next from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import distro from '@lib/distribution.json'

i18next.on('languageChanged', function (lng) {
  dayjs.locale(lng.toLowerCase())
})

export function addTranslations(requireContext) {
  if (typeof requireContext === 'object') {
    Object.keys(requireContext).forEach((key) => {
      const translations = requireContext[key]
      addTranslationResource(key, translations)
    })
    return
  }

  const keys = requireContext.keys()
  keys.forEach((key) => {
    const m = key.match(/\/(.+)\.yaml/)
    if (!m) {
      return
    }
    const lang = m[1]
    const translations = requireContext(key)
    addTranslationResource(lang, translations)
  })
}

export function addTranslationResource(lang, translations) {
  i18next.addResourceBundle(lang, 'translation', translations, true, false)
}

export const ALL_LANGUAGES = {
  zh: '简体中文',
  en: 'English',
}

i18next
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      en: {
        translation: {
          distro,
        },
      },
    },
    fallbackLng: 'en', // fallbackLng won't change the detected language
    whitelist: ['zh', 'en'], // whitelist will change the detected lanuage
    interpolation: {
      escapeValue: false,
      defaultVariables: { distro },
    },
  })

const isDistro = process.env.REACT_APP_DISTRO_BUILD_TAG !== undefined

export { distro, isDistro }

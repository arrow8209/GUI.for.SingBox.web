import { createI18n } from 'vue-i18n'

import { ReadFile } from '@/bridge'
import { LocalesFilePath } from '@/constant/app'
import { Lang } from '@/enums/app'

import en from './locale/en'
import zh from './locale/zh'

const messages: Recordable = {
  zh,
  en,
}

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackWarn: false,
  missingWarn: false,
  messages,
})

export const loadLocale = async (locale = i18n.global.locale.value) => {
  if (![Lang.ZH, Lang.EN].includes(locale as Lang)) {
    const message = await ReadFile(`${LocalesFilePath}/${locale}.json`).catch(() => '')
    message && i18n.global.setLocaleMessage(locale, JSON.parse(message))
  }
}

// 上游 v1.20 用 loadLocaleMessages/reloadLocale，本地等价别名
export const loadLocaleMessages = loadLocale
export const reloadLocale = () => loadLocale()

export default i18n

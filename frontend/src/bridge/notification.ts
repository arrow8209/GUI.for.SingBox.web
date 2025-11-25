import { message } from '@/utils'

interface NotifyOptions {
  silent?: boolean
}

export const Notify = async (
  title: string,
  body: string,
  icon = '',
  options: NotifyOptions = {},
) => {
  if (!('Notification' in window)) {
    message.info(`${title}: ${body}`)
    return
  }

  let permission = Notification.permission
  if (permission === 'default') {
    permission = await Notification.requestPermission()
  }

  if (permission !== 'granted') {
    message.info(`${title}: ${body}`)
    return
  }

  try {
    new Notification(title, {
      body,
      icon: formatIcon(icon),
      silent: options.silent,
    })
  } catch (err) {
    console.warn('Notification error', err)
    message.info(`${title}: ${body}`)
  }
}

const formatIcon = (icon: string) => {
  if (!icon) return undefined
  if (/^https?:\/\//i.test(icon)) return icon
  return `${window.location.origin}/${icon.replace(/^\//, '')}`
}

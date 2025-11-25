export const BrowserOpenURL = (url: string) => {
  if (!/^https?:\/\//i.test(url)) {
    url = 'http://' + url
  }
  window.open(url, '_blank', 'noopener')
}

export const ClipboardSetText = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(textarea)
    return ok
  }
}

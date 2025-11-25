let maximised = false

export const WindowSetAlwaysOnTop = (_pinned: boolean) => {
  // Not supported in browser mode.
}

export const WindowHide = () => {
  window.blur()
}

export const WindowMinimise = () => {
  window.blur()
}

export const WindowSetSize = (_width: number, _height: number) => {
  // Not supported in browser mode.
}

export const WindowReloadApp = () => {
  window.location.reload()
}

export const WindowToggleMaximise = () => {
  maximised = !maximised
  document.body.classList.toggle('app-maximised', maximised)
}

export const WindowIsMaximised = async () => maximised

export const WindowIsMinimised = async () => false

export const WindowSetSystemDefaultTheme = () => {
  document.body.removeAttribute('data-theme')
}

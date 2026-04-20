import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { closeEvents } from '@/bridge/events'
import router from '@/router'

const CSRF_KEY = 'csrf_token'
const API_BASE = import.meta.env.VITE_API_BASE || '/api'

export const useAuthStore = defineStore('auth', () => {
  const csrfToken = ref(sessionStorage.getItem(CSRF_KEY) || '')
  const mustChangePassword = ref(false)
  const loading = ref(false)
  const error = ref('')

  const isAuthenticated = computed(() => !!csrfToken.value)

  const setCsrf = (value: string) => {
    csrfToken.value = value
    if (value) {
      sessionStorage.setItem(CSRF_KEY, value)
    } else {
      sessionStorage.removeItem(CSRF_KEY)
    }
  }

  const login = async (username: string, password: string) => {
    loading.value = true
    error.value = ''
    try {
      const res = await fetch(`${API_BASE}/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ username, password }),
      })
      if (!res.ok) {
        const text = await res.text()
        try {
          const j = JSON.parse(text)
          throw new Error(j.error || 'login failed')
        } catch {
          throw new Error(text || 'login failed')
        }
      }
      const data = await res.json()
      setCsrf(data.csrfToken)
      mustChangePassword.value = !!data.mustChangePassword
      if (mustChangePassword.value) {
        router.replace('/change-password')
      } else {
        router.replace('/')
      }
    } catch (e: any) {
      error.value = e.message || 'login failed'
      throw e
    } finally {
      loading.value = false
    }
  }

  const logout = async () => {
    try {
      await fetch(`${API_BASE}/logout`, {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrfToken.value },
        credentials: 'include',
      })
    } catch {
      // ignore
    }
    setCsrf('')
    mustChangePassword.value = false
    closeEvents()
    router.replace('/login')
  }

  const forceLogout = () => {
    setCsrf('')
    mustChangePassword.value = false
    closeEvents()
    router.replace('/login')
  }

  const changePassword = async (oldPassword: string, newPassword: string) => {
    const res = await fetch(`${API_BASE}/change-password`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken.value,
      },
      credentials: 'include',
      body: JSON.stringify({ oldPassword, newPassword }),
    })
    if (!res.ok) {
      const text = await res.text()
      try {
        const j = JSON.parse(text)
        throw new Error(j.error || 'change password failed')
      } catch {
        throw new Error(text || 'change password failed')
      }
    }
    mustChangePassword.value = false
    router.replace('/')
  }

  return {
    csrfToken,
    mustChangePassword,
    loading,
    error,
    isAuthenticated,
    login,
    logout,
    forceLogout,
    changePassword,
  }
})

export const getStoredCsrf = () => sessionStorage.getItem(CSRF_KEY) || ''

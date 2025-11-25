import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import router from '@/router'
import { closeEvents } from '@/bridge/events'

const TOKEN_KEY = 'auth_token'
const API_BASE = import.meta.env.VITE_API_BASE || '/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(TOKEN_KEY) || '')
  const loading = ref(false)
  const error = ref('')

  const isAuthenticated = computed(() => !!token.value)

  const setToken = (value: string) => {
    token.value = value
    if (value) {
      localStorage.setItem(TOKEN_KEY, value)
    } else {
      localStorage.removeItem(TOKEN_KEY)
    }
  }

  const login = async (username: string, password: string) => {
    loading.value = true
    error.value = ''
    try {
      const res = await fetch(`${API_BASE}/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      })
      if (!res.ok) {
        const msg = await res.text()
        throw new Error(msg || 'login failed')
      }
      const data = await res.json()
      setToken(data.token)
      router.replace('/')
    } catch (e: any) {
      error.value = e.message || 'login failed'
      throw e
    } finally {
      loading.value = false
    }
  }

  const logout = async () => {
    try {
      if (token.value) {
        await fetch(`${API_BASE}/logout`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${token.value}` },
        })
      }
    } catch {
      // ignore
    }
    setToken('')
    closeEvents()
    router.replace('/login')
  }

  const forceLogout = () => {
    setToken('')
    closeEvents()
    router.replace('/login')
  }

  return { token, loading, error, isAuthenticated, login, logout, forceLogout }
})

export const getStoredToken = () => localStorage.getItem(TOKEN_KEY) || ''

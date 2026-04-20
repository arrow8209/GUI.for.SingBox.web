<script lang="ts" setup>
import { ref } from 'vue'

import { useAuthStore } from '@/stores/auth'
import { message } from '@/utils'

const authStore = useAuthStore()

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)

const submit = async () => {
  if (newPassword.value.length < 8) {
    message.error('Password must be at least 8 characters')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    message.error('Passwords do not match')
    return
  }
  loading.value = true
  try {
    await authStore.changePassword(oldPassword.value, newPassword.value)
    message.success('Password changed successfully')
  } catch (e: any) {
    message.error(e?.message || String(e) || 'change failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex items-center justify-center min-h-screen p-32">
    <form
      class="w-320 flex flex-col gap-12 p-24 rounded-8 bg-white shadow"
      @submit.prevent="submit"
    >
      <h2 class="text-20 font-bold">Change Password</h2>
      <p v-if="authStore.mustChangePassword" class="text-orange text-12">
        First-time setup: please change the initial password before continuing.
      </p>
      <input
        v-model="oldPassword"
        type="password"
        placeholder="Current password"
        autocomplete="current-password"
        class="px-12 py-8 border rounded-4"
      />
      <input
        v-model="newPassword"
        type="password"
        placeholder="New password (min 8 characters)"
        autocomplete="new-password"
        class="px-12 py-8 border rounded-4"
      />
      <input
        v-model="confirmPassword"
        type="password"
        placeholder="Confirm new password"
        autocomplete="new-password"
        class="px-12 py-8 border rounded-4"
      />
      <button
        :disabled="loading"
        type="submit"
        class="px-12 py-8 bg-blue text-white rounded-4 disabled:opacity-50"
      >
        {{ loading ? 'Submitting…' : 'Change password' }}
      </button>
    </form>
  </div>
</template>

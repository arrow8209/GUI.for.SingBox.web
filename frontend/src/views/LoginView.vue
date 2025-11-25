<script setup lang="ts">
import { reactive } from 'vue'

import { useAuthStore } from '@/stores'
import { message } from '@/utils'

const authStore = useAuthStore()

const form = reactive({
  username: '',
  password: '',
})

const handleSubmit = async () => {
  if (!form.username || !form.password) {
    message.warn('请输入用户名和密码')
    return
  }
  try {
    await authStore.login(form.username, form.password)
  } catch (err: any) {
    message.error(err.message || err)
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <h1>登录 GUI.for.SingBox</h1>
      <div class="form-item">
        <label>用户名</label>
        <Input v-model="form.username" placeholder="用户名" />
      </div>
      <div class="form-item">
        <label>密码</label>
        <Input v-model="form.password" placeholder="密码" type="password" />
      </div>
      <Button type="primary" :loading="authStore.loading" @click="handleSubmit" block>
        登录
      </Button>
      <div v-if="authStore.error" class="error">{{ authStore.error }}</div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f7f7fb;
}
.login-card {
  width: 320px;
  padding: 32px;
  border-radius: 16px;
  background: #fff;
  box-shadow: 0 10px 40px rgba(15, 23, 42, 0.1);
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.form-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.error {
  color: #e63946;
  font-size: 14px;
}
</style>

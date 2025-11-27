<script lang="ts" setup>
import { reactive } from 'vue'
import { useI18n } from 'vue-i18n'

import { ClipboardSetText, Exec, GetRealityPublicKey } from '@/bridge'
import { DraggableOptions } from '@/constant/app'
import { CoreWorkingDirectory, TunStackOptions, ShadowsocksMethodOptions } from '@/constant/kernel'
import {
  DefaultInboundMixed,
  DefaultInboundHttp,
  DefaultInboundSocks,
  DefaultInboundTun,
  DefaultInboundShadowsocks,
  DefaultInboundCustom,
} from '@/constant/profile'
import { Inbound } from '@/enums/kernel'
import { getKernelFileName, message, picker, sampleID } from '@/utils'

const model = defineModel<IProfile['inbounds']>({ required: true })

const { t } = useI18n()
const shadowsocksGenerating = reactive<Record<string, boolean>>({})

const handleDelete = (index: number) => {
  model.value.splice(index, 1)
}

const SHADOWSOCKS_2022_KEY_SIZES: Record<string, number> = {
  '2022-blake3-aes-128-gcm': 16,
  '2022-blake3-aes-256-gcm': 32,
  '2022-blake3-chacha20-poly1305': 32,
}

const getShadowsocksKeyBytes = (method: string) => SHADOWSOCKS_2022_KEY_SIZES[method] || 32

const fallbackBase64 = (bytes: number) => {
  const array = new Uint8Array(bytes)
  crypto.getRandomValues(array)
  let binary = ''
  array.forEach((b) => {
    binary += String.fromCharCode(b)
  })
  return btoa(binary)
}

const runSingBoxRand = async (bytes: number) => {
  const binary = `${CoreWorkingDirectory}/${getKernelFileName()}`
  try {
    const output = await Exec(binary, ['generate', 'rand', '--base64', String(bytes)])
    const trimmed = output.trim()
    if (trimmed) return trimmed
    throw new Error('empty output')
  } catch (error) {
    console.error('[inbounds] failed to run sing-box generate rand', error)
    message.error(t('kernel.inbounds.shadowsocks.generateFailed'))
    return fallbackBase64(bytes)
  }
}

const generateShadowsocksPassword = async (method: string) => {
  const bytes = getShadowsocksKeyBytes(method)
  return runSingBoxRand(bytes)
}

const inbounds = [
  {
    label: 'Mixed',
    value: () => {
      model.value.push({
        id: sampleID(),
        tag: 'mixed-in',
        type: Inbound.Mixed,
        enable: true,
        mixed: DefaultInboundMixed(),
      })
    },
  },
  {
    label: 'Http',
    value: () => {
      model.value.push({
        id: sampleID(),
        tag: 'http-in',
        type: Inbound.Http,
        enable: true,
        http: DefaultInboundHttp(),
      })
    },
  },
  {
    label: 'Socks',
    value: () => {
      model.value.push({
        id: sampleID(),
        tag: 'socks-in',
        type: Inbound.Socks,
        enable: true,
        socks: DefaultInboundSocks(),
      })
    },
  },
  {
    label: 'Tun',
    value: () => {
      model.value.push({
        id: sampleID(),
        tag: 'tun-in',
        type: Inbound.Tun,
        enable: true,
        tun: DefaultInboundTun(),
      })
    },
  },
  {
    label: 'Shadowsocks',
    value: () => {
      const inbound: IInbound = {
        id: sampleID(),
        tag: 'ss-in',
        type: Inbound.Shadowsocks,
        enable: true,
        shadowsocks: DefaultInboundShadowsocks(),
      }
      model.value.push(inbound)
      refreshShadowsocksPassword(inbound)
    },
  },
  {
    label: 'Custom JSON',
    value: () => {
      model.value.push({
        id: sampleID(),
        tag: 'custom-in',
        type: Inbound.Custom,
        enable: true,
        custom: DefaultInboundCustom(),
      })
    },
  },
]

const handleAdd = async () => {
  const fns = await picker.multi('common.add', inbounds)
  fns.forEach((fn) => fn())
}

const pickHost = (...hosts: (string | undefined)[]) => {
  return hosts.find((host) => typeof host === 'string' && host.trim().length > 0)?.trim() || ''
}

const ensurePort = (port?: number) => {
  if (!port || port <= 0) {
    throw new Error(t('kernel.inbounds.exportMissingPort'))
  }
  return port
}

const ensureUser = (users?: string[]) => {
  const value = users?.find((user) => user.trim().length > 0)
  if (!value) {
    throw new Error(t('kernel.inbounds.exportMissingUser'))
  }
  return value
}

const createQueryString = (params: Record<string, string | number | undefined>) => {
  const search = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === '') return
    search.set(key, String(value))
  })
  return search.toString()
}

const buildVlessRealityLink = async (inbound: IInbound) => {
  const details = inbound.vless
  if (!details?.tls.reality.enabled) throw new Error(t('kernel.inbounds.exportUnsupported'))

  const uuid = ensureUser(details.users)
  const host = pickHost(details.tls.reality.handshake.server, details.listen.listen)
  if (!host) throw new Error(t('kernel.inbounds.exportMissingServer'))

  const port = ensurePort(details.tls.reality.handshake.server_port || details.listen.listen_port)
  const privateKey = details.tls.reality.private_key.trim()
  if (!privateKey) throw new Error(t('kernel.inbounds.exportMissingPrivateKey'))

  const publicKey = await GetRealityPublicKey(privateKey)
  const shortId = details.tls.reality.short_id.find((id) => id.trim().length > 0) || ''
  const query = createQueryString({
    encryption: 'none',
    flow: 'xtls-rprx-vision',
    security: 'reality',
    sni: pickHost(details.tls.server_name, host),
    pbk: publicKey,
    sid: shortId,
    type: 'tcp',
  })
  const fragment = encodeURIComponent(inbound.tag || 'vless')
  const base = `vless://${uuid}@${host}:${port}`
  return `${base}${query ? `?${query}` : ''}#${fragment}`
}

const buildTrojanTLSLink = (inbound: IInbound) => {
  const details = inbound.trojan
  if (!details?.tls.enabled) throw new Error(t('kernel.inbounds.exportUnsupported'))
  const password = ensureUser(details.users)
  const host = pickHost(details.tls.server_name, details.listen.listen)
  if (!host) throw new Error(t('kernel.inbounds.exportMissingServer'))
  const port = ensurePort(details.listen.listen_port)
  const query = createQueryString({
    security: 'tls',
    sni: pickHost(details.tls.server_name, host),
    alpn: details.tls.alpn.length ? details.tls.alpn.join(',') : undefined,
    type: 'tcp',
  })
  const fragment = encodeURIComponent(inbound.tag || 'trojan')
  const base = `trojan://${encodeURIComponent(password)}@${host}:${port}`
  return `${base}${query ? `?${query}` : ''}#${fragment}`
}

const buildShareLink = async (inbound: IInbound) => {
  if (inbound.type === Inbound.VLESS) {
    return buildVlessRealityLink(inbound)
  }
  if (inbound.type === Inbound.Trojan) {
    return buildTrojanTLSLink(inbound)
  }
  throw new Error(t('kernel.inbounds.exportUnsupported'))
}

const canExportInbound = (inbound: IInbound) => {
  if (inbound.type === Inbound.VLESS) {
    return Boolean(inbound.vless?.tls.reality.enabled)
  }
  if (inbound.type === Inbound.Trojan) {
    return Boolean(inbound.trojan?.tls.enabled)
  }
  return false
}

const handleExport = async (inbound: IInbound) => {
  try {
    const link = await buildShareLink(inbound)
    const copied = await ClipboardSetText(link)
    if (!copied) throw new Error('ClipboardSetText Error')
    message.success(t('common.copied'))
  } catch (error: any) {
    message.error(error?.message || error || t('common.error'))
  }
}

const refreshShadowsocksPassword = async (inbound: IInbound) => {
  if (!inbound.shadowsocks) return
  const key = inbound.id
  shadowsocksGenerating[key] = true
  try {
    const password = await generateShadowsocksPassword(inbound.shadowsocks.method)
    inbound.shadowsocks.password = password
  } finally {
    shadowsocksGenerating[key] = false
  }
}

const handleShadowsocksMethodChange = async (inbound: IInbound, method: string) => {
  if (!inbound.shadowsocks) return
  inbound.shadowsocks.method = method
  await refreshShadowsocksPassword(inbound)
}

const getCustomError = (inbound: IInbound) => {
  if (inbound.type !== Inbound.Custom || !inbound.custom) return ''
  const json = inbound.custom.content || ''
  if (!json.trim()) {
    return t('kernel.inbounds.custom.placeholder')
  }
  try {
    const parsed = JSON.parse(json)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return t('kernel.inbounds.custom.mustObject')
    }
    return ''
  } catch (error: any) {
    return error?.message || String(error)
  }
}

defineExpose({ handleAdd })
</script>

<template>
  <Empty v-if="model.length === 0">
    <template #description>
      <div class="flex gap-8">
        <Button v-for="inbound in inbounds" :key="inbound.label" @click="inbound.value">
          {{ t('common.add') }} {{ inbound.label }}
        </Button>
      </div>
    </template>
  </Empty>
  <div v-draggable="[model, { ...DraggableOptions, handle: '.drag' }]">
    <Card v-for="(inbound, index) in model" :key="inbound.id" :title="inbound.tag" class="mb-8">
      <template #title-prefix>
        <Icon icon="drag" class="drag cursor-move" />
      </template>
      <template #extra>
        <div class="flex gap-4">
          <Button
            v-if="canExportInbound(inbound)"
            @click="handleExport(inbound)"
            icon="link"
            type="text"
            size="small"
            v-tips="'kernel.inbounds.export'"
          />
          <Button @click="handleDelete(index)" icon="delete" type="text" size="small" />
        </div>
      </template>
      <div class="form-item">
        <span class="form-label">{{ t('kernel.inbounds.enable') }}</span>
        <div class="form-value">
          <Switch v-model="inbound.enable" />
        </div>
      </div>
      <div class="form-item">
        <span class="form-label">{{ t('kernel.inbounds.tag') }}</span>
        <div class="form-value">
          <Input v-model="inbound.tag" />
        </div>
      </div>
      <div
        v-if="
          inbound.type !== Inbound.Tun &&
          inbound.type !== Inbound.Custom &&
          inbound[inbound.type]
        "
      >
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.listen.listen') }}</span>
          <div class="form-value">
            <Input v-model="inbound[inbound.type]!.listen.listen" />
          </div>
        </div>
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.listen.listen_port') }}</span>
          <div class="form-value">
            <Input v-model="inbound[inbound.type]!.listen.listen_port" type="number" />
          </div>
        </div>
        <div
          v-if="inbound.type === Inbound.Mixed || inbound.type === Inbound.Http || inbound.type === Inbound.Socks"
          :class="{ 'items-start': inbound[inbound.type]!.users.length }"
          class="form-item"
        >
          <span class="form-label">{{ t('kernel.inbounds.users') }}</span>
          <div class="form-value">
            <InputList
              v-model="inbound[inbound.type]!.users"
              :placeholder="t('kernel.inbounds.usersPlaceholder')"
            />
          </div>
        </div>
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.listen.tcp_fast_open') }}</span>
          <div class="form-value">
            <Switch v-model="inbound[inbound.type]!.listen.tcp_fast_open" />
          </div>
        </div>
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.listen.tcp_multi_path') }}</span>
          <div class="form-value">
            <Switch v-model="inbound[inbound.type]!.listen.tcp_multi_path" />
          </div>
        </div>
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.listen.udp_fragment') }}</span>
          <div class="form-value">
            <Switch v-model="inbound[inbound.type]!.listen.udp_fragment" />
          </div>
        </div>
        <template v-if="inbound.type === Inbound.Shadowsocks && inbound.shadowsocks">
          <div class="form-item">
            <span class="form-label">{{ t('kernel.inbounds.shadowsocks.method') }}</span>
            <div class="form-value">
              <Select
                v-model="inbound.shadowsocks.method"
                :options="ShadowsocksMethodOptions"
                :border="false"
                auto-size
                @change="(value) => handleShadowsocksMethodChange(inbound, value as string)"
              />
            </div>
          </div>
          <div class="form-item">
            <span class="form-label">{{ t('kernel.inbounds.shadowsocks.password') }}</span>
            <div class="form-value flex items-center gap-4 flex-wrap justify-end">
              <Input v-model="inbound.shadowsocks.password" type="text" class="ss-password-input" />
              <Button
                icon="refresh"
                type="text"
                size="small"
                :loading="shadowsocksGenerating[inbound.id]"
                :disabled="shadowsocksGenerating[inbound.id]"
                @click="() => refreshShadowsocksPassword(inbound)"
                v-tips="t('common.refresh')"
              />
            </div>
          </div>
        </template>
      </div>
      <div v-else-if="inbound.type === Inbound.Tun && inbound.tun">
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.tun.interface_name') }}</span>
          <div class="form-value">
            <Input v-model="inbound.tun.interface_name" editable />
          </div>
        </div>
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.tun.stack') }}</span>
          <div class="form-value">
            <Radio v-model="inbound.tun.stack" :options="TunStackOptions" />
          </div>
        </div>
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.tun.auto_route') }}</span>
          <div class="form-value">
            <Switch v-model="inbound.tun.auto_route" />
          </div>
        </div>
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.tun.strict_route') }}</span>
          <div class="form-value">
            <Switch v-model="inbound.tun.strict_route" />
          </div>
        </div>
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.tun.endpoint_independent_nat') }}</span>
          <div class="form-value">
            <Switch v-model="inbound.tun.endpoint_independent_nat" />
          </div>
        </div>
        <div class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.tun.mtu') }}</span>
          <div class="form-value">
            <Input v-model="inbound.tun.mtu" type="number" editable />
          </div>
        </div>
        <div :class="{ 'items-start': inbound.tun.address.length }" class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.tun.address') }}</span>
          <div class="form-value">
            <InputList v-model="inbound.tun.address" />
          </div>
        </div>
        <div :class="{ 'items-start': inbound.tun.route_address.length }" class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.tun.route_address') }}</span>
          <div class="form-value">
            <InputList v-model="inbound.tun.route_address" placeholder="0.0.0.0/1 ::1" />
          </div>
        </div>
        <div :class="{ 'items-start': inbound.tun.route_exclude_address.length }" class="form-item">
          <span class="form-label">{{ t('kernel.inbounds.tun.route_exclude_address') }}</span>
          <div class="form-value">
            <InputList
              v-model="inbound.tun.route_exclude_address"
              placeholder="192.168.0.0/16 fc00::/7"
            />
          </div>
        </div>
      </div>
      <div v-else-if="inbound.type === Inbound.Custom && inbound.custom">
        <div class="form-item flex-col items-start w-full">
          <div class="w-full mb-4">{{ t('kernel.inbounds.custom.label') }}</div>
          <Input
            v-model="inbound.custom.content"
            class="w-full"
            type="code"
            lang="json"
            editable
            :placeholder="t('kernel.inbounds.custom.placeholder')"
          />
          <p v-if="getCustomError(inbound)" class="text-error text-sm mt-2">
            {{ getCustomError(inbound) }}
          </p>
        </div>
      </div>
    </Card>
  </div>
</template>

<style scoped>
.ss-password-input {
  max-width: 320px;
}
</style>

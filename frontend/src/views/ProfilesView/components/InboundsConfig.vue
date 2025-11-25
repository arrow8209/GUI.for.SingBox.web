<script lang="ts" setup>
import { useI18n } from 'vue-i18n'

import { DraggableOptions } from '@/constant/app'
import { TunStackOptions } from '@/constant/kernel'
import {
  DefaultInboundMixed,
  DefaultInboundHttp,
  DefaultInboundSocks,
  DefaultInboundTun,
  DefaultInboundVless,
  DefaultInboundTrojan,
} from '@/constant/profile'
import { Inbound } from '@/enums/kernel'
import { picker, sampleID } from '@/utils'

const model = defineModel<IProfile['inbounds']>({ required: true })

const { t } = useI18n()

const handleDelete = (index: number) => {
  model.value.splice(index, 1)
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
    label: 'VLESS Reality',
    value: () => {
      model.value.push({
        id: sampleID(),
        tag: 'vless-reality-in',
        type: Inbound.VLESS,
        enable: true,
        vless: DefaultInboundVless(),
      })
    },
  },
  {
    label: 'Trojan TLS',
    value: () => {
      model.value.push({
        id: sampleID(),
        tag: 'trojan-tls-in',
        type: Inbound.Trojan,
        enable: true,
        trojan: DefaultInboundTrojan(),
      })
    },
  },
]

const handleAdd = async () => {
  const fns = await picker.multi('common.add', inbounds)
  fns.forEach((fn) => fn())
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
        <Button @click="handleDelete(index)" icon="delete" type="text" size="small" />
      </template>
      <div class="form-item">
        {{ t('kernel.inbounds.enable') }}
        <Switch v-model="inbound.enable" />
      </div>
      <div class="form-item">
        {{ t('kernel.inbounds.tag') }}
        <Input v-model="inbound.tag" />
      </div>
      <div v-if="inbound.type !== Inbound.Tun && inbound[inbound.type]">
        <div class="form-item">
          {{ t('kernel.inbounds.listen.listen') }}
          <Input v-model="inbound[inbound.type]!.listen.listen" />
        </div>
        <div class="form-item">
          {{ t('kernel.inbounds.listen.listen_port') }}
          <Input v-model="inbound[inbound.type]!.listen.listen_port" type="number" />
        </div>
        <div :class="{ 'items-start': inbound[inbound.type]!.users.length }" class="form-item">
          {{ t('kernel.inbounds.users') }}
          <InputList
            v-model="inbound[inbound.type]!.users"
            :placeholder="
              inbound.type === Inbound.VLESS
                ? t('kernel.inbounds.vless.uuidPlaceholder')
                : inbound.type === Inbound.Trojan
                  ? t('kernel.inbounds.trojan.passwordPlaceholder')
                  : 'user:password'
            "
          />
        </div>
        <div class="form-item">
          {{ t('kernel.inbounds.listen.tcp_fast_open') }}
          <Switch v-model="inbound[inbound.type]!.listen.tcp_fast_open" />
        </div>
        <div class="form-item">
          {{ t('kernel.inbounds.listen.tcp_multi_path') }}
          <Switch v-model="inbound[inbound.type]!.listen.tcp_multi_path" />
        </div>
        <div class="form-item">
          {{ t('kernel.inbounds.listen.udp_fragment') }}
          <Switch v-model="inbound[inbound.type]!.listen.udp_fragment" />
        </div>
        <template v-if="inbound.type === Inbound.VLESS && inbound.vless">
          <div class="form-item">
            {{ t('kernel.inbounds.vless.serverName') }}
            <Input v-model="inbound.vless.tls.server_name" />
          </div>
          <div class="form-item">
            {{ t('kernel.inbounds.vless.handshakeServer') }}
            <Input v-model="inbound.vless.tls.reality.handshake.server" />
          </div>
          <div class="form-item">
            {{ t('kernel.inbounds.vless.handshakePort') }}
            <Input v-model="inbound.vless.tls.reality.handshake.server_port" type="number" />
          </div>
          <div class="form-item">
            {{ t('kernel.inbounds.vless.privateKey') }}
            <Input v-model="inbound.vless.tls.reality.private_key" editable />
          </div>
          <div :class="{ 'items-start': inbound.vless.tls.reality.short_id.length }" class="form-item">
            {{ t('kernel.inbounds.vless.shortId') }}
            <InputList v-model="inbound.vless.tls.reality.short_id" placeholder="short id" />
          </div>
        </template>
        <template v-else-if="inbound.type === Inbound.Trojan && inbound.trojan">
          <div class="form-item">
            {{ t('kernel.inbounds.trojan.serverName') }}
            <Input v-model="inbound.trojan.tls.server_name" />
          </div>
          <div :class="{ 'items-start': inbound.trojan.tls.alpn.length }" class="form-item">
            {{ t('kernel.inbounds.trojan.alpn') }}
            <InputList v-model="inbound.trojan.tls.alpn" placeholder="h2" />
          </div>
          <div class="form-item">
            {{ t('kernel.inbounds.trojan.minVersion') }}
            <Input v-model="inbound.trojan.tls.min_version" />
          </div>
          <div class="form-item">
            {{ t('kernel.inbounds.trojan.maxVersion') }}
            <Input v-model="inbound.trojan.tls.max_version" />
          </div>
        </template>
      </div>
      <div v-else-if="inbound.type === Inbound.Tun && inbound.tun">
        <div class="form-item">
          {{ t('kernel.inbounds.tun.interface_name') }}
          <Input v-model="inbound.tun.interface_name" editable />
        </div>
        <div class="form-item">
          {{ t('kernel.inbounds.tun.stack') }}
          <Radio v-model="inbound.tun.stack" :options="TunStackOptions" />
        </div>
        <div class="form-item">
          {{ t('kernel.inbounds.tun.auto_route') }}
          <Switch v-model="inbound.tun.auto_route" />
        </div>
        <div class="form-item">
          {{ t('kernel.inbounds.tun.strict_route') }}
          <Switch v-model="inbound.tun.strict_route" />
        </div>
        <div class="form-item">
          {{ t('kernel.inbounds.tun.endpoint_independent_nat') }}
          <Switch v-model="inbound.tun.endpoint_independent_nat" />
        </div>
        <div class="form-item">
          {{ t('kernel.inbounds.tun.mtu') }}
          <Input v-model="inbound.tun.mtu" type="number" editable />
        </div>
        <div :class="{ 'items-start': inbound.tun.address.length }" class="form-item">
          {{ t('kernel.inbounds.tun.address') }}
          <InputList v-model="inbound.tun.address" />
        </div>
        <div :class="{ 'items-start': inbound.tun.route_address.length }" class="form-item">
          {{ t('kernel.inbounds.tun.route_address') }}
          <InputList v-model="inbound.tun.route_address" placeholder="0.0.0.0/1 ::1" />
        </div>
        <div :class="{ 'items-start': inbound.tun.route_exclude_address.length }" class="form-item">
          {{ t('kernel.inbounds.tun.route_exclude_address') }}
          <InputList
            v-model="inbound.tun.route_exclude_address"
            placeholder="192.168.0.0/16 fc00::/7"
          />
        </div>
      </div>
    </Card>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'

const state = ref(null)
const feed = ref([])
const online = ref(false)

async function refresh() {
  try {
    const s = await fetch('/api/state').then(r => r.json())
    state.value = s
    online.value = true
    if (Array.isArray(s.feed)) feed.value = s.feed.slice(-200).reverse()
  } catch {
    online.value = false
  }
}

async function toggle(name, on) {
  if (state.value) state.value.modules[name] = on
  await fetch('/api/toggle', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ name, on }),
  })
}

onMounted(() => {
  refresh()
  setInterval(refresh, 5000)
  const es = new EventSource('/api/feed')
  es.onmessage = (ev) => {
    feed.value.unshift(JSON.parse(ev.data))
    if (feed.value.length > 200) feed.value.length = 200
  }
})
</script>

<template>
  <v-app>
    <v-app-bar flat density="comfortable" color="background">
      <v-app-bar-title class="font-weight-medium">
        Eclipse <span class="text-medium-emphasis font-weight-regular">proxy</span>
      </v-app-bar-title>
      <v-chip size="small" :color="online ? 'success' : 'secondary'" variant="tonal">
        {{ online ? 'online' : 'offline' }}
      </v-chip>
    </v-app-bar>

    <v-main>
      <v-container style="max-width: 720px">
        <div class="text-overline text-medium-emphasis mb-2">Route</div>
        <v-card rounded="lg" class="mb-6 pa-4">
          <code>{{ state?.listen ?? '—' }} → {{ state?.server ?? '—' }}</code>
        </v-card>

        <div class="text-overline text-medium-emphasis mb-2">Players</div>
        <v-card rounded="lg" class="mb-6">
          <v-list density="compact" bg-color="transparent" v-if="state?.players?.length">
            <v-list-item v-for="p in state.players" :key="p">
              <code>{{ p }}</code>
            </v-list-item>
          </v-list>
          <div v-else class="pa-4 text-medium-emphasis">nobody connected</div>
        </v-card>

        <div class="text-overline text-medium-emphasis mb-2">Modules</div>
        <v-card rounded="lg" class="mb-6">
          <v-list density="compact" bg-color="transparent" v-if="state?.modules">
            <v-list-item v-for="(on, name) in state.modules" :key="name">
              <v-list-item-title>{{ name }}</v-list-item-title>
              <template #append>
                <v-switch
                  :model-value="on"
                  color="primary"
                  density="compact"
                  hide-details
                  inset
                  @update:model-value="toggle(name, $event)"
                />
              </template>
            </v-list-item>
          </v-list>
        </v-card>

        <div class="text-overline text-medium-emphasis mb-2">Packets</div>
        <v-card rounded="lg" class="pa-3">
          <div v-if="!feed.length" class="pa-2 text-medium-emphasis">waiting for traffic…</div>
          <div v-else style="max-height: 320px; overflow-y: auto">
            <div v-for="(e, i) in feed" :key="i" class="d-flex ga-3 py-1">
              <code :style="{ color: e.dir === 'up' ? '#60a5fa' : '#a78bfa', width: '38px' }">
                {{ e.dir === 'up' ? 'C→S' : 'S→C' }}
              </code>
              <code class="text-high-emphasis">{{ e.pkt }}</code>
            </div>
          </div>
        </v-card>
      </v-container>
    </v-main>
  </v-app>
</template>

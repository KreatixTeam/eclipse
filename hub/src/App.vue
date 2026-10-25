<script setup>
import { computed, onMounted, ref } from 'vue'

const account = ref(localStorage.getItem('eclipse-account') || '')
const servers = ref([])
const login = ref(null) // {id, code, uri}
const loginError = ref('')
const adding = ref(false)
const newName = ref('')
const newAddr = ref('')
const busy = ref(false)

async function api(path, opts = {}) {
  const sep = path.includes('?') ? '&' : '?'
  const r = await fetch(path + (account.value ? sep + 'account=' + account.value : ''), {
    headers: { 'content-type': 'application/json' },
    ...opts,
  })
  if (!r.ok) throw new Error(await r.text())
  return r.json()
}

async function refresh() {
  if (!account.value) return
  try {
    servers.value = await api('/api/servers')
  } catch {
    account.value = ''
    localStorage.removeItem('eclipse-account')
  }
}

async function startLogin() {
  loginError.value = ''
  login.value = await fetch('/api/login', { method: 'POST' }).then(r => r.json())
  const id = login.value.id
  const poll = setInterval(async () => {
    const s = await fetch('/api/login/' + id).then(r => r.json())
    if (!s.done) return
    clearInterval(poll)
    if (s.error) {
      loginError.value = s.error
      login.value = null
      return
    }
    account.value = s.account
    localStorage.setItem('eclipse-account', s.account)
    login.value = null
    refresh()
  }, 2000)
}

async function addServer() {
  busy.value = true
  try {
    await api('/api/servers', {
      method: 'POST',
      body: JSON.stringify({ account: account.value, name: newName.value, addr: newAddr.value }),
    })
    newName.value = newAddr.value = ''
    adding.value = false
    await refresh()
  } finally {
    busy.value = false
  }
}

async function toggle(s) {
  await api(`/api/servers/${s.id}/toggle?on=${!s.running}`, { method: 'POST' })
  await refresh()
}

async function remove(s) {
  await api(`/api/servers/${s.id}`, { method: 'DELETE' })
  await refresh()
}

function signOut() {
  account.value = ''
  localStorage.removeItem('eclipse-account')
  servers.value = []
}

const joinHost = computed(() => location.hostname)

onMounted(refresh)
</script>

<template>
  <v-app>
    <v-app-bar flat density="comfortable" color="background">
      <v-app-bar-title class="font-weight-medium">
        Eclipse <span class="text-medium-emphasis font-weight-regular">hosted proxy</span>
      </v-app-bar-title>
      <v-btn v-if="account" variant="text" size="small" @click="signOut">sign out</v-btn>
    </v-app-bar>

    <v-main>
      <v-container style="max-width: 640px">
        <!-- signed out -->
        <template v-if="!account">
          <v-card rounded="lg" class="pa-8 text-center">
            <div class="text-h6 mb-2">Play Bedrock with modules, anywhere.</div>
            <div class="text-medium-emphasis mb-6">
              Sign in with your Microsoft account, add a server, and join
              through your own proxy — works on PC, phone and console.
            </div>
            <v-btn v-if="!login" color="primary" size="large" rounded="lg" @click="startLogin">
              Sign in with Microsoft
            </v-btn>
            <template v-else>
              <div class="text-medium-emphasis mb-3">go to</div>
              <a :href="login.uri" target="_blank" class="text-primary text-h6">{{ login.uri }}</a>
              <div class="my-4 text-medium-emphasis">and enter this code</div>
              <div class="text-h4 font-weight-bold" style="letter-spacing: .15em">{{ login.code }}</div>
            </template>
            <div v-if="loginError" class="text-error mt-4">{{ loginError }}</div>
          </v-card>
        </template>

        <!-- signed in -->
        <template v-else>
          <div class="text-overline text-medium-emphasis mb-2">Your servers</div>
          <v-card rounded="lg" class="mb-6">
            <v-list density="comfortable" bg-color="transparent" v-if="servers.length">
              <v-list-item v-for="s in servers" :key="s.id">
                <v-list-item-title>{{ s.name }}</v-list-item-title>
                <v-list-item-subtitle>
                  <code>{{ joinHost }}:{{ s.port }}</code>
                  <span class="text-medium-emphasis"> → {{ s.addr }}</span>
                </v-list-item-subtitle>
                <template #append>
                  <v-switch
                    :model-value="s.running" color="primary" density="compact"
                    hide-details inset class="mr-4"
                    @update:model-value="toggle(s)"
                  />
                  <v-btn icon="$close" variant="text" size="small" @click="remove(s)">✕</v-btn>
                </template>
              </v-list-item>
            </v-list>
            <div v-else class="pa-6 text-medium-emphasis text-center">
              no servers yet — add one and join through {{ joinHost }}:&lt;port&gt;
            </div>
          </v-card>

          <v-card v-if="adding" rounded="lg" class="pa-5 mb-4">
            <v-text-field v-model="newName" label="Name" placeholder="Hypixel" variant="outlined" density="compact" hide-details class="mb-3" />
            <v-text-field v-model="newAddr" label="Server address" placeholder="play.example.com:19132" variant="outlined" density="compact" hide-details class="mb-4" />
            <div class="d-flex ga-2">
              <v-btn color="primary" :loading="busy" :disabled="!newName || !newAddr" @click="addServer">Add server</v-btn>
              <v-btn variant="text" @click="adding = false">Cancel</v-btn>
            </div>
          </v-card>
          <v-btn v-else color="primary" rounded="lg" block @click="adding = true">Add server</v-btn>
        </template>
      </v-container>
    </v-main>
  </v-app>
</template>

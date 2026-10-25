<script setup>
import { computed, onMounted, ref } from 'vue'

const account = ref(localStorage.getItem('eclipse-account') || '')
const gamertag = ref(localStorage.getItem('eclipse-gamertag') || '')
const servers = ref([])
const login = ref(null) // {id, code, uri}
const loginError = ref('')
const adding = ref(false)
const newName = ref('')
const newAddr = ref('')
const busy = ref(false)
const copied = ref('')
const error = ref('')

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
    error.value = ''
  } catch (e) {
    error.value = e.message
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
    gamertag.value = s.gamertag || ''
    localStorage.setItem('eclipse-account', s.account)
    localStorage.setItem('eclipse-gamertag', gamertag.value)
    login.value = null
    refresh()
  }, 2000)
}

async function addServer() {
  busy.value = true
  error.value = ''
  try {
    await api('/api/servers', {
      method: 'POST',
      body: JSON.stringify({ account: account.value, name: newName.value, addr: newAddr.value }),
    })
    newName.value = newAddr.value = ''
    adding.value = false
    await refresh()
  } catch (e) {
    error.value = e.message
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
  gamertag.value = ''
  localStorage.removeItem('eclipse-account')
  localStorage.removeItem('eclipse-gamertag')
  servers.value = []
}

async function copy(text) {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const el = document.createElement('textarea')
    el.value = text
    document.body.appendChild(el)
    el.select()
    document.execCommand('copy')
    el.remove()
  }
  copied.value = text
  setTimeout(() => (copied.value = ''), 1500)
}

const joinHost = computed(() => location.hostname)

onMounted(refresh)
</script>

<template>
  <v-app>
    <v-app-bar flat color="background">
      <v-app-bar-title class="font-weight-bold" style="font-size:22px">
        Eclipse
      </v-app-bar-title>
      <v-btn v-if="account" variant="text" size="large" @click="signOut">sign out</v-btn>
    </v-app-bar>

    <v-main>
      <v-container style="max-width: 560px" class="pa-4">

        <!-- signed out -->
        <template v-if="!account">
          <v-card rounded="xl" class="pa-6 text-center">
            <div class="text-h5 font-weight-bold mb-3">Play Bedrock with modules, anywhere.</div>
            <div class="text-body-1 text-medium-emphasis mb-8" style="line-height:1.6">
              Sign in with your Microsoft account, add a server, and join
              through your own proxy — works on PC, phone and console.
            </div>

            <v-btn v-if="!login" color="primary" size="x-large" rounded="lg" block @click="startLogin">
              Sign in with Microsoft
            </v-btn>

            <template v-else>
              <div class="text-body-1 text-medium-emphasis">1. go to</div>
              <a :href="login.uri" target="_blank" class="text-primary text-h6 d-block my-2">
                {{ login.uri }}
              </a>
              <div class="text-body-1 text-medium-emphasis mt-4">2. enter this code</div>
              <div
                class="text-h3 font-weight-bold my-4"
                style="letter-spacing:.18em; cursor:pointer"
                @click="copy(login.code)"
              >
                {{ login.code }}
              </div>
              <div class="text-body-2 text-medium-emphasis">
                tap to copy — waiting for you to finish…
              </div>
            </template>

            <div v-if="loginError" class="text-error text-body-1 mt-4">{{ loginError }}</div>
          </v-card>
        </template>

        <!-- signed in -->
        <template v-else>
          <div class="text-h6 font-weight-bold mb-1">Your servers</div>
          <div class="text-body-2 text-medium-emphasis mb-4">
            signed in{{ gamertag ? ' as ' + gamertag : '' }} · join an address below in Minecraft Bedrock
          </div>

          <v-alert v-if="error" type="error" variant="tonal" rounded="lg" class="mb-4">{{ error }}</v-alert>

          <div v-if="servers.length" class="d-flex flex-column ga-4 mb-6">
            <v-card v-for="s in servers" :key="s.id" rounded="xl" class="pa-5">
              <div class="d-flex align-center justify-space-between">
                <div class="text-h6 font-weight-bold">{{ s.name }}</div>
                <v-switch
                  :model-value="s.running"
                  color="primary"
                  hide-details
                  inset
                  @update:model-value="toggle(s)"
                />
              </div>

              <div
                class="mt-3 pa-4 rounded-lg d-flex align-center justify-space-between"
                style="background:#141416; cursor:pointer"
                @click="copy(joinHost + ':' + s.port)"
              >
                <code class="text-h6 font-weight-bold" style="color:#e8e8ea">
                  {{ joinHost }}:{{ s.port }}
                </code>
                <span class="text-body-2 text-medium-emphasis">
                  {{ copied === joinHost + ':' + s.port ? 'copied!' : 'tap to copy' }}
                </span>
              </div>

              <div class="d-flex align-center justify-space-between mt-3">
                <div class="text-body-2 text-medium-emphasis">
                  {{ s.running ? 'running' : 'stopped' }} → {{ s.addr }}
                </div>
                <v-btn variant="text" color="error" size="large" @click="remove(s)">Delete</v-btn>
              </div>
            </v-card>
          </div>

          <v-card v-else rounded="xl" class="pa-6 mb-6 text-center text-body-1 text-medium-emphasis">
            No servers yet — add one and join through {{ joinHost }}:&lt;port&gt;
          </v-card>

          <v-card v-if="adding" rounded="xl" class="pa-5">
            <v-text-field
              v-model="newName"
              label="Name"
              placeholder="Hypixel"
              variant="outlined"
              hide-details
              class="mb-4"
            />
            <v-text-field
              v-model="newAddr"
              label="Server address"
              placeholder="play.example.com:19132"
              variant="outlined"
              hide-details
              class="mb-5"
            />
            <v-btn
              color="primary"
              size="x-large"
              rounded="lg"
              block
              :loading="busy"
              :disabled="!newName || !newAddr"
              @click="addServer"
            >
              Add server
            </v-btn>
            <v-btn variant="text" size="large" block class="mt-2" @click="adding = false">Cancel</v-btn>
          </v-card>
          <v-btn
            v-else
            color="primary"
            size="x-large"
            rounded="lg"
            block
            @click="adding = true"
          >
            Add server
          </v-btn>
        </template>
      </v-container>
    </v-main>
  </v-app>
</template>

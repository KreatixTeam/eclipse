<script setup>
import { onMounted, ref } from 'vue'

const account = ref(localStorage.getItem('eclipse-account') || '')
const gamertag = ref('')
const joinHost = ref(location.hostname)
const servers = ref([])
const login = ref(null)
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
  if (!r.ok) throw new Error((await r.text()).trim())
  return r.json()
}

async function loadInfo() {
  const i = await api('/api/info')
  joinHost.value = i.joinHost
  gamertag.value = i.gamertag || ''
}

async function refresh() {
  if (!account.value) return
  try {
    servers.value = await api('/api/servers')
    error.value = ''
  } catch (e) {
    if (e.message === 'unknown account') signOut()
    else error.value = e.message
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
    login.value = null
    if (s.error) {
      loginError.value = s.error
      return
    }
    account.value = s.account
    localStorage.setItem('eclipse-account', s.account)
    await loadInfo()
    await refresh()
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
  s.running = !s.running
  try {
    await api(`/api/servers/${s.id}/toggle?on=${s.running}`, { method: 'POST' })
  } finally {
    await refresh()
  }
}

async function remove(s) {
  if (!confirm(`Remove ${s.name}?`)) return
  await api(`/api/servers/${s.id}`, { method: 'DELETE' })
  await refresh()
}

function signOut() {
  account.value = ''
  gamertag.value = ''
  localStorage.removeItem('eclipse-account')
  servers.value = []
}

async function copy(key, text) {
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
  copied.value = key
  setTimeout(() => copied.value === key && (copied.value = ''), 1400)
}

onMounted(async () => {
  await loadInfo().catch(() => {})
  await refresh()
})
</script>

<template>
  <v-app>
    <div class="page">
      <header class="top">
        <a href="/" class="brand">Eclipse</a>
        <div v-if="account" class="who">
          <span v-if="gamertag" class="gt">{{ gamertag }}</span>
          <button class="link" @click="signOut">Sign out</button>
        </div>
      </header>

      <!-- signed out -->
      <section v-if="!account" class="signin">
        <template v-if="!login">
          <h1>Sign in with your Xbox account</h1>
          <p class="muted">Eclipse joins servers as you. Your password never touches us, Microsoft hands over a sign-in token.</p>
          <v-btn color="primary" size="x-large" rounded="lg" block class="cta" @click="startLogin">
            Sign in with Microsoft
          </v-btn>
        </template>
        <template v-else>
          <div class="step">Step 1</div>
          <h1>Open microsoft.com/link</h1>
          <v-btn :href="login.uri" target="_blank" variant="outlined" size="x-large" rounded="lg" block class="mb-8">
            Open link
          </v-btn>
          <div class="step">Step 2</div>
          <h1>Enter this code</h1>
          <button class="code" @click="copy('code', login.code)">{{ login.code }}</button>
          <p class="muted center">{{ copied === 'code' ? 'Copied' : 'Tap the code to copy' }} · this page updates by itself</p>
        </template>
        <p v-if="loginError" class="err">{{ loginError }}</p>
      </section>

      <!-- signed in -->
      <section v-else>
        <h2>Servers</h2>
        <p v-if="error" class="err">{{ error }}</p>

        <div v-for="s in servers" :key="s.id" class="card">
          <div class="row">
            <div class="min0">
              <div class="name">{{ s.name }}</div>
              <div class="muted small ellipsis">{{ s.addr }}</div>
            </div>
            <v-switch :model-value="s.running" color="primary" hide-details inset density="compact" @update:model-value="toggle(s)" />
          </div>

          <div class="join" :class="{ off: !s.running }">
            <div class="label">Add this in Minecraft → Servers → Add Server</div>
            <button class="field" @click="copy(s.id + 'a', joinHost)">
              <span class="kv"><span class="k">Address</span><span class="v">{{ joinHost }}</span></span>
              <span class="c">{{ copied === s.id + 'a' ? 'Copied' : 'Copy' }}</span>
            </button>
            <button class="field" @click="copy(s.id + 'p', String(s.port))">
              <span class="kv"><span class="k">Port</span><span class="v">{{ s.port }}</span></span>
              <span class="c">{{ copied === s.id + 'p' ? 'Copied' : 'Copy' }}</span>
            </button>
          </div>

          <div class="row foot">
            <span class="muted small">{{ s.running ? 'Online' : 'Off, turn it on to join' }}</span>
            <button class="link danger" @click="remove(s)">Remove</button>
          </div>
        </div>

        <div v-if="!servers.length && !adding" class="empty muted">
          Add the server you want to play on. You get your own address to join it through Eclipse.
        </div>

        <div v-if="adding" class="card">
          <div class="name mb-4">New server</div>
          <v-text-field v-model="newName" label="Name" placeholder="The Hive" variant="outlined" rounded="lg" hide-details class="mb-3" />
          <v-text-field v-model="newAddr" label="Server address" placeholder="geo.hivebedrock.network" variant="outlined" rounded="lg" hide-details class="mb-2" />
          <p class="muted small mb-5">Port defaults to 19132.</p>
          <v-btn color="primary" size="x-large" rounded="lg" block :loading="busy" :disabled="!newName || !newAddr" @click="addServer">
            Add server
          </v-btn>
          <v-btn variant="text" size="large" block class="mt-2" @click="adding = false">Cancel</v-btn>
        </div>
        <v-btn v-else color="primary" size="x-large" rounded="lg" block class="mt-2" @click="adding = true">
          Add server
        </v-btn>
      </section>
    </div>
  </v-app>
</template>

<style>
html, body, .v-application { background: #000 !important; }
.page { width: 100%; box-sizing: border-box; max-width: 520px; margin: 0 auto; padding: 20px 18px 48px; color: #ededef; }
.top { display: flex; align-items: center; justify-content: space-between; height: 48px; margin-bottom: 28px; }
.brand { font-size: 22px; font-weight: 800; color: #fff; text-decoration: none; letter-spacing: -.01em; }
.who { display: flex; align-items: center; gap: 16px; }
.gt { font-size: 16px; font-weight: 600; }
.link { background: none; border: 0; color: #9a9aa0; font-size: 16px; padding: 8px 0; cursor: pointer; }
.link.danger { color: #f87171; }
h1 { font-size: 28px; line-height: 1.2; font-weight: 800; letter-spacing: -.02em; margin-bottom: 12px; }
h2 { font-size: 24px; font-weight: 800; margin-bottom: 16px; }
.muted { color: #8e8e94; font-size: 17px; line-height: 1.5; }
.small { font-size: 15px; }
.center { text-align: center; }
.err { color: #f87171; font-size: 16px; margin: 12px 0; }
.signin { padding-top: 24px; }
.cta { margin-top: 32px; }
.step { color: #3b82f6; font-size: 14px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; margin-bottom: 6px; }
.code { display: block; width: 100%; background: #0e0e10; border: 1px solid #1f1f23; border-radius: 16px; padding: 22px 0; margin: 8px 0 14px; color: #fff; font: 800 38px/1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .14em; cursor: pointer; }
.card { background: #0b0b0d; border: 1px solid #1a1a1e; border-radius: 20px; padding: 20px; margin-bottom: 16px; }
.row { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.min0 { min-width: 0; flex: 1; }
.row .v-input { flex: none !important; }
.card { overflow: hidden; }
.name { font-size: 21px; font-weight: 700; }
.ellipsis { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.join { margin-top: 18px; transition: opacity .25s ease; }
.join.off { opacity: .4; }
.label { color: #8e8e94; font-size: 14px; margin-bottom: 8px; }
.field { display: flex; align-items: center; width: 100%; gap: 12px; background: #141417; border: 0; border-radius: 12px; padding: 14px 16px; margin-bottom: 8px; color: #fff; text-align: left; cursor: pointer; }
.field .kv { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.field .k { color: #8e8e94; font-size: 13px; }
.field .v { font-size: 18px; font-weight: 600; letter-spacing: .01em; overflow-wrap: anywhere; }
.field .c { color: #3b82f6; font-size: 15px; font-weight: 600; flex: none; }
.foot { margin-top: 8px; }
.empty { padding: 8px 2px 24px; }
</style>

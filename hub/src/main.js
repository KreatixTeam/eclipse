import { createApp } from 'vue'
import { createVuetify } from 'vuetify'
import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import App from './App.vue'

const vuetify = createVuetify({
  theme: {
    defaultTheme: 'eclipse',
    themes: {
      eclipse: {
        dark: true,
        colors: {
          background: '#000000',
          surface: '#0a0a0b',
          'surface-variant': '#141416',
          primary: '#2563eb',
          secondary: '#6b6b70',
          success: '#4ade80',
          'on-background': '#e8e8ea',
          'on-surface': '#e8e8ea',
        },
      },
    },
  },
})

createApp(App).use(vuetify).mount('#app')

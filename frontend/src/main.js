import { createApp } from 'vue'
import { createPinia } from 'pinia'
import * as naive from 'naive-ui'
import App from './App.vue'
import router from './router'
import './styles/common.css'
import './styles/autoops.scss'
const app=createApp(App)
Object.entries(naive).filter(([name])=>name.startsWith('N')).forEach(([name,component])=>app.component(name,component))
app.use(createPinia()).use(router).mount('#app')

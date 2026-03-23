import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import { registerElementPlus } from './plugins/element-plus'
import router from './router'
import './assets/main.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)
registerElementPlus(app)
app.mount('#app')


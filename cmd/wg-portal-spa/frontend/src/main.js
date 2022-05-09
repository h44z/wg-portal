import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'


// Bootstrap (and theme)
//import "bootstrap/dist/css/bootstrap.min.css"
import "bootswatch/dist/lux/bootstrap.min.css";
import "bootstrap"
import "./assets/base.css"

// Fontawesome
import '@fortawesome/fontawesome-free/js/all.js';


const app = createApp(App)

app.use(createPinia())
app.use(router)

app.mount('#app')

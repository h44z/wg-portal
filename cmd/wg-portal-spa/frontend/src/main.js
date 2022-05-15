import { createApp } from "vue";
import { createPinia } from "pinia";

import App from "./App.vue";
import router from "./router";
// import Vue from 'vue'
// import VueI18n from 'vue-i18n';
// import VueI18n from "vue-i18n";
import { createI18n } from "vue-i18n/dist/vue-i18n.esm-bundler.js";
import messages from "./lang";

// Bootstrap (and theme)
//import "bootstrap/dist/css/bootstrap.min.css"
import "bootswatch/dist/lux/bootstrap.min.css";
import "bootstrap";
import "./assets/base.css";

// Fontawesome
import "@fortawesome/fontawesome-free/js/all.js";

// 2. Create i18n instance with options
export const i18n = createI18n({
  locale: "de", // set locale
  fallbackLocale: "en", // set fallback locale
  messages, // set locale messages
  // If you need to specify other options, you can set other options
  // ...
});

const app = createApp(App);

app.use(i18n)
// app.use(VueI18n);
// export const i18n = new VueI18n({
//   locale: "de",
//   fallbackLocale: "de",
//   messages,
// });

app.use(createPinia());
app.use(router);
// app.use(i18n);

app.mount("#app");

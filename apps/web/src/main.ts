import { createApp } from 'vue'
import PrimeVue from 'primevue/config';
import "primevue/resources/themes/bootstrap4-light-blue/theme.css";
import "primevue/resources/primevue.min.css";
import "primeicons/primeicons.css"
import ToastService from 'primevue/toastservice';
import ConfirmationService from 'primevue/confirmationservice';
import "/node_modules/primeflex/primeflex.css";
import App from './App.vue';
import "./assets/main.css";

const app = createApp(App);
app.use(PrimeVue);
app.use(ToastService);
app.use(ConfirmationService);
app.mount('#app');

import 'intersection-observer'
import Vue from 'vue'
import App from '@/App.vue'
import router from '@/router'
import store from '@/store'
import i18n from '@/plugins/i18n'
import api from '@/service/api.js'
import openAPI from '@/service/index.js'
import github from '@/service/github.js'
import Buefy from 'buefy'
import VueFullscreen from 'vue-fullscreen'
import Vue2TouchEvents from 'vue2-touch-events'
import VueSocialSharing from 'vue-social-sharing'
import VueSocketIOExt from 'vue-socket.io-extended';
import messageBus from '@/events/index.js'
import VueDOMPurifyHTML from 'vue-dompurify-html'


// Import Styles
import '@/assets/scss/app.scss'
import VAnimateCss from 'v-animate-css';

const io = require("socket.io-client");

const isDev = process.env.NODE_ENV === 'dev';
const protocol = document.location.protocol
const wsProtocol = protocol === 'https:' ? 'wss:' : 'ws:'
const devIp = process.env.VUE_APP_DEV_IP
const devPort = process.env.VUE_APP_DEV_PORT
const localhost = document.location.host
const localhostName = document.location.hostname
const baseIp = isDev ? `${devIp}` : `${localhostName}`
const baseURL = isDev ? `${devIp}:${devPort}` : `${localhost}`
const wsURL = `${wsProtocol}//${baseURL}`

const socket = io( {
	transports: ['websocket', 'polling'],
	path: '/v2/message_bus/socket.io/',
});

Vue.use(Buefy)
Vue.use(VueFullscreen)
Vue.use(VAnimateCss, { animateCSSPath: '/css/animate.min.css' });
Vue.use(Vue2TouchEvents)
Vue.use(VueSocketIOExt, socket);
Vue.use(VueSocialSharing);
Vue.use(VueDOMPurifyHTML, {
	default: {
		ALLOWED_ATTR: ['target', 'href']
	}
});

Vue.config.productionTip = false
// Vue normally just console.errors a component error and carries on with a
// blank/partial render - route it through the same on-screen overlay index.html
// installs for boot-time errors, so a failure is visible/reportable on-device
// instead of silently leaving a blank screen with nothing to go on.
Vue.config.errorHandler = function (err, vm, info) {
	console.error(err, info)
	if (window.__casaosShowBootError) {
		window.__casaosShowBootError('CasaOS hit an error', (err && err.stack) || String(err))
	}
}
Vue.prototype.$api = api;
Vue.prototype.$openAPI = openAPI;
// Separate from $api on purpose - this talks to api.github.com directly
// with a user-supplied token, not this CasaOS server's own API.
Vue.prototype.$github = github;
Vue.prototype.$baseIp = baseIp;
Vue.prototype.$baseURL = baseURL;
Vue.prototype.$protocol = protocol;
Vue.prototype.$wsProtocol = wsProtocol;


// Create an EventBus
Vue.prototype.$EventBus = new Vue();
Vue.prototype.$messageBus = messageBus;

new Vue({
	router,
	i18n,
	store,
	render: h => h(App)
}).$mount('#app')

// From here on, the app is otherwise working - a single failed background
// request (a widget refresh, a stray network blip) shouldn't blank a page
// that's actually fine, so index.html's overlay stops full-screening errors
// once this is set and just logs them instead, like any normal app would.
window.__casaosMounted = true

// A successful mount means index.html's own required-chunk cache-busting
// reload (see public/index.html) actually worked, or was never needed -
// clear its one-shot guard so a genuine future failure in this tab isn't
// silently left unrecovered because of a reload attempted much earlier.
try {
	sessionStorage.removeItem('casaos_reload_attempted')
} catch (e) {
	// storage unavailable (e.g. private mode) - nothing to clear
}






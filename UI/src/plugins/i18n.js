
import Vue      from 'vue'
import VueI18n  from 'vue-i18n'
import messages from '@/assets/lang'

Vue.use(VueI18n)

// localStorage.getItem can throw (not just return null) in some mobile
// browser privacy modes/embedded webviews, and this runs at module import
// time, before the app has even started - an uncaught throw here takes down
// the whole page before anything is on screen.
let savedLang
try {
	savedLang = localStorage.getItem('lang')
}
catch (error) {
	savedLang = null
}

const i18n = new VueI18n({
	// Define defalut language
	locale: savedLang || 'en_us',
	fallbackLocale: 'en_us',
	silentTranslationWarn: true,
	messages
})
export default i18n

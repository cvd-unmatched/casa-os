import { api } from './service.js'

const PREFIX = '/autoupdate'

const autoupdate = {
	// list every CasaOS-managed app's auto-update status and settings
	listApps() {
		return api.get(`${PREFIX}/apps`)
	},

	// set an app's settings: { autoUpdate: bool, notify: bool }
	setSettings(name, settings) {
		return api.put(`${PREFIX}/apps/${name}/settings`, settings)
	},

	// force a synchronous, read-only registry check for one app - never
	// applies an update itself, even if the app's autoUpdate is on
	recheck(name) {
		return api.post(`${PREFIX}/apps/${name}/recheck`)
	},
}

export default autoupdate

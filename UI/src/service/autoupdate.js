import { api } from './service.js'

const PREFIX = '/autoupdate'

const autoupdate = {
	// list every CasaOS-managed app's auto-update status and policy
	listApps() {
		return api.get(`${PREFIX}/apps`)
	},

	// set an app's policy: 'auto' | 'notify' | 'off'
	setPolicy(name, policy) {
		return api.put(`${PREFIX}/apps/${name}/policy`, { policy })
	},

	// force a synchronous, read-only registry check for one app - never
	// applies an update itself, even if the app's policy is 'auto'
	recheck(name) {
		return api.post(`${PREFIX}/apps/${name}/recheck`)
	},
}

export default autoupdate

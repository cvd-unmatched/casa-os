import { api } from './service.js'

// api.get/post/etc auto-prepend /v1 to whatever URL is passed (service.js's
// testVisionNum - skipped only for an absolute http(s) URL or one already
// starting /v2-/v9, deliberately NOT /v1 itself), so PREFIX here must stay
// bare to match autoupdate.js's own convention - adding /v1 ourselves would
// double up to /v1/v1/backup/... on every axios call.
const PREFIX = '/backup'

const backup = {
	// GET is a plain browser navigation (window.location.href), not an axios
	// call - buffering a multi-GB archive into a Blob first (the pattern
	// container.js's exportAsCompose uses for a few KB of YAML) risks a tab
	// crash. Unlike the axios calls below, a raw navigation never goes
	// through testVisionNum's auto /v1 prefix, so this builds the full path
	// by hand - confirmed via the real backend mount in route/v1.go. The
	// token has to travel as a query param since a navigation can't set an
	// Authorization header.
	getExportUrl() {
		const token = localStorage.getItem('access_token')
		return `/v1${PREFIX}/export?token=${encodeURIComponent(token)}`
	},

	// unlike export, a File posted as the raw body streams from disk via
	// the browser's XHR layer without JS-side buffering, so onUploadProgress
	// gives a real progress bar. timeout: 0 overrides service.js's shared
	// 60s default, which is far too short for a large import.
	importBackup(file, onUploadProgress) {
		return api.post(`${PREFIX}/import`, file, {
			headers: { 'Content-Type': 'application/octet-stream' },
			timeout: 0,
			onUploadProgress,
		})
	},
}

export default backup

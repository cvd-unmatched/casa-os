import { api } from './service.js'

// api.get/post/etc auto-prepend /v1 to whatever URL is passed (service.js's
// testVisionNum - skipped only for an absolute http(s) URL or one already
// starting /v2-/v9, deliberately NOT /v1 itself), so PREFIX here must stay
// bare to match autoupdate.js's own convention - adding /v1 ourselves would
// double up to /v1/v1/backup/... on every axios call.
const PREFIX = '/backup'

const backup = {
	// lists every exportable app (name, data paths, data size) without
	// archiving anything - populates the "include data" checklist shown
	// before an export actually starts.
	listApps() {
		return api.get(`${PREFIX}/apps`)
	},

	// GET is a plain browser navigation (window.location.href), not an axios
	// call - buffering a multi-GB archive into a Blob first (the pattern
	// container.js's exportAsCompose uses for a few KB of YAML) risks a tab
	// crash. Unlike the axios calls here, a raw navigation never goes
	// through testVisionNum's auto /v1 prefix, so this builds the full path
	// by hand - confirmed via the real backend mount in route/v1.go. The
	// token has to travel as a query param since a navigation can't set an
	// Authorization header. excludeDataNames lists apps whose data should be
	// skipped (their compose config is still included either way).
	getExportUrl(excludeDataNames = []) {
		const token = localStorage.getItem('access_token')
		const params = new URLSearchParams({ token })
		if (excludeDataNames.length)
			params.set('exclude_data', excludeDataNames.join(','))
		return `/v1${PREFIX}/export?${params.toString()}`
	},

	// stages an uploaded archive and returns a preview (per-app ports,
	// volumes, and conflicts) without installing anything. A File posted as
	// the raw body streams from disk via the browser's XHR layer without
	// JS-side buffering, so onUploadProgress gives a real progress bar.
	// timeout: 0 overrides service.js's shared 60s default, far too short
	// for a large upload.
	importPreview(file, onUploadProgress) {
		return api.post(`${PREFIX}/import/preview`, file, {
			headers: { 'Content-Type': 'application/octet-stream' },
			timeout: 0,
			onUploadProgress,
		})
	},

	// applies a previewed import with the review screen's edits.
	// apps: [{ name, skip, ports: [{service_name, target, protocol, published}],
	//          volumes: [{service_name, target, source}] }]
	importConfirm(previewId, apps) {
		return api.post(`${PREFIX}/import/confirm`, { preview_id: previewId, apps }, { timeout: 0 })
	},
}

export default backup

import { api } from './service.js'

const PREFIX = '/backup'

const backup = {
	// GET is a plain browser navigation (window.location.href), not an axios
	// call - buffering a multi-GB archive into a Blob first (the pattern
	// container.js's exportAsCompose uses for a few KB of YAML) risks a
	// tab crash. This just builds the URL; the token has to travel as a
	// query param since a navigation can't set an Authorization header.
	getExportUrl() {
		const token = localStorage.getItem('access_token')
		return `${PREFIX}/export?token=${encodeURIComponent(token)}`
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

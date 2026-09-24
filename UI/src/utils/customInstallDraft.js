// The compose YAML of the last customized-app install that failed, so the
// "install a customized app" form can pick up where it left off instead of
// starting blank once the panel it was typed into is gone. Expires so an
// abandoned attempt doesn't keep coming back forever.
const KEY = 'casaos_custom_install_draft'
const MAX_AGE_MS = 24 * 60 * 60 * 1000

export function saveCustomInstallDraft(yaml) {
	if (!yaml)
		return
	try {
		localStorage.setItem(KEY, JSON.stringify({ yaml, savedAt: Date.now() }))
	}
	catch {}
}

export function loadCustomInstallDraft() {
	try {
		const draft = JSON.parse(localStorage.getItem(KEY))
		if (draft && draft.yaml && Date.now() - draft.savedAt < MAX_AGE_MS)
			return draft.yaml
	}
	catch {}
	return ''
}

export function clearCustomInstallDraft() {
	try {
		localStorage.removeItem(KEY)
	}
	catch {}
}

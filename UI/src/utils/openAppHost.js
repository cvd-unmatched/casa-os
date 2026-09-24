// Resolves which host apps should open at. By default that's just the
// browser's current hostname, but that breaks when the dashboard itself is
// reached through something that doesn't proxy arbitrary ports - a
// Cloudflare Tunnel domain being the common case, since it only forwards
// the hostnames/ports it was explicitly configured with, not every app's
// own port on that same domain. This lets a user instead pick the box's
// LAN IP or its Tailscale IP.
import sys from '../service/sys.js'
import users from '../service/users.js'

export const OPEN_APP_HOST_PREFERENCE_KEY = 'open_app_host_preference'

// Both requests are tiny and local, but there's no reason to repeat them on
// every single app-grid refresh or click - so cache them briefly. Not for the
// whole page session: a lookup that failed once (an expired token mid-refresh,
// the request racing a just-saved preference) would otherwise silently keep
// every app link on the page's own address until a reload.
const CACHE_TTL_MS = 30 * 1000
let cachedResolution = null
let cachedAt = 0

function fetchPreferenceAndAccessIps() {
	return Promise.all([
		users.getCustomStorage(OPEN_APP_HOST_PREFERENCE_KEY).then(res => res.data.data.data).catch(() => null),
		sys.getAccessIPs().then(res => res.data.data).catch(() => null),
	])
}

/**
 * @description Resolves the host app links should use per the user's App
 * Links preference. Returns null when no preference is set, the fetch
 * failed, or the box doesn't actually have the requested kind of address -
 * callers should fall back to whatever they'd otherwise use (an app's own
 * configured hostname, then the current browser hostname). A preference the
 * user actively chose is meant to apply across every app, so it must be
 * checked before any per-app hostname, not after.
 * @return {Promise<string|null>}
 */
export function resolveOpenAppHost() {
	if (!cachedResolution || Date.now() - cachedAt > CACHE_TTL_MS) {
		cachedResolution = fetchPreferenceAndAccessIps()
		cachedAt = Date.now()
	}

	return cachedResolution.then(([preference, accessIps]) => {
		if (!preference || !accessIps)
			return null
		if (preference.mode === 'lan' && accessIps.lan_ips && accessIps.lan_ips.length)
			return accessIps.lan_ips[0]
		if (preference.mode === 'tailscale' && accessIps.tailscale_ip)
			return accessIps.tailscale_ip
		return null
	})
}

// Settings panels that change the preference call this so the next
// resolution picks it up immediately instead of the page needing a reload.
export function invalidateOpenAppHostCache() {
	cachedResolution = null
}

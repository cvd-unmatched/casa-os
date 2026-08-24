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

// Resolved once per page load and cached - both requests are tiny and
// local, but there's no reason to repeat them on every single app-grid
// refresh or click.
let cachedResolution = null

function fetchPreferenceAndAccessIps() {
	return Promise.all([
		users.getCustomStorage(OPEN_APP_HOST_PREFERENCE_KEY).then(res => res.data.data.data).catch(() => null),
		sys.getAccessIPs().then(res => res.data.data).catch(() => null),
	])
}

/**
 * @description Resolves the host app links should use. Falls back to
 * fallbackHost (the current browser hostname) whenever no preference is
 * set, the fetch failed, or the box doesn't actually have the requested
 * kind of address.
 * @param {string} fallbackHost
 * @return {Promise<string>}
 */
export function resolveOpenAppHost(fallbackHost) {
	if (!cachedResolution)
		cachedResolution = fetchPreferenceAndAccessIps()

	return cachedResolution.then(([preference, accessIps]) => {
		if (!preference || !accessIps)
			return fallbackHost
		if (preference.mode === 'lan' && accessIps.lan_ips && accessIps.lan_ips.length)
			return accessIps.lan_ips[0]
		if (preference.mode === 'tailscale' && accessIps.tailscale_ip)
			return accessIps.tailscale_ip
		return fallbackHost
	})
}

// Settings panels that change the preference call this so the next
// resolution picks it up immediately instead of the page needing a reload.
export function invalidateOpenAppHostCache() {
	cachedResolution = null
}

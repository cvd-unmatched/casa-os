// Talks directly to DHL's free, self-service Shipment Tracking - Unified API
// (api-eu.dhl.com/track/shipments) using a user-supplied API key from
// developer.dhl.com. Deliberately a separate axios instance, same reasoning
// as service/github.js and service/weather.js.
import axios from 'axios'

const dhl = axios.create({ baseURL: 'https://api-eu.dhl.com/track' })

export default {
	/**
	 * @description: Looks up the latest status for a DHL tracking number.
	 * Returns null on most failures (unknown number, network error, etc.) -
	 * callers should fall back to a manual tracking link rather than
	 * surfacing an error for a single package. A 401 is rethrown instead:
	 * it means the key itself is rejected (not registered, not yet approved
	 * by DHL, or wrong key copied) and will fail identically for every
	 * package, so it's worth telling the user rather than hiding forever
	 * behind "Could not check status".
	 * @param {string} apiKey
	 * @param {string} trackingNumber
	 * @return {Promise<Object|null>} { description, timestamp } or null
	 */
	async trackShipment(apiKey, trackingNumber) {
		try {
			const res = await dhl.get('/shipments', {
				headers: { 'DHL-API-Key': apiKey },
				params: { trackingNumber },
			})
			const shipment = res.data?.shipments?.[0]
			const status = shipment?.status
			if (!status) return null
			return {
				description: status.description || status.statusCode || '',
				timestamp: status.timestamp || null,
			}
		}
		catch (error) {
			if (error.response && error.response.status === 401) throw error
			return null
		}
	},
}

// Talks directly to DHL's free, self-service Shipment Tracking - Unified API
// (api-eu.dhl.com/track/shipments) using a user-supplied API key from
// developer.dhl.com. Deliberately a separate axios instance, same reasoning
// as service/github.js and service/weather.js.
import axios from 'axios'

const dhl = axios.create({ baseURL: 'https://api-eu.dhl.com/track' })

export default {
	/**
	 * @description: Looks up the latest status for a DHL tracking number.
	 * Returns null on any failure (invalid key, unknown number, network
	 * error, etc.) - callers should fall back to a manual tracking link
	 * rather than surfacing an error for a single package.
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
			return null
		}
	},
}

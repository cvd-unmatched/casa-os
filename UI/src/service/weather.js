// Talks directly to open-meteo.com - free, no API key required. Deliberately
// a separate axios instance from service.js's `instance`, same reasoning as
// service/github.js: this must not pick up the CasaOS-facing base URL/auth.
import axios from 'axios'

const geocoding = axios.create({ baseURL: 'https://geocoding-api.open-meteo.com/v1' })
const forecast = axios.create({ baseURL: 'https://api.open-meteo.com/v1' })

export default {
	/**
	 * @description: Looks up candidate locations for a place name.
	 * @param {string} query
	 * @return {Promise<Array>} up to 5 matches with name, country, latitude, longitude
	 */
	async searchLocation(query) {
		const res = await geocoding.get('/search', {
			params: { name: query, count: 5, language: 'en', format: 'json' },
		})
		return res.data.results || []
	},

	/**
	 * @description: Fetches current conditions plus today's hourly forecast
	 * for a location.
	 * @param {number} latitude
	 * @param {number} longitude
	 * @return {Promise<Object>} { current, hourly }
	 */
	async getForecast(latitude, longitude) {
		const res = await forecast.get('/forecast', {
			params: {
				latitude,
				longitude,
				current: 'temperature_2m,weathercode,windspeed_10m',
				hourly: 'temperature_2m,precipitation_probability,weathercode,windspeed_10m',
				timezone: 'auto',
				forecast_days: 1,
			},
		})
		return res.data
	},
}

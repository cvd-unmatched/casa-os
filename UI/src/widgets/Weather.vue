<template>
	<div class="widget has-text-white weather is-relative">
		<div class="blur-background"></div>
		<div class="widget-content">
			<!-- Header Start -->
			<div class="widget-header is-flex">
				<div class="widget-title is-flex-grow-1">
					{{ $t('Weather') }}
				</div>
				<b-icon
					class="is-clickable" icon="edit-outline" pack="casa"
					size="is-small" @click.native="promptLocation"
				/>
			</div>
			<!-- Header End -->

			<div v-if="!location" class="has-text-grey-100 is-size-7 py-2">
				{{ $t('Set a location to see the forecast.') }}
			</div>
			<div v-else-if="isLoading" class="has-text-grey-100 is-size-7 py-2">
				{{ $t('Loading forecast…') }}
			</div>
			<div v-else-if="loadError" class="has-text-grey-100 is-size-7 py-2">
				{{ $t('Could not load the forecast.') }}
			</div>
			<template v-else-if="current">
				<!-- Current Conditions Start -->
				<div class="current-row is-flex is-align-items-center mb-3">
					<span class="condition-icon mr-2">{{ conditionIcon(current.weathercode) }}</span>
					<span class="current-temp">{{ Math.round(current.temperature_2m) }}°</span>
					<span class="location-name one-line ml-2 is-flex-grow-1 has-text-grey-100 is-size-7" :title="location.name">
						{{ location.name }}
					</span>
				</div>
				<!-- Current Conditions End -->

				<!-- Time-of-day blocks Start -->
				<div v-for="block in dayBlocks" :key="block.label" class="day-block mb-2">
					<div class="is-flex is-align-items-center mb-1">
						<span class="block-label is-flex-grow-1 has-text-weight-semibold is-size-7">{{ $t(block.label) }}</span>
						<span class="condition-icon mr-1">{{ conditionIcon(block.weathercode) }}</span>
						<span class="is-size-7">{{ Math.round(block.temperature) }}°</span>
					</div>
					<div class="outfit-row is-flex is-align-items-flex-start is-size-7 has-text-grey-100">
						<span class="mr-1">🧑</span>
						<span class="is-flex-grow-1">{{ block.himItems.join(', ') }}</span>
					</div>
					<div class="outfit-row is-flex is-align-items-flex-start is-size-7 has-text-grey-100">
						<span class="mr-1">👩</span>
						<span class="is-flex-grow-1">{{ block.herItems.join(', ') }}</span>
					</div>
				</div>
				<!-- Time-of-day blocks End -->
			</template>
		</div>
	</div>
</template>

<script>
const locationConfig = 'weather_location'

// Representative hours used to summarize the day - matches how most people
// think about "what should I wear this morning/afternoon/evening".
const DAY_BLOCKS = [
	{ label: 'Morning', hour: 8 },
	{ label: 'Afternoon', hour: 14 },
	{ label: 'Evening', hour: 20 },
]

// WMO weather codes (used by Open-Meteo) grouped into the categories the
// outfit advice and icon picking need.
const RAIN_CODES = [51, 53, 55, 56, 57, 61, 63, 65, 66, 67, 80, 81, 82]
const SNOW_CODES = [71, 73, 75, 77, 85, 86]
const STORM_CODES = [95, 96, 99]
const FOG_CODES = [45, 48]
const CLOUDY_CODES = [2, 3]

export default {
	// eslint-disable-next-line vue/multi-word-component-names
	name: 'weather',
	icon: 'wallpaper-outline',
	title: 'Weather',
	initShow: false,
	data() {
		return {
			location: null,
			isLoading: false,
			loadError: false,
			current: null,
			dayBlocks: [],
		}
	},
	mounted() {
		this.$api.users.getCustomStorage(locationConfig).then((res) => {
			const saved = res.data.data
			if (saved && saved.latitude !== undefined) {
				this.location = saved
				this.load()
			}
		})
	},
	methods: {
		promptLocation() {
			this.$buefy.dialog.prompt({
				message: this.$t('City or town'),
				inputAttrs: {
					placeholder: this.$t('e.g. Brussels'),
					value: this.location ? this.location.name : '',
				},
				trapFocus: true,
				confirmText: this.$t('Search'),
				onConfirm: value => this.searchAndSave(value),
			})
		},

		async searchAndSave(query) {
			if (!query || !query.trim())
				return
			try {
				const results = await this.$weather.searchLocation(query.trim())
				if (results.length === 0) {
					this.$buefy.toast.open({ message: this.$t('No matching location found.'), type: 'is-danger' })
					return
				}
				const match = results[0]
				const location = {
					name: [match.name, match.admin1, match.country].filter(Boolean).join(', '),
					latitude: match.latitude,
					longitude: match.longitude,
				}
				this.location = location
				this.$api.users.setCustomStorage(locationConfig, location)
				this.load()
			}
			catch (error) {
				this.$buefy.toast.open({ message: this.$t('Could not search for that location.'), type: 'is-danger' })
			}
		},

		async load() {
			this.isLoading = true
			this.loadError = false
			try {
				const data = await this.$weather.getForecast(this.location.latitude, this.location.longitude)
				this.current = data.current
				this.dayBlocks = this.buildDayBlocks(data.hourly)
			}
			catch (error) {
				this.loadError = true
			}
			finally {
				this.isLoading = false
			}
		},

		// Picks the hourly entry closest to each representative hour and
		// attaches outfit advice to it.
		buildDayBlocks(hourly) {
			if (!hourly || !hourly.time)
				return []
			return DAY_BLOCKS.map((block) => {
				const index = hourly.time.findIndex(t => Number(t.slice(11, 13)) === block.hour)
				const i = index === -1 ? 0 : index
				const temperature = hourly.temperature_2m[i]
				const weathercode = hourly.weathercode[i]
				const windspeed = hourly.windspeed_10m[i]
				const { himItems, herItems } = this.adviseOutfit(temperature, weathercode, windspeed)
				return { label: block.label, temperature, weathercode, himItems, herItems }
			})
		},

		conditionIcon(code) {
			if (STORM_CODES.includes(code)) return '⛈️'
			if (SNOW_CODES.includes(code)) return '❄️'
			if (RAIN_CODES.includes(code)) return '🌧️'
			if (FOG_CODES.includes(code)) return '🌫️'
			if (CLOUDY_CODES.includes(code)) return '☁️'
			if (code === 1) return '🌤️'
			return '☀️'
		},

		// Rule-based outfit suggestions from temperature/condition/wind - not
		// a weather service, just practical layering advice per band.
		adviseOutfit(tempC, code, windKmh) {
			let himItems
			let herItems
			if (tempC <= 0) {
				himItems = ['heavy coat', 'thermal layers', 'beanie', 'gloves']
				herItems = ['heavy coat', 'thermal layers', 'beanie', 'gloves', 'scarf']
			}
			else if (tempC <= 9) {
				himItems = ['warm jacket', 'sweater', 'long pants']
				herItems = ['warm coat', 'sweater', 'jeans or leggings']
			}
			else if (tempC <= 17) {
				himItems = ['light jacket or hoodie', 'jeans']
				herItems = ['light jacket', 'jeans or skirt with tights']
			}
			else if (tempC <= 23) {
				himItems = ['t-shirt', 'light pants']
				herItems = ['blouse or t-shirt', 'skirt or light pants']
			}
			else if (tempC <= 29) {
				himItems = ['t-shirt', 'shorts']
				herItems = ['sundress or t-shirt', 'shorts']
			}
			else {
				himItems = ['tank top', 'shorts', 'sandals']
				herItems = ['sundress', 'sandals']
			}

			if (RAIN_CODES.includes(code) || STORM_CODES.includes(code)) {
				himItems.push('umbrella')
				herItems.push('umbrella')
			}
			if (SNOW_CODES.includes(code)) {
				himItems.push('snow boots')
				herItems.push('snow boots')
			}
			if (windKmh >= 30) {
				himItems.push('windbreaker')
				herItems.push('windbreaker')
			}

			return { himItems, herItems }
		},
	},
}
</script>

<style lang="scss" scoped>
.weather {
	.current-temp {
		font-size: 1.75rem;
		font-weight: 600;
	}

	.condition-icon {
		font-size: 1.25rem;
		line-height: 1;
	}

	.day-block {
		border-top: 1px solid $border;
		padding-top: 0.5rem;

		&:first-of-type {
			border-top: 1px solid $border;
		}
	}

	.outfit-row {
		line-height: 1.3;
	}
}
</style>

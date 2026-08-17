<template>
	<div class="widget has-text-white packages is-relative">
		<div class="blur-background"></div>
		<div class="widget-content">
			<!-- Header Start -->
			<div class="widget-header is-flex">
				<div class="widget-title is-flex-grow-1">
					{{ $t('Packages') }}
				</div>
				<b-icon
					class="is-clickable mr-2" icon="protection-outline" pack="casa"
					size="is-small" :title="$t('Connect DHL for live status')" @click.native="promptDhlKey"
				/>
				<b-icon class="is-clickable" icon="plus-outline" pack="casa" size="is-small" @click.native="openAddModal" />
			</div>
			<!-- Header End -->

			<div v-if="items.length === 0" class="has-text-grey-100 is-size-7 py-2">
				{{ $t('No packages tracked yet.') }}
			</div>

			<div v-for="item in items" :key="item.id" class="package-row mb-2 is-flex is-align-items-flex-start">
				<div class="is-flex-grow-1 is-size-7">
					<div class="one-line has-text-weight-semibold">
						{{ item.nickname || item.trackingNumber }}
					</div>
					<div class="has-text-grey-100">
						{{ carrierLabel(item.carrier) }} · {{ item.trackingNumber }}
					</div>
					<div v-if="item.carrier === 'dhl' && dhlApiKey" class="mt-1">
						<span v-if="statuses[item.id] === undefined" class="has-text-grey-100">{{ $t('Checking status…') }}</span>
						<span v-else-if="statuses[item.id]">{{ statuses[item.id].description }}</span>
						<span v-else class="has-text-grey-100">{{ $t('Could not check status - use the track link instead.') }}</span>
					</div>
					<label v-else-if="item.carrier !== 'dhl' || !dhlApiKey" class="mt-1 is-flex is-align-items-center is-clickable">
						<b-checkbox :value="item.delivered" size="is-small" @input="toggleDelivered(item)" />
						<span class="ml-1">{{ item.delivered ? $t('Delivered') : $t('Mark delivered') }}</span>
					</label>
				</div>
				<div class="is-flex-shrink-0 is-flex is-align-items-center">
					<b-icon
						class="is-clickable mr-1" icon="right-outline" pack="casa"
						size="is-small" :title="$t('Track')" @click.native="openTracking(item)"
					/>
					<b-icon
						class="is-clickable" icon="trash-outline" pack="casa"
						size="is-small" @click.native="removePackage(item)"
					/>
				</div>
			</div>
		</div>
	</div>
</template>

<script>
import PackageFormModal from '@/components/widgets/PackageFormModal.vue'

const packagesConfig = 'packages_config'

// DHL's free-tier Unified Tracking API caps at 250 requests/day. Leaving
// headroom below that (for manual refreshes, multiple open tabs/devices, and
// the initial "add package" check) rather than targeting the cap exactly.
const DHL_DAILY_REQUEST_BUDGET = 200
// Never check more often than this regardless of how few packages are
// tracked - there's no reason to hammer the API for a single package just
// because the daily-budget math would technically allow it.
const MIN_DHL_CHECK_INTERVAL_MS = 30 * 60 * 1000

// Only DHL has a free, self-service tracking API. Everything else falls
// back to a direct link to the carrier's own tracking page plus a manual
// "mark delivered" toggle - no live status, but zero cost and nothing to
// break.
const CARRIERS = [
	{ id: 'dhl', label: 'DHL Express', urlTemplate: 'https://www.dhl.com/global-en/home/tracking.html?tracking-id={num}', liveApi: true },
	{ id: 'bpost', label: 'bpost', urlTemplate: 'https://track.bpost.cloud/btr/web/#/search?itemCode={num}&lang=en' },
	{ id: 'ups', label: 'UPS', urlTemplate: 'https://www.ups.com/track?tracknum={num}' },
	{ id: 'fedex', label: 'FedEx', urlTemplate: 'https://www.fedex.com/fedextrack/?trknbr={num}' },
	{ id: 'postnl', label: 'PostNL', urlTemplate: 'https://jouw.postnl.nl/track-and-trace/{num}' },
	{ id: 'gls', label: 'GLS', urlTemplate: 'https://gls-group.eu/EU/en/parcel-tracking?match={num}' },
	{ id: 'other', label: 'Other', urlTemplate: 'https://www.google.com/search?q={num}+tracking' },
]

export default {
	// eslint-disable-next-line vue/multi-word-component-names
	name: 'packages',
	icon: 'downloads-outline',
	title: 'Packages',
	initShow: false,
	data() {
		return {
			dhlApiKey: '',
			items: [],
			// When the last DHL check actually ran (ms epoch) - persisted
			// (not just in-memory) since the request budget below has to hold
			// across page reloads and multiple open tabs/devices, not just
			// within one browser session.
			lastDhlCheckAt: 0,
			// Seeded from each item's persisted lastStatusDescription on
			// mount, then kept live from actual API responses - this way a
			// rate-limited skip still shows the last known status instead of
			// a permanent "Checking status…".
			statuses: {},
		}
	},
	mounted() {
		this.$api.users.getCustomStorage(packagesConfig).then((res) => {
			const saved = res.data.data
			if (saved) {
				this.dhlApiKey = saved.dhlApiKey || ''
				this.items = saved.items || []
				this.lastDhlCheckAt = saved.lastDhlCheckAt || 0
				this.items.forEach((item) => {
					if (item.carrier === 'dhl' && item.lastStatusDescription)
						this.$set(this.statuses, item.id, { description: item.lastStatusDescription })
				})
				this.refreshDhlStatuses()
			}
		})
		// A tick to re-check periodically while the dashboard stays open - the
		// actual request budget is enforced inside refreshDhlStatuses, not
		// here, since DHL's free tier caps at 250 requests/day and that has to
		// hold regardless of how many times this widget mounts. See the
		// webhook notifications plan for why this whole thing is client-side
		// in the first place: the DHL key/tracked-package list only exist in
		// browser-stored custom storage, which the backend has no session
		// context to read.
		this.pollTimer = setInterval(() => this.refreshDhlStatuses(), MIN_DHL_CHECK_INTERVAL_MS)
	},
	beforeDestroy() {
		clearInterval(this.pollTimer)
	},
	methods: {
		carrierLabel(id) {
			const carrier = CARRIERS.find(c => c.id === id)
			return carrier ? carrier.label : id
		},

		trackUrl(item) {
			const carrier = CARRIERS.find(c => c.id === item.carrier) || CARRIERS[CARRIERS.length - 1]
			return carrier.urlTemplate.replace('{num}', encodeURIComponent(item.trackingNumber))
		},

		openTracking(item) {
			window.open(this.trackUrl(item), '_blank', 'noopener')
		},

		promptDhlKey() {
			this.$buefy.dialog.prompt({
				message: this.$t('DHL API key'),
				inputAttrs: {
					placeholder: this.$t('Paste a free API key from {url} for live DHL status.', { url: 'developer.dhl.com' }),
					value: this.dhlApiKey,
				},
				trapFocus: true,
				confirmText: this.$t('Save'),
				onConfirm: (value) => {
					this.dhlApiKey = value.trim()
					this.save()
					this.refreshDhlStatuses()
				},
			})
		},

		openAddModal() {
			this.$buefy.modal.open({
				parent: this,
				component: PackageFormModal,
				hasModalCard: true,
				trapFocus: true,
				canCancel: ['escape', 'outside'],
				props: { carriers: CARRIERS },
				events: { save: pkg => this.addPackage(pkg) },
			})
		},

		addPackage(pkg) {
			const item = { id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`, delivered: false, ...pkg }
			this.items.push(item)
			this.save()
			if (item.carrier === 'dhl' && this.dhlApiKey)
				this.refreshOne(item)
		},

		removePackage(item) {
			this.$buefy.dialog.confirm({
				title: this.$t('Stop tracking?'),
				message: this.$t('This removes {nickname} from your tracked packages.', { nickname: item.nickname || item.trackingNumber }),
				type: 'is-danger',
				confirmText: this.$t('Remove'),
				cancelText: this.$t('Cancel'),
				onConfirm: () => {
					this.items = this.items.filter(i => i.id !== item.id)
					this.$delete(this.statuses, item.id)
					this.save()
				},
			})
		},

		toggleDelivered(item) {
			item.delivered = !item.delivered
			this.save()
		},

		async refreshOne(item) {
			const previousDescription = item.lastStatusDescription
			this.$set(this.statuses, item.id, undefined)
			const status = await this.$dhl.trackShipment(this.dhlApiKey, item.trackingNumber)
			this.$set(this.statuses, item.id, status)

			if (status && status.description !== previousDescription) {
				item.lastStatusDescription = status.description
				this.save()
				// previousDescription undefined means this is the first check
				// this item has ever had (nothing to compare against, so it's
				// not a "change" worth a webhook).
				if (previousDescription !== undefined)
					this.notifyDeliveryChange(item, status)
			}
		},

		async notifyDeliveryChange(item, status) {
			try {
				const res = await this.$api.sys.getWebhooks()
				const destinations = (res.data.data && res.data.data.destinations) || []
				const title = this.$t('Package update')
				const message = `${item.nickname || item.trackingNumber}: ${status.description}`
				const fields = { carrier: this.carrierLabel(item.carrier), tracking_number: item.trackingNumber }
				destinations
					.filter(d => d.events.includes('package_update'))
					.forEach(d => this.postWebhook(d, title, message, fields))
			}
			catch (error) {
				// best-effort - a failed webhook lookup shouldn't disrupt the widget
			}
		},

		async postWebhook(destination, title, message, fields) {
			let body
			if (destination.type === 'discord') {
				body = {
					embeds: [{
						title,
						description: message,
						color: 0x2ECC71,
						fields: Object.entries(fields).map(([name, value]) => ({ name, value, inline: true })),
					}],
				}
			}
			else if (destination.type === 'slack') {
				body = { text: `*${title}*\n${message}` }
			}
			else {
				body = { event: 'package_update', title, message, timestamp: new Date().toISOString(), data: fields }
			}

			try {
				await fetch(destination.url, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify(body),
				})
			}
			catch (error) {
				// best-effort - a broken webhook URL shouldn't surface as an
				// error for what's otherwise a normal status refresh
			}
		},

		refreshDhlStatuses() {
			if (!this.dhlApiKey) return
			const dhlItems = this.items.filter(item => item.carrier === 'dhl')
			if (dhlItems.length === 0) return

			// Spread checks out enough that (checks per day) * (tracked DHL
			// packages) stays under DHL_DAILY_REQUEST_BUDGET, with a floor of
			// MIN_DHL_CHECK_INTERVAL_MS so a single package still isn't checked
			// unnecessarily often.
			const cyclesPerDay = DHL_DAILY_REQUEST_BUDGET / dhlItems.length
			const minInterval = Math.max(MIN_DHL_CHECK_INTERVAL_MS, (24 * 60 * 60 * 1000) / cyclesPerDay)
			if (Date.now() - this.lastDhlCheckAt < minInterval) return

			this.lastDhlCheckAt = Date.now()
			this.save()
			dhlItems.forEach(item => this.refreshOne(item))
		},

		save() {
			this.$api.users.setCustomStorage(packagesConfig, {
				dhlApiKey: this.dhlApiKey,
				items: this.items,
				lastDhlCheckAt: this.lastDhlCheckAt,
			})
		},
	},
}
</script>

<style lang="scss" scoped>
.packages {
	.package-row {
		border-top: 1px solid $border;
		padding-top: 0.5rem;

		&:first-of-type {
			border-top: none;
			padding-top: 0;
		}
	}
}
</style>

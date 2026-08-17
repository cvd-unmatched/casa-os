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
			// Not persisted - live DHL lookups are re-checked each time the
			// widget mounts rather than cached, so status can't go stale.
			statuses: {},
		}
	},
	mounted() {
		this.$api.users.getCustomStorage(packagesConfig).then((res) => {
			const saved = res.data.data
			if (saved) {
				this.dhlApiKey = saved.dhlApiKey || ''
				this.items = saved.items || []
				this.refreshDhlStatuses()
			}
		})
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
			this.$set(this.statuses, item.id, undefined)
			const status = await this.$dhl.trackShipment(this.dhlApiKey, item.trackingNumber)
			this.$set(this.statuses, item.id, status)
		},

		refreshDhlStatuses() {
			if (!this.dhlApiKey) return
			this.items
				.filter(item => item.carrier === 'dhl')
				.forEach(item => this.refreshOne(item))
		},

		save() {
			this.$api.users.setCustomStorage(packagesConfig, { dhlApiKey: this.dhlApiKey, items: this.items })
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

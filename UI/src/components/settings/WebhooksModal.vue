<script>
const EVENT_TYPES = [
	{ key: 'container_crash', label: 'Container crashes' },
	// the auto-updater fires three related but distinct event types
	// (detected, applied, failed) - a single checkbox controls all three
	// so "Docker image updates" behaves as one on/off toggle instead of
	// requiring three near-identical checkboxes, while still letting
	// webhook.Send's exact eventType match actually deliver all three.
	{ key: 'image_update', label: 'Docker image updates', relatedKeys: ['image_update', 'image_update_applied', 'image_update_failed'] },
	{ key: 'disk_warning', label: 'Disk space warnings' },
	{ key: 'package_update', label: 'Package delivery updates' },
]

const DESTINATION_TYPES = [
	{ key: 'discord', label: 'Discord' },
	{ key: 'slack', label: 'Slack' },
	{ key: 'generic', label: 'Generic JSON' },
]

export default {
	name: 'WebhooksModal',
	data() {
		return {
			isLoading: true,
			diskWarningThresholdPercent: 90,
			destinations: [],
			newDestination: this.emptyDestination(),
			testingId: null,
			eventTypes: EVENT_TYPES,
			destinationTypes: DESTINATION_TYPES,
		}
	},
	created() {
		this.load()
	},
	methods: {
		emptyDestination() {
			return { id: '', name: '', type: 'discord', url: '', events: [] }
		},

		async load() {
			this.isLoading = true
			try {
				const res = await this.$api.sys.getWebhooks()
				const cfg = res.data.data || {}
				this.diskWarningThresholdPercent = cfg.diskWarningThresholdPercent || 90
				this.destinations = cfg.destinations || []
			}
			catch (error) {
				this.$buefy.toast.open({ message: this.$t('Could not load webhook settings.'), type: 'is-danger' })
			}
			finally {
				this.isLoading = false
			}
		},

		async save() {
			try {
				await this.$api.sys.setWebhooks({
					diskWarningThresholdPercent: Number(this.diskWarningThresholdPercent),
					destinations: this.destinations,
				})
				this.$buefy.toast.open({ message: this.$t('Saved'), type: 'is-success' })
			}
			catch (error) {
				this.$buefy.toast.open({ message: this.$t('Could not save webhook settings.'), type: 'is-danger' })
			}
		},

		addDestination() {
			if (!this.newDestination.name.trim() || !this.newDestination.url.trim())
				return
			this.destinations.push({
				...this.newDestination,
				id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
			})
			this.newDestination = this.emptyDestination()
			this.save()
		},

		removeDestination(id) {
			this.destinations = this.destinations.filter(d => d.id !== id)
			this.save()
		},

		toggleEvent(destination, eventKey) {
			const eventType = EVENT_TYPES.find(e => e.key === eventKey)
			const keys = (eventType && eventType.relatedKeys) || [eventKey]
			if (destination.events.includes(eventKey))
				destination.events = destination.events.filter(e => !keys.includes(e))
			else
				destination.events = Array.from(new Set([...destination.events, ...keys]))
			this.save()
		},

		async sendTest(destination) {
			this.testingId = destination.id || 'new'
			try {
				await this.$api.sys.testWebhook(destination.type, destination.url)
				this.$buefy.toast.open({ message: this.$t('Test notification sent.'), type: 'is-success' })
			}
			catch (error) {
				this.$buefy.toast.open({
					message: error.response?.data?.message || this.$t('Test notification failed.'),
					type: 'is-danger',
				})
			}
			finally {
				this.testingId = null
			}
		},
	},
}
</script>

<template>
	<div class="modal-card webhooks-modal">
		<header class="modal-card-head">
			<p class="modal-card-title">
				{{ $t('Webhooks') }}
			</p>
			<b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
		</header>
		<section class="modal-card-body">
			<p class="mb-4 has-text-grey">
				{{ $t('Get notified in Discord, Slack, or any endpoint that accepts a JSON POST when something happens.') }}
			</p>

			<b-loading v-model="isLoading" :is-full-page="false" />

			<template v-if="!isLoading">
				<b-field :label="$t('Disk warning threshold (%)')" class="mb-4">
					<b-input v-model="diskWarningThresholdPercent" type="number" min="1" max="100" @blur="save" />
				</b-field>

				<div
					v-for="destination in destinations" :key="destination.id"
					class="destination-card mb-3 p-3"
				>
					<div class="is-flex is-align-items-center mb-2">
						<b-icon class="mr-2" icon="share-outline" pack="casa" size="is-small" />
						<span class="is-flex-grow-1 has-text-weight-semibold">{{ destination.name }}</span>
						<span class="tag is-size-7 mr-2">{{ destination.type }}</span>
						<b-button
							class="mr-2" rounded size="is-small" type="is-dark"
							:loading="testingId === destination.id" @click="sendTest(destination)"
						>
							{{ $t('Send test') }}
						</b-button>
						<b-icon class="is-clickable" icon="trash-outline" pack="casa" size="is-small" @click.native="removeDestination(destination.id)" />
					</div>
					<div class="is-flex is-flex-wrap-wrap">
						<label
							v-for="event in eventTypes" :key="event.key"
							class="event-option is-flex is-align-items-center mr-3 mb-1 is-clickable"
						>
							<b-checkbox :value="destination.events.includes(event.key)" size="is-small" @input="toggleEvent(destination, event.key)" />
							<span class="ml-1 is-size-7">{{ $t(event.label) }}</span>
						</label>
					</div>
				</div>

				<div class="add-destination p-3">
					<p class="has-text-weight-semibold mb-2">
						{{ $t('Add destination') }}
					</p>
					<b-field :label="$t('Name')" class="mb-2">
						<b-input v-model="newDestination.name" :placeholder="$t('e.g. My Discord')" />
					</b-field>
					<b-field :label="$t('Type')" class="mb-2">
						<b-select v-model="newDestination.type" expanded>
							<option v-for="type in destinationTypes" :key="type.key" :value="type.key">
								{{ type.label }}
							</option>
						</b-select>
					</b-field>
					<b-field :label="$t('Webhook URL')" class="mb-2">
						<b-input v-model="newDestination.url" :placeholder="$t('https://...')" />
					</b-field>
					<b-button rounded size="is-small" type="is-dark" :disabled="!newDestination.name.trim() || !newDestination.url.trim()" @click="addDestination">
						{{ $t('Add') }}
					</b-button>
				</div>
			</template>
		</section>
	</div>
</template>

<style lang="scss" scoped>
.webhooks-modal {
	.modal-card-body {
		min-height: 12rem;
		position: relative;
	}

	.destination-card,
	.add-destination {
		border: 1px solid $border;
		border-radius: 8px;
	}
}
</style>

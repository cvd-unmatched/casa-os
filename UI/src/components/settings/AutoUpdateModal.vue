<script>
const POLICIES = [
	{ key: 'notify', label: 'Notify only' },
	{ key: 'auto', label: 'Auto-update' },
	{ key: 'off', label: 'Off' },
]

export default {
	name: 'AutoUpdateModal',
	data() {
		return {
			isLoading: true,
			apps: [],
			policies: POLICIES,
			rechecking: null,
		}
	},
	created() {
		this.load()
	},
	methods: {
		async load() {
			this.isLoading = true
			try {
				const res = await this.$api.autoupdate.listApps()
				this.apps = res.data.data || []
			}
			catch (error) {
				this.$buefy.toast.open({ message: this.$t('Could not load auto-update settings.'), type: 'is-danger' })
			}
			finally {
				this.isLoading = false
			}
		},

		async setPolicy(app, policy) {
			const previous = app.policy
			app.policy = policy
			try {
				await this.$api.autoupdate.setPolicy(app.name, policy)
			}
			catch (error) {
				app.policy = previous
				this.$buefy.toast.open({ message: this.$t('Could not save policy for {name}.', { name: app.name }), type: 'is-danger' })
			}
		},

		async recheck(app) {
			this.rechecking = app.name
			try {
				const res = await this.$api.autoupdate.recheck(app.name)
				Object.assign(app, res.data.data)
			}
			catch (error) {
				this.$buefy.toast.open({ message: this.$t('Could not check {name} for updates.', { name: app.name }), type: 'is-danger' })
			}
			finally {
				this.rechecking = null
			}
		},
	},
}
</script>

<template>
	<div class="modal-card autoupdate-modal">
		<header class="modal-card-head">
			<p class="modal-card-title">
				{{ $t('Auto-Update') }}
			</p>
			<b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
		</header>
		<section class="modal-card-body">
			<p class="mb-4 has-text-grey">
				{{ $t('Checks each app\'s image against its registry for a newer version. Nothing auto-updates unless you set it to "Auto-update" - everything else defaults to notify-only.') }}
			</p>

			<b-loading v-model="isLoading" :is-full-page="false" />

			<template v-if="!isLoading">
				<div v-if="apps.length === 0" class="has-text-grey-100 is-size-7 py-2">
					{{ $t('No managed apps found.') }}
				</div>

				<div v-for="app in apps" :key="app.appType + ':' + app.name" class="app-row mb-2 p-3">
					<div class="is-flex is-align-items-center">
						<span class="is-flex-grow-1 has-text-weight-semibold one-line" :title="app.currentImage">{{ app.name }}</span>
						<b-icon
							class="is-clickable mr-2" icon="restart-outline" pack="casa" size="is-small"
							:class="{ spinning: rechecking === app.name }" @click.native="recheck(app)"
						/>
						<b-select :value="app.policy" size="is-small" @input="setPolicy(app, $event)">
							<option v-for="p in policies" :key="p.key" :value="p.key">
								{{ $t(p.label) }}
							</option>
						</b-select>
					</div>
					<div class="is-size-7 has-text-grey mt-1">
						<template v-if="app.isUncontrolled">
							{{ $t('Uncontrolled - not eligible for auto-update.') }}
						</template>
						<template v-else-if="app.updateAvailable">
							{{ app.currentTag }} &rarr; <span class="has-text-weight-semibold">{{ app.latestKnownTag }}</span> {{ $t('available') }}
						</template>
						<template v-else-if="app.latestKnownTag">
							{{ $t('Up to date') }} ({{ app.currentTag }})
						</template>
						<template v-else>
							{{ app.currentTag }} - {{ $t('no comparable version tags found') }}
						</template>
					</div>
				</div>
			</template>
		</section>
	</div>
</template>

<style lang="scss" scoped>
.autoupdate-modal {
	.modal-card-body {
		min-height: 12rem;
		position: relative;
	}

	.app-row {
		border: 1px solid $border;
		border-radius: 8px;
	}

	.one-line {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}

	.spinning {
		animation: autoupdate-modal-spin 0.8s linear infinite;
	}

	@keyframes autoupdate-modal-spin {
		from {
			transform: rotate(0deg);
		}

		to {
			transform: rotate(360deg);
		}
	}
}
</style>

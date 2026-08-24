<script>
import { OPEN_APP_HOST_PREFERENCE_KEY, invalidateOpenAppHostCache } from '@/utils/openAppHost'

export default {
	name: 'OpenAppHostModal',
	data() {
		return {
			mode: 'current',
			accessIps: { lan_ips: [], tailscale_ip: '' },
			loading: true,
			saving: false,
		}
	},
	created() {
		Promise.all([
			this.$api.sys.getAccessIPs().then(res => res.data.data).catch(() => ({ lan_ips: [], tailscale_ip: '' })),
			this.$api.users.getCustomStorage(OPEN_APP_HOST_PREFERENCE_KEY).then(res => res.data.data.data).catch(() => null),
		]).then(([accessIps, preference]) => {
			this.accessIps = accessIps || { lan_ips: [], tailscale_ip: '' }
			this.mode = (preference && preference.mode) || 'current'
		}).finally(() => {
			this.loading = false
		})
	},
	methods: {
		choose(mode) {
			if (this.mode === mode)
				return
			this.mode = mode
			this.saving = true
			this.$api.users.setCustomStorage(OPEN_APP_HOST_PREFERENCE_KEY, { data: { mode } }).finally(() => {
				this.saving = false
			})
			invalidateOpenAppHostCache()
		},
	},
}
</script>

<template>
	<div class="modal-card open-app-host-modal">
		<header class="modal-card-head">
			<p class="modal-card-title">
				{{ $t('App Links') }}
			</p>
			<b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
		</header>
		<section class="modal-card-body">
			<p class="mb-4 has-text-grey">
				{{ $t('Clicking an app normally opens it at this page\'s own address. If that address doesn\'t route to the app\'s port - a Cloudflare Tunnel domain, for example - pick a different address to use instead.') }}
			</p>

			<b-loading v-model="loading" :is-full-page="false" />

			<template v-if="!loading">
				<label class="host-option is-flex is-align-items-center mb-2 p-3" @click="choose('current')">
					<b-radio v-model="mode" native-value="current" class="mr-2" @input="choose('current')" />
					<span class="is-flex-grow-1">
						{{ $t('Current address (default)') }}
						<span class="is-block is-size-7 has-text-grey">{{ $t('Whatever address this page is open at right now') }}</span>
					</span>
				</label>

				<label
					class="host-option is-flex is-align-items-center mb-2 p-3"
					:class="{ 'is-disabled': !accessIps.lan_ips.length }"
					@click="accessIps.lan_ips.length && choose('lan')"
				>
					<b-radio
						v-model="mode" native-value="lan" class="mr-2" :disabled="!accessIps.lan_ips.length"
						@input="choose('lan')"
					/>
					<span class="is-flex-grow-1">
						{{ $t('Local network IP') }}
						<span class="is-block is-size-7 has-text-grey">
							{{ accessIps.lan_ips.length ? accessIps.lan_ips.join(', ') : $t('No local network address detected') }}
						</span>
					</span>
				</label>

				<label
					class="host-option is-flex is-align-items-center mb-2 p-3"
					:class="{ 'is-disabled': !accessIps.tailscale_ip }"
					@click="accessIps.tailscale_ip && choose('tailscale')"
				>
					<b-radio
						v-model="mode" native-value="tailscale" class="mr-2" :disabled="!accessIps.tailscale_ip"
						@input="choose('tailscale')"
					/>
					<span class="is-flex-grow-1">
						{{ $t('Tailscale IP') }}
						<span class="is-block is-size-7 has-text-grey">
							{{ accessIps.tailscale_ip || $t('No Tailscale address detected on this server') }}
						</span>
					</span>
				</label>
			</template>
		</section>
	</div>
</template>

<style lang="scss" scoped>
.open-app-host-modal {
	.modal-card-body {
		min-height: 8rem;
	}

	.host-option {
		border: 1px solid $border;
		border-radius: 8px;
		cursor: pointer;

		&.is-disabled {
			cursor: not-allowed;
			opacity: 0.5;
		}
	}
}
</style>

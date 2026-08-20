<template>
	<div class="widget has-text-white app-network-usage is-relative">
		<div class="blur-background"></div>
		<div class="widget-content">
			<!-- Header Start -->
			<div class="widget-header is-flex">
				<div class="widget-title is-flex-grow-1">
					{{ $t('App Network Usage') }}
				</div>
				<b-icon
					class="is-clickable" icon="restart-outline" pack="casa"
					size="is-small" :class="{ spinning: isLoading }" @click.native="load"
				/>
			</div>
			<!-- Header End -->

			<div v-if="topApps.length === 0" class="has-text-grey-100 is-size-7 py-2">
				{{ $t('No active network traffic from installed apps.') }}
			</div>

			<div v-for="app in topApps" :key="app.title" class="app-row mb-2 is-flex is-align-items-center">
				<b-image
					v-if="app.icon" :src="app.icon"
					:src-fallback="require('@/assets/img/app/default.svg')" class="app-icon mr-2"
				/>
				<b-icon v-else class="mr-2" icon="docker-outline" pack="casa" size="is-small" />
				<span class="one-line is-flex-grow-1 is-size-7" :title="app.title">{{ app.title }}</span>
				<span class="rate is-size-7 has-text-grey-100 ml-2">
					<b-icon class="up" icon="up-arrow" pack="casa" size="is-small" />{{ renderSize(app.txBytesPerSec) }}/s
					<b-icon class="down ml-2" icon="down-arrow" pack="casa" size="is-small" />{{ renderSize(app.rxBytesPerSec) }}/s
				</span>
			</div>

			<!-- Recent connections (secondary extension) Start -->
			<div v-if="topConnections.length > 0" class="connections-section mt-3 pt-2">
				<div class="has-text-weight-semibold is-size-7 mb-1">
					{{ $t('Recent connections') }}
				</div>
				<div
					v-for="conn in topConnections" :key="conn.remoteIp"
					class="connection-row is-flex is-align-items-center is-size-7 has-text-grey-100"
				>
					<span class="one-line is-flex-grow-1">{{ conn.remoteIp }}</span>
					<span class="ml-2">{{ conn.count }}×</span>
					<span v-if="conn.localPorts.length > 0" class="ml-2">:{{ conn.localPorts[0] }}</span>
				</div>
			</div>
			<!-- Recent connections End -->
		</div>
	</div>
</template>

<script>
import { mixin } from '@/mixins/mixin'

export default {
	// eslint-disable-next-line vue/multi-word-component-names
	name: 'appNetworkUsage',
	icon: 'network-outline',
	title: 'App Network Usage',
	initShow: false,
	mixins: [mixin],
	data() {
		return {
			isLoading: false,
			apps: [],
			connections: [],
			pollTimer: null,
		}
	},
	computed: {
		topApps() {
			return [...this.apps]
				.sort((a, b) => (b.rxBytesPerSec + b.txBytesPerSec) - (a.rxBytesPerSec + a.txBytesPerSec))
				.slice(0, 5)
		},
		topConnections() {
			return this.connections.slice(0, 5)
		},
	},
	mounted() {
		this.load()
		this.pollTimer = setInterval(() => this.load(), 10000)
	},
	beforeDestroy() {
		clearInterval(this.pollTimer)
	},
	methods: {
		async load() {
			this.isLoading = true
			try {
				const [usageRes, connRes] = await Promise.all([
					this.$api.container.getUsage(),
					this.$api.sys.getConnections(),
				])
				this.apps = (usageRes.data.data || []).map(item => ({
					title: item.title,
					icon: item.icon,
					rxBytesPerSec: item.network_rx_bytes_per_sec || 0,
					txBytesPerSec: item.network_tx_bytes_per_sec || 0,
				}))
				this.connections = connRes.data.data || []
			}
			catch (error) {
				// best-effort widget - leave whatever was last successfully
				// loaded displayed rather than blanking it on a transient error
			}
			finally {
				this.isLoading = false
			}
		},
	},
}
</script>

<style lang="scss" scoped>
.app-network-usage {
	// hard safety net - nothing should ever visually escape the widget box,
	// even if a row's content still doesn't fully fit after wrapping
	overflow-x: hidden;

	.app-icon {
		width: 1.25rem;
		height: 1.25rem;
		border-radius: 0.25rem;
		object-fit: cover;
	}

	.app-row {
		// the rate span (flex-shrink: 0, since compressing the numbers
		// would make them misleading) can be wider than the remaining
		// space once the title has shrunk as far as it can - letting the
		// row wrap drops it to its own line instead of overflowing the
		// widget's right edge.
		flex-wrap: wrap;
	}

	.rate {
		flex-shrink: 0;
		margin-left: auto;

		.up {
			color: rgb(0, 143, 251);
		}

		.down {
			color: rgb(0, 227, 150);
		}
	}

	.one-line {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		// a flex child defaults to min-width: auto, which blocks shrinking
		// below its content's natural width - without this, a long app
		// name pushes past its row instead of actually ellipsizing.
		min-width: 0;
	}

	.connections-section {
		border-top: 1px solid $border;
	}

	.connection-row {
		padding: 0.125rem 0;
	}

	.spinning {
		animation: app-network-usage-spin 1s linear infinite;
	}

	@keyframes app-network-usage-spin {
		from {
			transform: rotate(0deg);
		}

		to {
			transform: rotate(360deg);
		}
	}
}
</style>

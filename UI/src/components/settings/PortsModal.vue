<script>
export default {
	name: 'PortsModal',
	data() {
		return {
			ports: [],
			isLoading: true,
			error: '',
		}
	},
	created() {
		this.load()
	},
	methods: {
		load() {
			this.isLoading = true
			this.error = ''
			this.$api.port.listUsage().then((res) => {
				this.ports = res.data.data || []
			}).catch(() => {
				this.error = this.$t('Could not load port usage.')
			}).finally(() => {
				this.isLoading = false
			})
		},
	},
}
</script>

<template>
	<div class="modal-card ports-modal">
		<header class="modal-card-head">
			<p class="modal-card-title">
				{{ $t('Port Usage') }}
			</p>
			<b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
		</header>
		<section class="modal-card-body">
			<b-loading v-model="isLoading" :is-full-page="false" />

			<p v-if="error" class="has-text-danger is-size-7">
				{{ error }}
			</p>
			<p v-else-if="!isLoading && ports.length === 0" class="has-text-grey is-size-7">
				{{ $t('No published ports found.') }}
			</p>

			<div
				v-for="(port, index) in ports"
				:key="`${port.app_name}-${port.service_name}-${port.published}-${port.protocol}-${index}`"
				class="port-row mb-2 is-flex is-align-items-center"
			>
				<span class="tag is-dark port-number">{{ port.published }}/{{ port.protocol }}</span>
				<span class="is-flex-grow-1 ml-2 one-line" :title="port.display_name">{{ port.display_name }}</span>
				<span class="is-size-7 has-text-grey ml-2 one-line service-name" :title="port.service_name">{{ port.service_name }}</span>
			</div>
		</section>
	</div>
</template>

<style lang="scss" scoped>
.ports-modal {
	.modal-card-body {
		min-height: 8rem;
		max-height: 24rem;
		overflow-y: auto;
	}

	.port-row {
		border: 1px solid $border;
		border-radius: 8px;
		padding: 0.5rem 0.75rem;
	}

	.port-number {
		flex-shrink: 0;
		font-variant-numeric: tabular-nums;
	}

	.one-line {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.service-name {
		max-width: 8rem;
	}
}
</style>

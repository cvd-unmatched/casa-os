<template>
	<div class="widget has-text-white disk-usage is-relative">
		<div class="blur-background"></div>
		<div class="widget-content">
			<!-- Header Start -->
			<div class="widget-header is-flex">
				<div class="widget-title is-flex-grow-1">
					{{ $t('Disk Usage') }}
				</div>
			</div>
			<!-- Header End -->

			<div v-if="disks.length === 0" class="has-text-grey-100 is-size-7 py-2">
				{{ $t('No mounted disks found.') }}
			</div>

			<div v-for="item in disks" :key="item.mountpoint" class="disk-usage-row mb-3">
				<div class="is-flex is-align-items-center is-justify-content-space-between is-size-7 mb-1">
					<span class="one-line mount-label">{{ item.mountpoint }}</span>
					<span class="has-text-grey-100 is-flex-shrink-0 ml-2">
						{{ renderSize(item.used) }} / {{ renderSize(item.total) }}
					</span>
				</div>
				<b-progress
					:type="item.usedPercent | getProgressType" :value="item.usedPercent" class="mt-1"
					size="is-small"
				></b-progress>
			</div>
		</div>
	</div>
</template>

<script>
import { mixin } from '@/mixins/mixin'

export default {
	// eslint-disable-next-line vue/multi-word-component-names
	name: 'diskUsage',
	icon: 'storage-outline',
	title: 'Disk Usage',
	initShow: true,
	mixins: [mixin],
	data() {
		return {
			disks: [],
			timer: null,
		}
	},
	mounted() {
		this.getDisksUsage()
		this.timer = setInterval(() => {
			this.getDisksUsage()
		}, 30000)
	},
	beforeDestroy() {
		clearInterval(this.timer)
	},
	methods: {
		getDisksUsage() {
			this.$api.sys.getAllDisksUsage().then((res) => {
				if (res.data.success === 200)
					this.disks = res.data.data || []
			})
		},
	},
}
</script>

<style lang="scss">
.disk-usage {
	.mount-label {
		max-width: 60%;
	}

	.progress {
		border-radius: 6px;
		height: 8px;

		&::-webkit-progress-bar {
			background: rgba(172, 184, 195, 0.4);
		}

		&::-webkit-progress-value {
			opacity: 1;
			border-radius: 6px;
		}
	}
}
</style>

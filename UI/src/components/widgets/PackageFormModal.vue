<script>
export default {
	name: 'PackageFormModal',
	props: {
		carriers: {
			type: Array,
			required: true,
		},
	},
	data() {
		return {
			nickname: '',
			carrier: this.carriers[0].id,
			trackingNumber: '',
		}
	},
	methods: {
		save() {
			if (!this.trackingNumber.trim())
				return
			this.$emit('save', {
				nickname: this.nickname.trim(),
				carrier: this.carrier,
				trackingNumber: this.trackingNumber.trim(),
			})
			this.$emit('close')
		},
	},
}
</script>

<template>
	<div class="modal-card package-form-modal">
		<header class="modal-card-head">
			<p class="modal-card-title">
				{{ $t('Track a package') }}
			</p>
			<b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
		</header>
		<section class="modal-card-body">
			<b-field :label="$t('Carrier')">
				<b-select v-model="carrier" expanded>
					<option v-for="c in carriers" :key="c.id" :value="c.id">
						{{ c.label }}
					</option>
				</b-select>
			</b-field>
			<b-field :label="$t('Tracking number')">
				<b-input v-model="trackingNumber" :placeholder="$t('e.g. 1234567890')" @keyup.native.enter="save" />
			</b-field>
			<b-field :label="$t('Nickname (optional)')">
				<b-input v-model="nickname" :placeholder="$t('e.g. New headphones')" @keyup.native.enter="save" />
			</b-field>
		</section>
		<footer class="modal-card-foot is-justify-content-flex-end">
			<b-button rounded type="is-dark" :disabled="!trackingNumber.trim()" @click="save">
				{{ $t('Add') }}
			</b-button>
		</footer>
	</div>
</template>

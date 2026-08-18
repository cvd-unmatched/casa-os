<template>
	<div class="overlay">
		<b-loading v-model="isLoading" :is-full-page="false"></b-loading>
		<header class="modal-card-head">
			<div class="is-flex-grow-1 is-flex ">
				<!-- Title Start -->
				<h3 class="title is-5 one-line mr-4">{{ item.name }}</h3>
				<!-- Title End -->
			</div>
			<div class="is-flex is-align-items-center">
				<!-- Download File Button Start -->
				<b-button :label="$t('Download')" class="mr-2" icon-left="download" rounded size="is-small"
					type="is-primary" @click="download" />
				<!-- Download File Button End -->
				<!-- Close Button Start -->
				<div class="close-button" @click="$emit('close')">
					<b-icon icon="close-outline" pack="casa"></b-icon>
				</div>
				<!-- Close File Button End -->
			</div>
		</header>

		<!-- Table Start -->
		<div class="is-flex is-justify-content-center is-align-items-center is-flex-grow-1 v-container">
			<div class="scrollbars-light doc-container csv-container">
				<table v-if="rows.length > 0" class="csv-table">
					<thead>
						<tr>
							<th v-for="(cell, i) in rows[0]" :key="'h-' + i">{{ cell }}</th>
						</tr>
					</thead>
					<tbody>
						<tr v-for="(row, r) in rows.slice(1)" :key="'r-' + r">
							<td v-for="(cell, c) in row" :key="'c-' + c">{{ cell }}</td>
						</tr>
					</tbody>
				</table>
				<div v-else-if="!isLoading" class="has-text-grey-100 py-4 px-4">
					{{ $t('This file is empty.') }}
				</div>
			</div>
		</div>
		<!-- Table End -->
	</div>
</template>

<script>
import { mixin } from '@/mixins/mixin'

export default {
	mixins: [mixin],
	props: {
		item: {
			type: Object,
			default: () => {
				return {
					path: '',
					name: '',
				}
			},
		},
	},
	data() {
		return {
			isLoading: true,
			rows: [],
		}
	},
	mounted() {
		this.readFile()
	},
	methods: {
		readFile() {
			this.$api.file.download(this.item.path).then((res) => {
				const text = typeof res.data === 'object' ? JSON.stringify(res.data) : String(res.data)
				this.rows = parseCsv(text)
				this.isLoading = false
			}).catch(() => {
				this.isLoading = false
			})
		},
		download() {
			this.downloadFile(this.item)
		},
	},
}

/**
 * @description: Minimal RFC 4180-ish CSV parser - handles quoted fields
 * (including embedded commas, newlines, and "" as an escaped quote) without
 * pulling in a new dependency for what's otherwise plain delimited text.
 * @param {string} text
 * @return {Array<Array<string>>}
 */
function parseCsv(text) {
	const rows = []
	let row = []
	let field = ''
	let inQuotes = false

	for (let i = 0; i < text.length; i++) {
		const char = text[i]

		if (inQuotes) {
			if (char === '"') {
				if (text[i + 1] === '"') {
					field += '"'
					i++
				}
				else {
					inQuotes = false
				}
			}
			else {
				field += char
			}
			continue
		}

		if (char === '"') {
			inQuotes = true
		}
		else if (char === ',') {
			row.push(field)
			field = ''
		}
		else if (char === '\n' || char === '\r') {
			if (char === '\r' && text[i + 1] === '\n') i++
			row.push(field)
			field = ''
			if (row.length > 1 || row[0] !== '') rows.push(row)
			row = []
		}
		else {
			field += char
		}
	}

	if (field !== '' || row.length > 0) {
		row.push(field)
		if (row.length > 1 || row[0] !== '') rows.push(row)
	}

	return rows
}
</script>

<style lang="scss" scoped>
.csv-container {
	width: 100%;
	height: 100%;
	overflow: auto;
	background-color: $white;
}

.csv-table {
	border-collapse: collapse;
	font-size: 0.8125rem;
	white-space: nowrap;

	th,
	td {
		border: 1px solid rgba(0, 0, 0, 0.1);
		padding: 0.375rem 0.625rem;
		text-align: left;
	}

	th {
		background-color: rgba(0, 0, 0, 0.04);
		font-weight: 600;
		position: sticky;
		top: 0;
	}

	tbody tr:nth-child(even) {
		background-color: rgba(0, 0, 0, 0.02);
	}
}
</style>

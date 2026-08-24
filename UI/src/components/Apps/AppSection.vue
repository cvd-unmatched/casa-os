<template>
	<div class="home-section has-text-left">
		<!-- Title Bar Start -->
		<div class="app-title-bar is-flex is-align-items-center mb-5">
			<app-section-title-tip
				id="appTitle1"
				class="is-flex-grow-1 has-text-sub-04"
				label="Drag icons to sort."
				title="Apps"
			>
			</app-section-title-tip>

			<b-dropdown animation="fade1" aria-role="menu" class="file-dropdown" position="is-bottom-left">
				<template #trigger>
					<b-icon
						class="polymorphic is-clickable has-text-grey-100"
						icon="plus-outline"
						pack="casa"
						size="is-24"
					></b-icon>
				</template>
				<b-dropdown-item aria-role="menuitem" @click="showInstall(0, 'custom')">
					{{ $t('Custom Install APP') }}
				</b-dropdown-item>
				<b-dropdown-item aria-role="menuitem" @click="showExternalLinkPanel">
					{{ $t('Add external link/APP') }}
				</b-dropdown-item>
				<b-dropdown-item aria-role="menuitem" @click="promptNewFolder">
					{{ $t('New Folder') }}
				</b-dropdown-item>
			</b-dropdown>
		</div>
		<!-- Title Bar End -->

		<!-- App List Start -->
		<draggable
			v-model="displayList"
			:draggable="draggable"
			:move="handleDragMove"
			class="app-list contextmenu-canvas"
			tag="div"
			v-bind="dragOptions"
			@end="onDisplaySortEnd"
			@start="drag = true"
		>
			<!-- App Icon Card Start -->
			<template v-if="!isLoading">
				<div
					v-for="item in displayList"
					:id="(item.__folder ? 'folder-' : 'app-') + (item.__folder ? item.id : item.name)"
					:key="(item.__folder ? 'folder-' : 'app-') + (item.__folder ? item.id : item.name)"
					class="handle"
				>
					<app-folder-card
						v-if="item.__folder"
						:folder="item"
						:preview-apps="folderPreviewApps(item)"
						@open="openFolder"
					></app-folder-card>
					<app-card
						v-else
						:item="item"
						@configApp="showConfigPanel"
						@importApp="showContainerPanel"
						@updateState="getList"
					></app-card>
				</div>
			</template>
			<template v-else>
				<div v-for="index in skCount" :id="'app-' + index" :key="'app-' + index" class="handle">
					<app-card-skeleton :index="index"></app-card-skeleton>
				</div>
			</template>
			<!-- App Icon Card End -->
			<!-- <b-loading slot="footer" v-model="isLoading" :is-full-page="false"></b-loading> -->
		</draggable>
		<!-- App List End -->

		<template v-if="oldAppList.length > 0">
			<!-- Title Bar Start -->
			<div class="title-bar is-flex is-align-items-center mt-2rem mb-5">
				<app-section-title-tip
					id="appTitle2"
					class="is-flex-grow-1 has-text-sub-04"
					label="To be rebuilt."
					title="Legacy app (To be rebuilt)."
				>
				</app-section-title-tip>
			</div>
			<!-- Title Bar End -->

			<!-- App List Start -->
			<div class="app-list contextmenu-canvas">
				<!-- Application not imported Start -->
				<div v-for="item in oldAppList" :id="'app-' + item.name" :key="'app-' + item.name" class="handle">
					<app-card
						:isCasa="false"
						:item="item"
						@configApp="showConfigPanel"
						@importApp="showContainerPanel"
						@updateState="getList"
					></app-card>
				</div>
				<!-- Application not imported End -->
			</div>
			<!-- App List End -->
		</template>
	</div>
</template>

<script>
import AppCard from './AppCard.vue'
import AppCardSkeleton from './AppCardSkeleton.vue'
import AppFolderCard from './AppFolderCard.vue'
import AppFolderPanel from './AppFolderPanel.vue'
import AppPanel from './AppPanel.vue'
import ExternalLinkPanel from '@/components/Apps/ExternalLinkPanel'
import AppSectionTitleTip from './AppSectionTitleTip.vue'
import draggable from 'vuedraggable'
import xor from 'lodash/xor'
import concat from 'lodash/concat'
import events from '@/events/events'
import last from 'lodash/last'
import business_ShowNewAppTag from '@/mixins/app/Business_ShowNewAppTag'
import business_LinkApp from '@/mixins/app/Business_LinkApp'
import isEqual from 'lodash/isEqual'
import { ice_i18n } from '@/mixins/base/common-i18n'
import YAML from 'yamljs'
import { nanoid } from 'nanoid'
import { FOLDER_THEMES } from '@/utils/folderThemes'
import { resolveOpenAppHost } from '@/utils/openAppHost'

const SYNCTHING_STORE_ID = 74

// meta_data :: build-in app
const builtInApplications = [
	{
		id: '1',
		name: 'App Store',
		title: {
			en_us: 'App Store'
		},
		icon: require(`@/assets/img/app/appstore.svg`),
		status: 'running',
		app_type: 'system'
	},
	{
		id: '2',
		name: 'Files',
		title: {
			en_us: 'Files'
		},
		icon: require(`@/assets/img/app/files.svg`),
		status: 'running',
		app_type: 'system'
	}
]

const orderConfig = 'app_order'
const groupsConfig = 'app_groups'
const displayOrderConfig = 'app_display_order'
const publicUrlsConfig = 'app_public_urls'

const FOLDER_COLORS = ['#5B8DEF', '#61C454', '#F2994A', '#EB5757', '#9B51E0', '#2D9CDB', '#F2C94C', '#56CCF2']

export default {
	mixins: [business_ShowNewAppTag, business_LinkApp],
	data () {
		return {
			user_id: localStorage.getItem('user_id'),
			appList: [],
			oldAppList: [],
			appConfig: {},
			drag: false,
			isLoading: false,
			isShowing: false,
			importHelpText: 'Click icon to import.',
			appHelpText: 'Drag icons to sort.',
			draggable: '.handle',
			retryCount: 0,
			appListErrorMessage: '',
			skCount: 0,
			ListRefreshTimer: null,
			groups: [],
			displayList: [],
			displayOrder: [],
			publicUrls: {},
			// consecutive-miss counter per app name, keyed by name - see pruneMissingApps()
			missingAppStreak: {}
		}
	},
	components: {
		AppCard,
		AppFolderCard,
		draggable,
		AppSectionTitleTip,
		AppCardSkeleton
	},
	provide () {
		return {
			openAppStore: this.showInstall,
			getFolders: () => this.groups,
			getAppList: () => this.appList,
			getFolderColors: () => FOLDER_COLORS,
			getFolderThemes: () => FOLDER_THEMES,
			createFolder: this.createFolder,
			renameFolder: this.renameFolder,
			changeFolderColor: this.changeFolderColor,
			changeFolderTheme: this.changeFolderTheme,
			deleteFolder: this.deleteFolder,
			moveAppToFolder: this.moveAppToFolder,
			removeAppFromFolder: this.removeAppFromFolder,
			getPublicUrl: appName => this.publicUrls[appName] || '',
			setPublicUrl: this.setPublicUrl
		}
	},
	computed: {
		dragOptions () {
			return {
				animation: 300,
				group: 'description',
				disabled: false,
				ghostClass: 'ghost'
			}
		},
		showDragTip () {
			return this.draggable === '.handle'
		},
		exsitingAppsShow () {
			return this.$store.state.existingAppsSwitch
		}
	},
	created () {
		this.getPublicUrls()
		this.getGroups().then(() => this.getList())
		this.draggable = this.isMobile() ? '' : '.handle'
		this.$EventBus.$on(events.OPEN_APP_STORE_AND_GOTO_SYNCTHING, () => {
			this.showInstall(SYNCTHING_STORE_ID)
		})

		this.$EventBus.$on(events.RELOAD_APP_LIST, () => {
			this.getList()
		})

		this.$EventBus.$on(events.SHOW_CUSTOM_INSTALL_WITH_COMPOSE, (composeYaml) => {
			this.showInstall(0, 'custom', composeYaml)
		})

		this.ListRefreshTimer = setInterval(() => {
			this.getList()
		}, 5000)
	},
	beforeDestroy () {
		this.$EventBus.$off(events.OPEN_APP_STORE_AND_GOTO_SYNCTHING)
		window.removeEventListener('resize', this.getSkCount)

		clearInterval(this.ListRefreshTimer)
	},
	mounted () {
		window.addEventListener('resize', this.getSkCount)
		this.getSkCount()
	},
	methods: {
		isMobile () {
			const userAgent = navigator.userAgent
			const mobileRegex =
				/(phone|pad|pod|iPhone|iPod|ios|iPad|Android|Mobile|BlackBerry|IEMobile|MQQBrowser|JUC|Fennec|wOSBrowser|BrowserNG|WebOS|Symbian|Windows Phone)/i
			const isMobile = mobileRegex.exec(userAgent)
			return isMobile !== null
		},

		getSkCount () {
			const windowWidth = window.innerWidth
			if (windowWidth < 1024) {
				this.skCount = 4
			} else if (windowWidth < 1216) {
				this.skCount = 6
			} else if (windowWidth < 1408) {
				this.skCount = 8
			} else {
				this.skCount = 10
			}
		},

		/**
		 * @description: Fetch the list of installed apps
		 * @return {*} void
		 */
		async getList () {
			try {
				const orgAppList = await this.$openAPI.appGrid.getAppGrid().then(res => res.data.data || [])
				const openAppHost = await resolveOpenAppHost(this.$baseIp)
				let orgOldAppList = [],
					orgNewAppList = []
				orgAppList.forEach(item => {
					item.hostname = item.hostname || openAppHost
					// Container app does not have icon.
					item.icon = item.icon || require(`@/assets/img/app/default.svg`)
					if (item.app_type === 'v1app' || item.app_type === 'container') {
						orgOldAppList.push(item)
					} else {
						orgNewAppList.push(item)
					}
				})
				this.oldAppList = orgOldAppList

				let listLinkApp = await this.getLinkAppList()
				listLinkApp.forEach(item => {
					// linkApp does not have title.
					item.title = {
						en_us: item.name
					}
				})
				// all app list
				let casaAppList = concat(builtInApplications, orgNewAppList, listLinkApp)
				// get app sort info.
				let lateSortList = await this.$api.users
					.getCustomStorage(orderConfig)
					.then(res => res.data.data.data || [])

				// filter anything not in casaAppList.
				const propList = casaAppList.map(obj => obj.name)
				const existingList = lateSortList.filter(item => propList.includes(item))
				const futureList = propList.filter(item => !lateSortList.includes(item))
				const newSortList = existingList.concat(futureList)

				// then sort.
				const sortedAppList = casaAppList.sort((obj1, obj2) => {
					return newSortList.indexOf(obj1.name) - newSortList.indexOf(obj2.name)
				})

				const sortedList = sortedAppList.map(obj => obj.name)
				this.appList = sortedAppList
				if (!isEqual(lateSortList, sortedList)) {
					this.saveSortData()
				}

				this.rebuildDisplayList()

				this.isLoading = false
				this.retryCount = 0
				this.appListErrorMessage = ''
			} catch (error) {
				console.error(error)
				this.isLoading = true
				if (this.retryCount < 5) {
					setTimeout(() => {
						this.retryCount++
						this.getList()
					}, 2000)
				} else {
					this.appListErrorMessage = 'Failed to get app list.'
					this.$buefy.toast.open({
						message: this.$t(`Failed to load apps, please refresh later.`),
						type: 'is-danger'
					})
				}
			}
		},

		/**
		 * @description:
		 * @param {Array} oriList
		 * @param {Array} newList
		 * @return {*}
		 */
		getNewSortList (oriList, newList) {
			let xorList = xor(oriList, newList)
			// xorList.reverse()
			return concat(oriList, xorList)
		},

		/**
		 * @description: Save Sort Table
		 * @param {*}
		 * @return {*}
		 */
		saveSortData () {
			let newList = this.appList.map(item => {
				// compose milestone :: name is unique, global index.
				return item.name
			})
			let data = {
				data: newList
			}
			this.$api.users.setCustomStorage(orderConfig, data)
		},
		/**
		 * @description: Handle on Sort End
		 * @param {*}
		 * @return {*}
		 */
		onSortEnd () {
			this.drag = false
			this.saveSortData()
		},

		/**
		 * @description: Load persisted folders + combined display order
		 * @return {*} Promise
		 */
		async getGroups () {
			this.groups = await this.$api.users
				.getCustomStorage(groupsConfig)
				.then(res => res.data.data.data || [])
				.catch(() => [])
			this.displayOrder = await this.$api.users
				.getCustomStorage(displayOrderConfig)
				.then(res => res.data.data.data || [])
				.catch(() => [])
		},

		saveGroups () {
			this.$api.users.setCustomStorage(groupsConfig, { data: this.groups })
		},

		saveDisplayOrder () {
			this.$api.users.setCustomStorage(displayOrderConfig, { data: this.displayOrder })
		},

		/**
		 * @description: An app's externally-reachable url (e.g. a Cloudflare
		 * Tunnel hostname) isn't part of the app's own compose config, just a
		 * per-user note about where it's reachable - stored the same way as
		 * folders/order, keyed by app name, so it survives across sessions
		 * without needing any backend changes.
		 */
		async getPublicUrls () {
			this.publicUrls = await this.$api.users
				.getCustomStorage(publicUrlsConfig)
				.then(res => res.data.data.data || {})
				.catch(() => ({}))
		},

		savePublicUrls () {
			this.$api.users.setCustomStorage(publicUrlsConfig, { data: this.publicUrls })
		},

		setPublicUrl (appName, url) {
			const trimmed = (url || '').trim()
			if (trimmed) this.$set(this.publicUrls, appName, trimmed)
			else this.$delete(this.publicUrls, appName)
			this.savePublicUrls()
		},

		/**
		 * @description: Drop app names from folders that no longer exist (e.g.
		 * uninstalled) - once they've been missing for several consecutive
		 * getList() polls in a row, not on the first miss. getList() runs every
		 * 5s and its own error handling only catches a fetch that throws; a
		 * fetch that *succeeds* but returns a short/partial app-grid response
		 * (daemon restarting, backend hiccup, container mid-recreate) used to
		 * look identical to a real uninstall and got saved as one immediately,
		 * permanently evicting the app from its folder over a single bad tick.
		 * Also skip entirely on an empty appList - never trust a response with
		 * nothing in it enough to act on it.
		 * @return {*} void
		 */
		pruneMissingApps () {
			if (this.appList.length === 0) return

			const MISS_THRESHOLD = 3
			const validNames = new Set(this.appList.map(item => item.name))
			const referencedNames = new Set()

			this.groups.forEach((group) => {
				group.appNames.forEach((name) => {
					referencedNames.add(name)
					if (validNames.has(name)) {
						delete this.missingAppStreak[name]
					} else {
						this.missingAppStreak[name] = (this.missingAppStreak[name] || 0) + 1
					}
				})
			})

			// forget streaks for names no folder references anymore
			Object.keys(this.missingAppStreak).forEach((name) => {
				if (!referencedNames.has(name)) delete this.missingAppStreak[name]
			})

			let changed = false
			this.groups.forEach((group) => {
				const before = group.appNames.length
				group.appNames = group.appNames.filter(name => (this.missingAppStreak[name] || 0) < MISS_THRESHOLD)
				if (group.appNames.length !== before) changed = true
			})
			if (changed) this.saveGroups()
		},

		/**
		 * @description: Recompute the combined (folders + ungrouped apps) list that drives the grid
		 * @return {*} void
		 */
		rebuildDisplayList () {
			this.pruneMissingApps()

			const grouped = new Set()
			this.groups.forEach(group => group.appNames.forEach(name => grouped.add(name)))

			const ungrouped = this.appList.filter(item => !grouped.has(item.name))
			const folderItems = this.groups.map(group => ({
				__folder: true,
				id: group.id,
				name: group.name,
				appNames: group.appNames,
				color: group.color,
				theme: group.theme
			}))

			const keyOf = item => (item.__folder ? `folder:${item.id}` : item.name)
			const orderIndex = (key) => {
				const idx = this.displayOrder.indexOf(key)
				return idx === -1 ? this.displayOrder.length : idx
			}

			const combined = [...folderItems, ...ungrouped].sort((a, b) => orderIndex(keyOf(a)) - orderIndex(keyOf(b)))

			this.displayList = combined

			const newOrder = combined.map(keyOf)
			if (!isEqual(this.displayOrder, newOrder)) {
				this.displayOrder = newOrder
				this.saveDisplayOrder()
			}
		},

		/**
		 * @description: Handle on Sort End for the combined folders + apps grid
		 * @return {*} void
		 */
		/**
		 * @description: vuedraggable's :move handler - lets us intercept a drag
		 * while it's hovering the CENTER of another tile (rather than near its
		 * edges) and treat that as "drop to merge into a folder" instead of a
		 * normal reorder, matching iOS/Android home-screen behaviour.
		 * Returning false blocks the normal reorder for this cursor position.
		 * @return {boolean}
		 */
		handleDragMove (evt, originalEvent) {
			const draggedItem = evt.draggedContext && evt.draggedContext.element
			const relatedItem = evt.relatedContext && evt.relatedContext.element

			if (!draggedItem || !relatedItem || relatedItem === draggedItem) {
				this.clearMergeTarget()
				return true
			}

			const relatedRect = evt.related && evt.related.getBoundingClientRect ? evt.related.getBoundingClientRect() : null
			const pointer = originalEvent && (typeof originalEvent.clientX === 'number'
				? originalEvent
				: (originalEvent.touches && originalEvent.touches[0]))

			if (!relatedRect || !pointer || draggedItem.__folder) {
				this.clearMergeTarget()
				return true
			}

			// Already armed for this exact target: stay armed as long as the
			// pointer is anywhere within its FULL bounds, not just the smaller
			// zone that armed it. A real mouse isn't perfectly still, and
			// re-checking the tight zone on every single move event meant a
			// tiny jitter right before releasing the mouse would silently
			// cancel the merge and fall back to a plain reorder instead.
			if (this._mergeTargetItem === relatedItem) {
				const stillWithinBounds = pointer.clientX >= relatedRect.left && pointer.clientX <= relatedRect.right
					&& pointer.clientY >= relatedRect.top && pointer.clientY <= relatedRect.bottom
				if (stillWithinBounds)
					return false
				this.clearMergeTarget()
				return true
			}

			// Not yet armed for this target: require the pointer to be well
			// within its center before arming, so normal reordering near the
			// edges of a tile is unaffected.
			const marginX = relatedRect.width * 0.25
			const marginY = relatedRect.height * 0.25
			const withinCenter = pointer.clientX > relatedRect.left + marginX
				&& pointer.clientX < relatedRect.right - marginX
				&& pointer.clientY > relatedRect.top + marginY
				&& pointer.clientY < relatedRect.bottom - marginY

			if (withinCenter) {
				this.setMergeTarget(evt.related, relatedItem, draggedItem)
				return false
			}

			this.clearMergeTarget()
			return true
		},

		setMergeTarget (el, targetItem, draggedItem) {
			if (this._mergeTargetEl && this._mergeTargetEl !== el)
				this._mergeTargetEl.classList.remove('drag-merge-target')
			this._mergeTargetEl = el
			this._mergeTargetItem = targetItem
			this._draggedItem = draggedItem
			el.classList.add('drag-merge-target')
		},

		clearMergeTarget () {
			if (this._mergeTargetEl)
				this._mergeTargetEl.classList.remove('drag-merge-target')
			this._mergeTargetEl = null
			this._mergeTargetItem = null
			this._draggedItem = null
		},

		performMerge (draggedItem, targetItem) {
			if (targetItem.__folder) {
				this.moveAppToFolder(draggedItem.name, targetItem.id)
			}
			else {
				const id = nanoid()
				this.groups.push({ id, name: this.$t('New Folder'), appNames: [targetItem.name, draggedItem.name], color: null })
				this.saveGroups()
				this.rebuildDisplayList()
			}
		},

		onDisplaySortEnd () {
			this.drag = false

			if (this._mergeTargetItem && this._draggedItem) {
				const targetItem = this._mergeTargetItem
				const draggedItem = this._draggedItem
				this.clearMergeTarget()
				this.performMerge(draggedItem, targetItem)
				return
			}
			this.clearMergeTarget()

			this.displayOrder = this.displayList.map(item => (item.__folder ? `folder:${item.id}` : item.name))
			this.saveDisplayOrder()
		},

		folderPreviewApps (folder) {
			return folder.appNames
				.map(name => this.appList.find(item => item.name === name))
				.filter(Boolean)
				.slice(0, 4)
		},

		promptNewFolder () {
			this.$buefy.dialog.prompt({
				message: this.$t('Folder name'),
				inputAttrs: {
					placeholder: this.$t('New Folder'),
					maxlength: 40
				},
				trapFocus: true,
				confirmText: this.$t('Create'),
				onConfirm: (value) => {
					if (value && value.trim()) this.createFolder(value.trim())
				}
			})
		},

		createFolder (name, appName) {
			const id = nanoid()
			this.groups.push({ id, name, appNames: appName ? [appName] : [], color: null })
			this.saveGroups()
			this.rebuildDisplayList()
			return id
		},

		renameFolder (id, name) {
			const group = this.groups.find(g => g.id === id)
			if (!group) return
			group.name = name
			this.saveGroups()
			this.rebuildDisplayList()
		},

		changeFolderColor (id, color) {
			const group = this.groups.find(g => g.id === id)
			if (!group) return
			group.color = color
			// a folder is either a plain color or a theme, never both
			group.theme = null
			this.saveGroups()
			this.rebuildDisplayList()
		},

		changeFolderTheme (id, theme) {
			const group = this.groups.find(g => g.id === id)
			if (!group) return
			group.theme = theme
			group.color = null
			this.saveGroups()
			this.rebuildDisplayList()
		},

		deleteFolder (id) {
			this.groups = this.groups.filter(g => g.id !== id)
			this.saveGroups()
			this.rebuildDisplayList()
		},

		moveAppToFolder (appName, folderId) {
			this.groups.forEach((group) => {
				const idx = group.appNames.indexOf(appName)
				if (idx !== -1) group.appNames.splice(idx, 1)
			})
			const target = this.groups.find(g => g.id === folderId)
			if (target && !target.appNames.includes(appName)) target.appNames.push(appName)
			this.saveGroups()
			this.rebuildDisplayList()
		},

		removeAppFromFolder (appName) {
			let changed = false
			this.groups.forEach((group) => {
				const idx = group.appNames.indexOf(appName)
				if (idx !== -1) {
					group.appNames.splice(idx, 1)
					changed = true
				}
			})
			if (changed) {
				this.saveGroups()
				this.rebuildDisplayList()
			}
		},

		openFolder (folder) {
			this.$buefy.modal.open({
				parent: this,
				component: AppFolderPanel,
				hasModalCard: true,
				customClass: 'app-folder-panel-modal',
				fullScreen: this.isMobile(),
				trapFocus: true,
				canCancel: ['escape', 'outside'],
				scroll: 'keep',
				animation: 'zoom-in',
				events: {
					configApp: (item, isCasa) => this.showConfigPanel(item, isCasa),
					importApp: item => this.showContainerPanel(item),
					updateState: () => this.getList()
				},
				props: {
					folderId: folder.id
				}
			})
		},

		/**
		 * @description: Show Install Panel Programmatic
		 * @return {*} void
		 */
		async showInstall (storeId = 0, mode = '', composeYaml = '') {
			if (mode === 'custom') {
				this.$messageBus('apps_custominstall')
			}
			this.isShowing = true

			const networks = await this.$api.container.getNetworks()
			const memory = this.$store.state.hardwareInfo.mem
			const configData = {
				networks: networks.data.data,
				memory: memory
			}
			this.isShowing = false
			this.$buefy.modal.open({
				parent: this,
				component: AppPanel,
				hasModalCard: true,
				customClass: 'app-panel',
				trapFocus: true,
				canCancel: ['escape'],
				scroll: 'keep',
				animation: 'zoom-in',
				events: {
					updateState: () => {
						this.getList()
					}
				},
				props: {
					id: '0',
					state: 'install',
					configData: configData,
					storeId: storeId,
					settingData: mode !== 'custom' ? undefined : {},
					settingComposeData: composeYaml || undefined
				}
			})
		},

		/**
		 * @description: Show Settings Panel Programmatic
		 * @param {Object} {id:String,status:String }
		 * @param {Boolean} isCasa
		 * @return {*}
		 */
		async showConfigPanel (item, isCasa) {
			let name = item.name
			this.$messageBus('appsexsiting_open', name)
			try {
				if (item?.app_type === 'LinkApp') {
					await this.showExternalLinkPanel(item)
					return
				}
				const networks = await this.$api.container.getNetworks()
				const memory = this.$store.state.hardwareInfo.mem
				const configData = {
					networks: networks.data.data,
					memory: memory
				}
				const ret = await this.$openAPI.appManagement.compose.myComposeApp(name, {
					headers: {
						'content-type': 'application/yaml',
						accept: 'application/yaml'
					}
				})
				this.$buefy.modal.open({
					parent: this,
					component: AppPanel,
					hasModalCard: true,
					customClass: '',
					trapFocus: true,
					canCancel: [''],
					scroll: 'keep',
					animation: 'zoom-in',
					events: {
						updateState: () => {
							this.getList()
						}
					},
					props: {
						id: name,
						state: 'update',
						isCasa: isCasa,
						// 区分 terminal
						runningStatus: item.status,
						configData: configData,
						// settingData: ret.data,
						settingComposeData: ret.data
					}
				})
			} catch (e) {
				console.error(e)
			}
		},

		async showContainerPanel (item) {
			this.$messageBus('appsexsiting_open', item.name)
			let id = item.name
			let networks
			let ret
			try {
				networks = await this.$api.container.getNetworks()
				ret = await this.$api.container.getInfo(id)
			}
			catch (error) {
				this.$buefy.toast.open({
					message: this.$t('Could not load this container\'s details - it may have corrupted state Docker can\'t fully read.'),
					type: 'is-danger',
				})
				return
			}
			const memory = this.$store.state.hardwareInfo.mem
			const configData = {
				networks: networks.data.data,
				memory: memory
			}
			this.$buefy.modal.open({
				parent: this,
				component: AppPanel,
				hasModalCard: true,
				customClass: '',
				trapFocus: true,
				canCancel: [''],
				scroll: 'keep',
				animation: 'zoom-in',
				events: {
					updateState: () => {
						this.getList()
					}
				},
				props: {
					id: id,
					state: 'update',
					isCasa: false,
					runningStatus: item.status,
					configData: configData,
					settingData: ret.data.data
				}
			})
		},

		async showExternalLinkPanel (item = {}) {
			this.$buefy.modal.open({
				parent: this,
				component: ExternalLinkPanel,
				hasModalCard: true,
				customClass: '',
				trapFocus: true,
				canCancel: [''],
				scroll: 'keep',
				animation: 'zoom-in',
				events: {
					updateState: () => {
						this.$messageBus('apps_external')
						this.getList().then(() => {
							this.scrollToNewApp()
						})
					}
				},
				props: {
					linkName: item.name,
					linkHost: item.hostname,
					linkIcon: item.icon
				}
			})
		},

		scrollToNewApp () {
			// business :: scroll to last position
			let name = last(this.newAppIds)
			let showEl = document.getElementById('app-' + name)
			showEl?.scrollIntoView({ behavior: 'smooth', block: 'end' })
		},

		messageBusToast (message, type) {
			let duration = 5000
			this.$buefy.toast.open({
				message: message,
				duration,
				type
			})
		}
	},
	sockets: {
		'app:install-end' () {
			this.getList().then(() => {
				this.scrollToNewApp()
			})
		},
		'app:install-error' () {
			this.getList().then(() => {
				this.scrollToNewApp()
			})
		},
		'app:uninstall-end' () {
			this.getList()
		},
		'app:apply-changes-error' (res) {
			// toast info
			this.messageBusToast(res.Properties.message, 'is-danger')
		},
		'app:apply-changes-end' (res) {
			let languages = JSON.parse(res.Properties['app:title'])
			const title = ice_i18n(languages)
			// toast info
			this.messageBusToast(title + ' is OK', 'is-success')

			// business :: Tagging of new app / scrollIntoView
			this.addIdToSessionStorage(res.Properties['app:name'])

			this.getList().then(() => {
				this.scrollToNewApp()
			})
		},
		/**
		 * @description: Update App Version
		 * @param {Object} data
		 * @return {void}
		 */
		'app:update-end' (data) {
			if (data.Properties['docker:image:updated'] === 'true') {
				// business :: Tagging of new app / scrollIntoView
				this.addIdToSessionStorage(data.Properties['app:name'])

				this.$buefy.toast.open({
					message: this.$t(`{name} has been updated to the latest version!`, {
						name: data.Properties.name
					}),
					type: 'is-success'
				})
				this.getList().then(() => {
					this.scrollToNewApp()
				})
			}
		},
		'app:update-error' (data) {
			if (data.Properties.cid === this.item.id) {
				this.isUpdating = false
				this.$buefy.toast.open({
					message: this.$t(data.Properties['error']),
					type: 'is-danger'
				})
			}
		}
	}
}
</script>

<style lang="scss" scoped>
.app-list {
	position: relative;
	display: grid;
	gap: 1rem;

	@include touch {
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}

	@include desktop {
		grid-template-columns: repeat(4, minmax(0, 1fr));
	}

	@include fullhd {
		grid-template-columns: repeat(5, minmax(0, 1fr));
	}
}

.handle.drag-merge-target {
	> * {
		transform: scale(1.08);
		box-shadow: 0 0 0 3px $casablue, 0 6px 16px rgba(0, 0, 0, 0.3);
		transition: transform 0.15s, box-shadow 0.15s;
	}
}
</style>

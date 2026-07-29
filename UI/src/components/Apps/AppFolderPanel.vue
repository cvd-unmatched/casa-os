<script>
import AppCard from './AppCard.vue'

export default {
  name: 'AppFolderPanel',
  components: {
    AppCard,
  },
  inject: ['getFolders', 'getAppList', 'getFolderColors', 'renameFolder', 'changeFolderColor', 'deleteFolder'],
  props: {
    folderId: {
      type: String,
      required: true,
    },
  },
  computed: {
    folder() {
      return this.getFolders().find(g => g.id === this.folderId)
    },
    folderColors() {
      return this.getFolderColors()
    },
    isPresetColor() {
      return !this.folder.color || this.folderColors.includes(this.folder.color)
    },
    customSwatchStyle() {
      return this.folder.color && !this.isPresetColor
        ? { background: this.folder.color }
        : {}
    },
    apps() {
      if (!this.folder)
        return []
      const names = this.folder.appNames
      return this.getAppList().filter(item => names.includes(item.name))
    },
  },
  watch: {
    folder(value) {
      // the folder was deleted (e.g. from another tab) while this panel was open
      if (!value)
        this.$emit('close')
    },
  },
  methods: {
    rename() {
      this.$buefy.dialog.prompt({
        message: this.$t('Folder name'),
        inputAttrs: {
          value: this.folder.name,
          maxlength: 40,
        },
        trapFocus: true,
        confirmText: this.$t('Save'),
        onConfirm: (value) => {
          if (value && value.trim())
            this.renameFolder(this.folderId, value.trim())
        },
      })
    },
    confirmDelete() {
      this.$buefy.dialog.confirm({
        title: this.$t('Attention'),
        message: this.$t('This only removes the folder. The apps inside it are not uninstalled.'),
        type: 'is-dark',
        confirmText: this.$t('Delete folder'),
        cancelText: this.$t('Cancel'),
        onConfirm: () => {
          this.deleteFolder(this.folderId)
          this.$emit('close')
        },
      })
    },
    onConfigApp(item, isCasa) {
      this.$emit('configApp', item, isCasa)
    },
    onImportApp(item) {
      this.$emit('importApp', item)
    },
    onUpdateState() {
      this.$emit('updateState')
    },
    pickColor(color) {
      this.changeFolderColor(this.folderId, color)
    },
  },
}
</script>

<template>
  <div v-if="folder" class="modal-card app-folder-panel">
    <header class="modal-card-head is-flex is-align-items-center">
      <p class="modal-card-title is-flex-grow-1">
        {{ folder.name }}
      </p>
      <b-icon class="is-clickable mr-4" icon="folder-outline" pack="casa" @click.native="rename" />
      <b-icon class="is-clickable mr-4" icon="trash-outline" pack="casa" @click.native="confirmDelete" />
      <b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
    </header>
    <div class="color-picker is-flex is-align-items-center px-5 pt-4">
      <span class="mr-3 has-text-grey is-size-7">{{ $t('Color') }}</span>
      <span
        class="color-swatch mr-2" :class="{ selected: !folder.color }"
        style="background: rgba(144, 149, 153, 0.4)"
        @click="pickColor(null)"
      />
      <span
        v-for="color in folderColors" :key="color" class="color-swatch mr-2"
        :class="{ selected: folder.color === color }" :style="{ background: color }"
        @click="pickColor(color)"
      />
      <label
        class="color-swatch custom-swatch" :class="{ selected: !isPresetColor }"
        :style="customSwatchStyle" :title="$t('Custom color')"
      >
        <input type="color" class="color-input" :value="folder.color || '#909599'" @input="pickColor($event.target.value)">
      </label>
    </div>
    <section class="modal-card-body">
      <div v-if="apps.length === 0" class="has-text-centered has-text-grey py-6">
        {{ $t('This folder is empty. Move apps into it from their menu.') }}
      </div>
      <div v-else class="app-list contextmenu-canvas">
        <div v-for="item in apps" :id="'folder-app-' + item.name" :key="'folder-app-' + item.name">
          <app-card
            :current-folder-id="folderId"
            :item="item"
            @configApp="onConfigApp"
            @importApp="onImportApp"
            @updateState="onUpdateState"
          />
        </div>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.app-folder-panel {
  .color-swatch {
    display: inline-block;
    width: 1.25rem;
    height: 1.25rem;
    border-radius: 50%;
    cursor: pointer;
    box-sizing: border-box;
    border: 2px solid transparent;
    transition: border-color 0.15s;

    &.selected {
      border-color: hsla(208, 20%, 20%, 1);
    }
  }

  .custom-swatch {
    position: relative;
    overflow: hidden;
    background: conic-gradient(red, yellow, lime, cyan, blue, magenta, red);

    .color-input {
      position: absolute;
      top: -25%;
      left: -25%;
      width: 150%;
      height: 150%;
      border: none;
      padding: 0;
      cursor: pointer;
      opacity: 0;
    }
  }

  .modal-card-body {
    min-height: 12rem;
  }

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
  }
}
</style>

<script>
import { findFolderTheme } from '@/utils/folderThemes'

export default {
  name: 'AppFolderCard',
  props: {
    folder: {
      type: Object,
      required: true,
    },
    previewApps: {
      type: Array,
      default: () => [],
    },
  },
  computed: {
    itemCount() {
      return this.folder.appNames.length
    },
    activeTheme() {
      return this.folder.theme ? findFolderTheme(this.folder.theme) : null
    },
    tintStyle() {
      if (this.activeTheme)
        return { background: this.activeTheme.gradient }
      if (!this.folder.color)
        return {}
      const hex = this.folder.color.replace('#', '')
      const r = parseInt(hex.substring(0, 2), 16)
      const g = parseInt(hex.substring(2, 4), 16)
      const b = parseInt(hex.substring(4, 6), 16)
      return { backgroundColor: `rgba(${r}, ${g}, ${b}, 0.55)` }
    },
  },
  methods: {
    open() {
      this.$emit('open', this.folder)
    },
  },
}
</script>

<template>
  <div
    class="common-card is-flex is-align-items-center is-justify-content-center app-card app-folder-card"
    @click="open"
  >
    <div class="blur-background" :style="tintStyle" />
    <div v-if="activeTheme" class="theme-decorations">
      <span v-for="(deco, idx) in activeTheme.decorations" :key="idx" class="theme-decoration">{{ deco }}</span>
    </div>
    <span v-if="itemCount > 0" class="folder-count">{{ itemCount }}</span>
    <div class="cards-content">
      <div class="has-text-centered is-flex is-justify-content-center is-flex-direction-column pt-5 pb-3px img-c">
        <div class="folder-shape">
          <div class="folder-tab" />
          <div class="folder-preview is-64x64">
            <div v-for="n in 4" :key="n" class="folder-preview-slot">
              <img v-if="previewApps[n - 1]" :src="previewApps[n - 1].icon" alt="">
            </div>
          </div>
        </div>
        <p class="mt-3 one-line">
          <a class="one-line" style="cursor:default">{{ folder.name }}</a>
        </p>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.app-folder-card {
  cursor: pointer;
}

.theme-decorations {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 1;
}

.theme-decoration {
  position: absolute;
  font-size: 1rem;
  line-height: 1;
  opacity: 0.85;
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.3));
  animation: folder-theme-float ease-in-out infinite;

  &:nth-child(1) {
    top: 12%;
    left: 15%;
    animation-duration: 3.2s;
    animation-delay: 0s;
  }

  &:nth-child(2) {
    top: 55%;
    right: 12%;
    animation-duration: 2.6s;
    animation-delay: 0.6s;
  }

  &:nth-child(3) {
    bottom: 10%;
    left: 45%;
    animation-duration: 3.6s;
    animation-delay: 1.2s;
  }
}

@keyframes folder-theme-float {
  0%, 100% {
    transform: translateY(0) rotate(0deg);
    opacity: 0.7;
  }

  50% {
    transform: translateY(-8px) rotate(8deg);
    opacity: 1;
  }
}

.folder-count {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  min-width: 1.125rem;
  height: 1.125rem;
  padding: 0 0.25rem;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 0.65rem;
  line-height: 1.125rem;
  text-align: center;
  z-index: 5;
}

.folder-shape {
  position: relative;
}

.folder-tab {
  position: absolute;
  top: -0.3rem;
  left: 0.3rem;
  width: 45%;
  height: 0.4rem;
  border-radius: 0.25rem 0.25rem 0 0;
  background: rgba(255, 255, 255, 0.35);
}

.folder-preview {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-template-rows: repeat(2, 1fr);
  gap: 3px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.12);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.2);
}

.folder-preview-slot {
  border-radius: 4px;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
}
</style>

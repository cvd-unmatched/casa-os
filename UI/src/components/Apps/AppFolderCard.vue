<script>
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
    <div class="blur-background" />
    <div class="cards-content">
      <div class="has-text-centered is-flex is-justify-content-center is-flex-direction-column pt-5 pb-3px img-c">
        <div class="folder-preview is-64x64">
          <div v-for="n in 4" :key="n" class="folder-preview-slot">
            <img v-if="previewApps[n - 1]" :src="previewApps[n - 1].icon" alt="">
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

.folder-preview {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-template-rows: repeat(2, 1fr);
  gap: 2px;
  padding: 6px;
  border-radius: 8px;
  background: rgba(144, 149, 153, 0.16);
  box-sizing: border-box;
}

.folder-preview-slot {
  border-radius: 3px;
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

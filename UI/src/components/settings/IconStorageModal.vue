<script>
import { mixin } from '@/mixins/mixin'

const iconStorageConfig = 'icon_storage_mountpoint'

export default {
  name: 'IconStorageModal',
  mixins: [mixin],
  data() {
    return {
      disks: [],
      selected: '',
      isLoading: true,
    }
  },
  created() {
    this.load()
  },
  methods: {
    async load() {
      const [disksRes, configRes] = await Promise.all([
        this.$api.sys.getAllDisksUsage(),
        this.$api.users.getCustomStorage(iconStorageConfig),
      ])
      this.disks = disksRes.data.data || []
      this.selected = (configRes.data.data && configRes.data.data.mountpoint) || ''
      this.isLoading = false
    },
    select(mountpoint) {
      this.selected = mountpoint
      this.$api.users.setCustomStorage(iconStorageConfig, { mountpoint })
      this.$buefy.toast.open({
        message: this.$t('Custom icons will be saved on {mountpoint}.', { mountpoint }),
        type: 'is-success',
      })
    },
  },
}
</script>

<template>
  <div class="modal-card icon-storage-modal">
    <header class="modal-card-head">
      <p class="modal-card-title">
        {{ $t('Icon Storage Disk') }}
      </p>
      <b-icon class="is-clickable" icon="close-outline" pack="casa" @click.native="$emit('close')" />
    </header>
    <section class="modal-card-body">
      <p class="mb-4 has-text-grey">
        {{ $t('Custom app icons you upload get saved into a casaos-custom-icons folder on whichever disk you pick here.') }}
      </p>
      <b-loading v-model="isLoading" :is-full-page="false" />
      <div v-if="!isLoading && disks.length === 0" class="has-text-grey">
        {{ $t('No mounted disks found.') }}
      </div>
      <label
        v-for="disk in disks" :key="disk.mountpoint"
        class="disk-option is-flex is-align-items-center mb-2 p-3"
      >
        <input
          type="radio" name="iconStorageDisk" class="mr-3" :value="disk.mountpoint"
          :checked="selected === disk.mountpoint" @change="select(disk.mountpoint)"
        >
        <div class="is-flex-grow-1">
          <div class="has-text-weight-semibold">
            {{ disk.mountpoint }}
          </div>
          <div class="is-size-7 has-text-grey">
            {{ renderSize(disk.used) }} / {{ renderSize(disk.total) }} {{ $t('used') }}
          </div>
        </div>
      </label>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.icon-storage-modal {
  .modal-card-body {
    min-height: 8rem;
    position: relative;
  }

  .disk-option {
    border: 1px solid hsla(208, 16%, 91%, 1);
    border-radius: 8px;
    cursor: pointer;
  }
}
</style>

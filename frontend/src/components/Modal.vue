<template>
  <Teleport to="#modals">
    <div v-show="visible" class="modal-backdrop fade show" @click="closeBackdrop">
      <div class="modal fade show" tabindex="-1">
        <div class="modal-dialog modal-lg modal-dialog-centered modal-dialog-scrollable"  @click.stop="">
          <div class="modal-content" ref="body">
            <div class="modal-header">
              <h5 class="modal-title">{{ title }}</h5>
              <button @click="closeModal" class="btn-close" aria-label="Close"></button>
            </div>
            <div class="modal-body col-md-12">
              <slot></slot>
            </div>
            <div class="modal-footer">
              <slot name="footer"></slot>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style>
.modal.show {
  display:block;
}
.modal.show {
  opacity: 1;
}
.modal-backdrop {
  background-color: rgba(0,0,0,0.6) !important;
}
.modal-backdrop.show {
  opacity: 1 !important;
}
</style>

<script setup>
const props = defineProps({
  title: String,
  visible: Boolean,
  closeOnBackdrop: Boolean,
})

const emit = defineEmits(['close'])

function closeBackdrop() {
  if(props.closeOnBackdrop) {
    console.log("CLOSING BD")
    emit('close')
  }
}

function closeModal() {
  console.log("CLOSING")
  emit('close')
}
</script>
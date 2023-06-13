<template>
  <Teleport to="#dialogs">
    <div v-if="visible" class="modal-backdrop fade show">
      <div class="modal fade show" tabindex="-1">
        <div class="modal-dialog modal-dialog-scrollable"  @click.stop="">
          <div class="modal-content" ref="body">
            <div class="modal-header">
              <h5 class="modal-title">{{ title }}</h5>
            </div>
            <div class="modal-body">
              {{ question }}
            </div>
            <div class="modal-footer pt-0 border-top-0">
              <button type="button" class="btn btn-primary" @click="no">No</button>
              <button type="button" class="btn btn-success" @click="yes">Yes</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style>
</style>

<script setup>
import {ref} from "vue";

const title = ref("Default Title")
const question = ref("Default Question")
const visible = ref(true)

const emit = defineEmits(['no', 'yes'])

function showDialog(titleStr, questionStr) {
  visible.value = true
  title.value = titleStr
  question.value = questionStr
}

function yes() {
  visible.value = false
  console.log("Chosen yes")
  emit('yes')
}

function no() {
  visible.value = false
  console.log("Chosen no")
  emit('no')
}
</script>
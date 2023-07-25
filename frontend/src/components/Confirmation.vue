<script setup>
import {ref} from "vue";
import {useI18n} from "vue-i18n";

const { t } = useI18n()

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
  emit('yes')
}

function no() {
  visible.value = false
  emit('no')
}
</script>

<template>
  <Teleport to="#dialogs">
    <div v-if="visible" class="modal-backdrop fade show">
      <div class="modal fade show" tabindex="-1">
        <div class="modal-dialog modal-dialog-scrollable" @click.stop="">
          <div class="modal-content" ref="body">
            <div class="modal-header">
              <h5 class="modal-title">{{ title }}</h5>
            </div>
            <div class="modal-body">
              {{ question }}
            </div>
            <div class="modal-footer pt-0 border-top-0">
              <button type="button" class="btn btn-primary" @click="no">{{ $t('general.no') }}</button>
              <button type="button" class="btn btn-success" @click="yes">{{ $t('general.yes') }}</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style>
</style>

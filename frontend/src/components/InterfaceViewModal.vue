<script setup>
import Modal from "./Modal.vue";
import {computed, ref, watch} from "vue";
import { useI18n } from 'vue-i18n';
import {interfaceStore} from "@/stores/interfaces";
import Prism from 'vue-prism-component'
import 'prismjs/components/prism-ini'

const { t } = useI18n()

const interfaces = interfaceStore()

const props = defineProps({
  interfaceId: String,
  visible: Boolean,
})

const configString = ref("")

const emit = defineEmits(['close'])

const selectedInterface = computed(() => {
  return interfaces.Find(props.interfaceId)
})

const title = computed(() => {
  if (!props.visible) {
    return "" // otherwise interfaces.GetSelected will die...
  }

  return t("interfaces.interface.show") + ": " + selectedInterface.value.Identifier
})

// functions

watch(() => props.visible, async (newValue, oldValue) => {
      if (oldValue === false && newValue === true) { // if modal is shown
        console.log(selectedInterface.value)
        await interfaces.InterfaceConfig(selectedInterface.value.Identifier)
        configString.value = interfaces.configuration
      }
    }
)

function close() {
  emit('close')
}

</script>

<template>
  <Modal :title="title" :visible="visible" @close="close">
    <template #default>
      <Prism language="ini" :code="configString"></Prism>
    </template>
    <template #footer>
      <button class="btn btn-primary" type="button" @click.prevent="close">Close</button>
    </template>
  </Modal>
</template>

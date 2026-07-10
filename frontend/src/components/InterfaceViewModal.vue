<script setup>
import Modal from "./Modal.vue";
import {computed, ref, watch} from "vue";
import { useI18n } from 'vue-i18n';
import {interfaceStore} from "@/stores/interfaces";
import Prism from '@/helpers/prism-setup';

const { t } = useI18n()

const interfaces = interfaceStore()

const props = defineProps({
  interfaceId: String,
  visible: Boolean,
})

const configString = ref("")

const highlightedConfig = computed(() => {
  const grammar = Prism.languages.ini
  if (grammar) {
    return Prism.highlight(configString.value, grammar, 'ini')
  }
  return configString.value
})

const emit = defineEmits(['close'])

const selectedInterface = computed(() => {
  return interfaces.Find(props.interfaceId)
})

const title = computed(() => {
  if (!props.visible) {
    return "" // otherwise interfaces.GetSelected will die...
  }

  return t("modals.interface-view.headline") + " " + selectedInterface.value.Identifier
})

// functions

watch(() => props.visible, async (newValue, oldValue) => {
      if (oldValue === false && newValue === true) { // if modal is shown
        console.log(selectedInterface.value)
        await interfaces.LoadInterfaceConfig(selectedInterface.value.Identifier)
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
      <pre class="language-ini"><code class="language-ini" v-html="highlightedConfig"></code></pre>
    </template>
    <template #footer>
      <button class="btn btn-primary" type="button" @click.prevent="close">{{ $t('general.close') }}</button>
    </template>
  </Modal>
</template>

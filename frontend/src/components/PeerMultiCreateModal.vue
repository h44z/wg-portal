<script setup>
import Modal from "./Modal.vue";
import {peerStore} from "@/stores/peers";
import {interfaceStore} from "@/stores/interfaces";
import {computed, ref} from "vue";
import { useI18n } from 'vue-i18n';
import { notify } from "@kyvg/vue3-notification";
import { VueTagsInput } from '@vojtechlanka/vue-tags-input';
import { freshInterface } from '@/helpers/models';

const { t } = useI18n()

const peers = peerStore()
const interfaces = interfaceStore()

const props = defineProps({
  visible: Boolean,
})

const emit = defineEmits(['close'])

const selectedInterface = computed(() => {
  let i = interfaces.GetSelected;

  if (!i) {
    i = freshInterface() // dummy interface to avoid 'undefined' exceptions
  }

  return i
})

function freshForm() {
  return {
    Identifiers: [],
    Prefix: "",
  }
}

const currentTag = ref("")
const formData = ref(freshForm())
const isSaving = ref(false)

const title = computed(() => {
  if (!props.visible) {
    return "" // otherwise interfaces.GetSelected will die...
  }
  if (selectedInterface.value.Mode === "server") {
    return t("modals.peer-multi-create.headline-peer")
  } else {
    return t("modals.peer-multi-create.headline-endpoint")
  }
})

function close() {
  formData.value = freshForm()
  emit('close')
}

function handleChangeUserIdentifiers(tags) {
  formData.value.Identifiers = tags.map(tag => tag.text)
}

async function save() {
  if (isSaving.value) return
  isSaving.value = true
  if (formData.value.Identifiers.length === 0) {
    notify({
      title: "Missing Identifiers",
      text: "At least one identifier is required to create a new peer.",
      type: 'error',
    })
    isSaving.value = false
    return
  }

  try {
    await peers.CreateMultiplePeers(selectedInterface.value.Identifier, formData.value)
    close()
  } catch (e) {
    console.log(e)
    notify({
      title: "Failed to create peers!",
      text: e.toString(),
      type: 'error',
    })
  } finally {
    isSaving.value = false
  }
}

</script>

<template>
  <Modal :title="title" :visible="visible" @close="close">
    <template #default>
      <fieldset>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peer-multi-create.identifiers.label') }}</label>
          <vue-tags-input class="form-control" v-model="currentTag"
                          :tags="formData.Identifiers.map(str => ({ text: str }))"
                          :placeholder="$t('modals.peer-multi-create.identifiers.placeholder')"
                          :add-on-key="[13, 188, 32, 9]"
                          :save-on-key="[13, 188, 32, 9]"
                          :allow-edit-tags="true"
                          :separators="[',', ';', ' ']"
                          @tags-changed="handleChangeUserIdentifiers"/>
          <small class="form-text text-muted">{{ $t('modals.peer-multi-create.identifiers.description') }}</small>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peer-multi-create.prefix.label') }}</label>
          <input type="text" class="form-control" :placeholder="$t('modals.peer-multi-create.prefix.placeholder')" v-model="formData.Prefix">
          <small class="form-text text-muted">{{ $t('modals.peer-multi-create.prefix.description') }}</small>
        </div>
      </fieldset>
    </template>
    <template #footer>
      <button class="btn btn-primary me-1" type="button" @click.prevent="save" :disabled="isSaving">
        <span v-if="isSaving" class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span>
        {{ $t('general.save') }}
      </button>
      <button class="btn btn-secondary" type="button" @click.prevent="close">{{ $t('general.close') }}</button>
    </template>
  </Modal>
</template>

<script setup>
import Modal from "./Modal.vue";
import {peerStore} from "@/stores/peers";
import {interfaceStore} from "@/stores/interfaces";
import {computed, ref, watch} from "vue";
import { useI18n } from 'vue-i18n';
import { notify } from "@kyvg/vue3-notification";
import Vue3TagsInput from "vue3-tags-input";
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
    Suffix: "",
  }
}

const formData = ref(freshForm())

function close() {
  formData.value = freshForm()
  emit('close')
}

function handleChangeUserIdentifiers(tags) {
  formData.value.Identifiers = tags
}

async function save() {
  if (formData.value.Identifiers.length === 0) {
    notify({
      title: "Missing Identifiers",
      text: "At least one identifier is required to create a new peer.",
      type: 'error',
    })
    return
  }

  try {
    await peers.CreateMultiplePeers(selectedInterface.value.Identifier, formData.value)
    close()
  } catch (e) {
    console.log(e)
    notify({
      title: "Backend Connection Failure",
      text: "Failed to create peers!",
      type: 'error',
    })
  }
}

</script>

<template>
  <Modal :title="t('modals.peerscreate.title')" :visible="visible" @close="close">
    <template #default>
      <fieldset>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peerscreate.identifiers') }}</label>
          <vue3-tags-input class="form-control" :tags="formData.Identifiers"
                           :placeholder="t('modals.peerscreate.identifiers.placeholder')"
                           :add-tag-on-keys="[13, 188, 32, 9]"
                           @on-tags-changed="handleChangeUserIdentifiers"/>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.peerscreate.peernamesuffix') }}</label>
          <input type="text" class="form-control" :placeholder="t('modals.peerscreate.peernamesuffix.placeholder')" v-model="formData.Suffix">
        </div>
      </fieldset>
    </template>
    <template #footer>
      <button class="btn btn-primary me-1" type="button" @click.prevent="save">Create</button>
      <button class="btn btn-secondary" type="button" @click.prevent="close">Cancel</button>
    </template>
  </Modal>
</template>

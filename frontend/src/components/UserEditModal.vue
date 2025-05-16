<script setup>
import Modal from "./Modal.vue";
import {userStore} from "@/stores/users";
import {computed, ref, watch} from "vue";
import { useI18n } from 'vue-i18n';
import { notify } from "@kyvg/vue3-notification";
import {freshUser} from "@/helpers/models";
import {settingsStore} from "@/stores/settings";

const { t } = useI18n()

const users = userStore()
const settings = settingsStore()

const props = defineProps({
  userId: String,
  visible: Boolean,
})

const emit = defineEmits(['close'])

const selectedUser = computed(() => {
  return users.Find(props.userId)
})

const title = computed(() => {
  if (!props.visible) {
    return "" // otherwise interfaces.GetSelected will die...
  }
  if (selectedUser.value) {
    return t("modals.user-edit.headline-edit") + " " + selectedUser.value.Identifier
  }
  return t("modals.user-edit.headline-new")
})

const formData = ref(freshUser())

const passwordWeak = computed(() => {
  return formData.value.Password && formData.value.Password.length > 0 && formData.value.Password.length < settings.Setting('MinPasswordLength')
})

const formValid = computed(() => {
  if (formData.value.Source !== 'db') {
    return true // nothing to validate
  }
  if (props.userId !== '#NEW#' && passwordWeak.value) {
    return false
  }
  if (props.userId === '#NEW#' && (!formData.value.Password || formData.value.Password.length < 1)) {
    return false
  }
  if (props.userId === '#NEW#' && passwordWeak.value) {
    return false
  }
  if (!formData.value.Identifier || formData.value.Identifier.length < 1) {
    return false
  }
  return true
})


// functions

watch(() => props.visible, async (newValue, oldValue) => {
      if (oldValue === false && newValue === true) { // if modal is shown
        if (!selectedUser.value) {
          formData.value = freshUser()
        } else { // fill existing userdata
          formData.value.Identifier = selectedUser.value.Identifier
          formData.value.Email = selectedUser.value.Email
          formData.value.Source = selectedUser.value.Source
          formData.value.IsAdmin = selectedUser.value.IsAdmin
          formData.value.Firstname = selectedUser.value.Firstname
          formData.value.Lastname = selectedUser.value.Lastname
          formData.value.Phone = selectedUser.value.Phone
          formData.value.Department = selectedUser.value.Department
          formData.value.Notes = selectedUser.value.Notes
          formData.value.Password = ""
          formData.value.Disabled = selectedUser.value.Disabled
          formData.value.Locked = selectedUser.value.Locked
        }
      }
    }
)

function close() {
  formData.value = freshUser()
  emit('close')
}

async function save() {
  try {
    if (props.userId!=='#NEW#') {
      await users.UpdateUser(selectedUser.value.Identifier, formData.value)
    } else {
      await users.CreateUser(formData.value)
    }
    close()
  } catch (e) {
    notify({
      title: "Failed to save user!",
      text: e.toString(),
      type: 'error',
    })
  }
}

async function del() {
  try {
    await users.DeleteUser(selectedUser.value.Identifier)
    close()
  } catch (e) {
    notify({
      title: "Failed to delete user!",
      text: e.toString(),
      type: 'error',
    })
  }
}

</script>

<template>
  <Modal :title="title" :visible="visible" @close="close">
    <template #default>
      <fieldset v-if="formData.Source==='db'">
        <legend class="mt-4">{{ $t('modals.user-edit.header-general') }}</legend>
        <div v-if="props.userId==='#NEW#'" class="form-group">
          <label class="form-label mt-4">{{ $t('modals.user-edit.identifier.label') }}</label>
          <input v-model="formData.Identifier" class="form-control" :placeholder="$t('modals.user-edit.identifier.placeholder')" type="text">
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.user-edit.source.label') }}</label>
          <input v-model="formData.Source" class="form-control" disabled="disabled" :placeholder="$t('modals.user-edit.source.placeholder')" type="text">
        </div>
        <div v-if="formData.Source==='db'" class="form-group">
          <label class="form-label mt-4">{{ $t('modals.user-edit.password.label') }}</label>
          <input v-model="formData.Password" aria-describedby="passwordHelp" class="form-control" :class="{ 'is-invalid': passwordWeak,  'is-valid': formData.Password !== '' && !passwordWeak }" :placeholder="$t('modals.user-edit.password.placeholder')" type="password">
          <div class="invalid-feedback">{{ $t('modals.user-edit.password.too-weak') }}</div>
          <small v-if="props.userId!=='#NEW#'" id="passwordHelp" class="form-text text-muted">{{ $t('modals.user-edit.password.description') }}</small>
        </div>
      </fieldset>
      <fieldset v-if="formData.Source==='db'">
        <legend class="mt-4">{{ $t('modals.user-edit.header-personal') }}</legend>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.user-edit.email.label') }}</label>
          <input v-model="formData.Email" class="form-control" :placeholder="$t('modals.user-edit.email.placeholder')" type="email">
        </div>
        <div class="row">
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.user-edit.firstname.label') }}</label>
            <input v-model="formData.Firstname" class="form-control" :placeholder="$t('modals.user-edit.firstname.placeholder')" type="text">
          </div>
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.user-edit.lastname.label') }}</label>
            <input v-model="formData.Lastname" class="form-control" :placeholder="$t('modals.user-edit.lastname.placeholder')" type="text">
          </div>
        </div>
        <div class="row">
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.user-edit.phone.label') }}</label>
            <input v-model="formData.Phone" class="form-control" :placeholder="$t('modals.user-edit.phone.placeholder')" type="text">
          </div>
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.user-edit.department.label') }}</label>
            <input v-model="formData.Department" class="form-control" :placeholder="$t('modals.user-edit.department.placeholder')" type="text">
          </div>
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">{{ $t('modals.user-edit.header-notes') }}</legend>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.user-edit.notes.label') }}</label>
          <textarea v-model="formData.Notes" class="form-control" rows="2"></textarea>
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">{{ $t('modals.user-edit.header-state') }}</legend>
        <div class="form-check form-switch">
          <input v-model="formData.Disabled" class="form-check-input" type="checkbox">
          <label class="form-check-label" >{{ $t('modals.user-edit.disabled.label') }}</label>
        </div>
        <div class="form-check form-switch">
          <input v-model="formData.Locked" class="form-check-input" type="checkbox">
          <label class="form-check-label" >{{ $t('modals.user-edit.locked.label') }}</label>
        </div>
        <div class="form-check form-switch" v-if="formData.Source==='db'">
          <input v-model="formData.IsAdmin" checked="" class="form-check-input" type="checkbox">
          <label class="form-check-label">{{ $t('modals.user-edit.admin.label') }}</label>
        </div>
      </fieldset>

    </template>
    <template #footer>
      <div class="flex-fill text-start">
        <button v-if="props.userId!=='#NEW#'" class="btn btn-danger me-1" type="button" @click.prevent="del">{{ $t('general.delete') }}</button>
      </div>
      <button class="btn btn-primary me-1" type="button" @click.prevent="save" :disabled="!formValid">{{ $t('general.save') }}</button>
      <button class="btn btn-secondary" type="button" @click.prevent="close">{{ $t('general.close') }}</button>
    </template>
  </Modal>
</template>

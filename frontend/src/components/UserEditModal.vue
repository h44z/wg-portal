<script setup>
import Modal from "./Modal.vue";
import {userStore} from "@/stores/users";
import {computed, ref, watch} from "vue";
import { useI18n } from 'vue-i18n';
import { notify } from "@kyvg/vue3-notification";

const { t } = useI18n()

const users = userStore()

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
    return t("users.edit") + ": " + selectedUser.value.Identifier
  }
  return t("users.new")
})

const formData = ref(freshFormData())

function freshFormData() {
  return {
    Identifier: "",

    Email: "",
    Source: "db",
    IsAdmin: false,

    Firstname: "",
    Lastname: "",
    Phone: "",
    Department: "",
    Notes: "",

    Password: "",

    Disabled: false,
  }
}

// functions

watch(() => props.visible, async (newValue, oldValue) => {
      if (oldValue === false && newValue === true) { // if modal is shown
        if (!selectedUser.value) {
          formData.value = freshFormData()
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
        }
      }
    }
)

function close() {
  formData.value = freshFormData()
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
      <fieldset>
        <legend class="mt-4">General</legend>
        <div v-if="props.userId==='#NEW#'" class="form-group">
          <label class="form-label mt-4">{{ $t('modals.useredit.identifier') }}</label>
          <input v-model="formData.Identifier" class="form-control" placeholder="The user id" type="text">
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.useredit.source') }}</label>
          <input v-model="formData.Source" class="form-control" disabled="disabled" placeholder="The user source" type="text">
        </div>
        <div v-if="formData.Source==='db'" class="form-group">
          <label class="form-label mt-4">{{ $t('modals.useredit.password') }}</label>
          <input v-model="formData.Password" aria-describedby="passwordHelp" class="form-control" placeholder="Password" type="text">
          <small v-if="props.userId!=='#NEW#'" id="passwordHelp" class="form-text text-muted">Leave this field blank to keep current password.</small>
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">User Information</legend>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.useredit.email') }}</label>
          <input v-model="formData.Email" class="form-control" placeholder="Email" type="email">
        </div>
        <div class="row">
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.useredit.firstname') }}</label>
            <input v-model="formData.Firstname" class="form-control" placeholder="Firstname" type="text">
          </div>
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.useredit.lastname') }}</label>
            <input v-model="formData.Lastname" class="form-control" placeholder="Lastname" type="text">
          </div>
        </div>
        <div class="row">
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.useredit.phone') }}</label>
            <input v-model="formData.Phone" class="form-control" placeholder="Phone" type="text">
          </div>
          <div class="form-group col-md-6">
            <label class="form-label mt-4">{{ $t('modals.useredit.department') }}</label>
            <input v-model="formData.Department" class="form-control" placeholder="Department" type="text">
          </div>
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">Notes</legend>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('modals.useredit.notes') }}</label>
          <textarea v-model="formData.Notes" class="form-control" rows="2"></textarea>
        </div>
      </fieldset>
      <fieldset>
        <legend class="mt-4">State</legend>
        <div class="form-check form-switch">
          <input v-model="formData.Disabled" class="form-check-input" type="checkbox">
          <label class="form-check-label" >Disabled</label>
        </div>
        <div class="form-check form-switch">
          <input v-model="formData.IsAdmin" checked="" class="form-check-input" type="checkbox">
          <label class="form-check-label">Is Admin</label>
        </div>
      </fieldset>

    </template>
    <template #footer>
      <div class="flex-fill text-start">
        <button v-if="props.userId!=='#NEW#'" class="btn btn-danger me-1" type="button" @click.prevent="del">Delete</button>
      </div>
      <button class="btn btn-primary me-1" type="button" @click.prevent="save">Save</button>
      <button class="btn btn-secondary" type="button" @click.prevent="close">Discard</button>
    </template>
  </Modal>
</template>

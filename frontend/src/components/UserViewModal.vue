<script setup>
import Modal from "./Modal.vue";
import {userStore} from "../stores/users";
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
  let user = users.Find(props.userId)
  if (user) {
    return user
  }

  return {} // return empty object to avoid "undefined" access problems
})

const title = computed(() => {
  if (!props.visible) {
    return "" // otherwise interfaces.GetSelected will die...
  }
  return t("users.view") + ": " + selectedUser.value.Identifier
})

const userPeers = computed(() => {
  return users.Peers
})

// functions

watch(() => props.visible, async (newValue, oldValue) => {
      if (oldValue === false && newValue === true) { // if modal is shown
        await users.LoadUserPeers(selectedUser.value.Identifier)
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
      <ul class="nav nav-tabs">
        <li class="nav-item">
          <a class="nav-link active" data-bs-toggle="tab" href="#user">User</a>
        </li>
        <li class="nav-item">
          <a class="nav-link" data-bs-toggle="tab" href="#peers">Peers</a>
        </li>
      </ul>
      <div id="interfaceTabs" class="tab-content">
        <div id="user" class="tab-pane fade active show">
          <ul class="list-group list-group-flush">
            <li class="list-group-item">
              User Information:
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>{{ $t('users.label.email') }}:</td>
                  <td>{{selectedUser.Email}}</td>
                </tr>
                <tr>
                  <td>{{ $t('users.label.firstname') }}:</td>
                  <td>{{selectedUser.Firstname}}</td>
                </tr>
                <tr>
                  <td>{{ $t('users.label.lastname') }}:</td>
                  <td>{{selectedUser.Lastname}}</td>
                </tr>
                <tr>
                  <td>{{ $t('users.label.phone') }}:</td>
                  <td>{{selectedUser.Phone}}</td>
                </tr>
                <tr>
                  <td>{{ $t('users.label.department') }}:</td>
                  <td>{{selectedUser.Department}}</td>
                </tr>
                </tbody>
              </table>
            </li>
            <li class="list-group-item">
              Notes:
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr><td>{{selectedUser.Notes}}</td></tr>
                </tbody>
              </table>
            </li>
          </ul>
        </div>
        <div id="peers" class="tab-pane fade">
          <ul v-if="userPeers.length===0" class="list-group list-group-flush">
            <li class="list-group-item">{{ $t('users.nopeers.message') }}</li>
          </ul>

          <table v-if="userPeers.length!==0" id="peerTable" class="table table-sm">
            <thead>
            <tr>
              <th scope="col">{{ $t('user.peers.name') }}</th>
              <th scope="col">{{ $t('user.peers.interface') }}</th>
              <th scope="col">{{ $t('user.peers.ips') }}</th>
              <th scope="col"></th><!-- Actions -->
            </tr>
            </thead>
            <tbody>
            <tr v-for="peer in userPeers" :key="peer.Identifier">
              <td>{{peer.DisplayName}}</td>
              <td>{{peer.InterfaceIdentifier}}</td>
              <td>
                <span v-for="ip in peer.Addresses" :key="ip" class="badge rounded-pill bg-light">{{ ip }}</span>
              </td>
            </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
    <template #footer>
      <button class="btn btn-primary" type="button" @click.prevent="close">Close</button>
    </template>
  </Modal>
</template>

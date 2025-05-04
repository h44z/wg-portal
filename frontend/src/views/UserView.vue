<script setup>
import {userStore} from "@/stores/users";
import {ref,onMounted} from "vue";
import UserEditModal from "../components/UserEditModal.vue";
import UserViewModal from "../components/UserViewModal.vue";

const users = userStore()

const editUserId = ref("")
const viewedUserId = ref("")

const selectAll = ref(false)

function toggleSelectAll() {
  users.FilteredAndPaged.forEach(user => {
    user.IsSelected = selectAll.value;
  });
}

onMounted(() => {
  users.LoadUsers()
})
</script>

<template>
  <UserEditModal :userId="editUserId" :visible="editUserId!==''" @close="editUserId=''"></UserEditModal>
  <UserViewModal :userId="viewedUserId" :visible="viewedUserId!==''" @close="viewedUserId=''"></UserViewModal>

  <!-- User list -->
  <div class="mt-4 row">
    <div class="col-12 col-lg-5">
      <h1>{{ $t('users.headline') }}</h1>
    </div>
    <div class="col-12 col-lg-4 text-lg-end">
      <div class="form-group d-inline">
        <div class="input-group mb-3">
          <input v-model="users.filter" class="form-control" :placeholder="$t('general.search.placeholder')" type="text" @keyup="users.afterPageSizeChange">
          <button class="input-group-text btn btn-primary" :title="$t('general.search.button')"><i class="fa-solid fa-search"></i></button>
        </div>
      </div>
    </div>
    <div class="col-12 col-lg-3 text-lg-end">
      <a class="btn btn-primary ms-2" href="#" :title="$t('users.button-add-user')" @click.prevent="editUserId='#NEW#'">
        <i class="fa fa-plus me-1"></i><i class="fa fa-user"></i>
      </a>
    </div>
  </div>
  <div class="mt-2 table-responsive">
    <div v-if="users.Count===0">
      <h4>{{ $t('users.no-user.headline') }}</h4>
      <p>{{ $t('users.no-user.abstract') }}</p>
    </div>
    <table v-if="users.Count!==0"  id="userTable" class="table table-sm">
      <thead>
        <tr>
          <th scope="col">
            <input class="form-check-input" :title="$t('general.select-all')" type="checkbox" v-model="selectAll" @change="toggleSelectAll">
          </th><!-- select -->
          <th scope="col"></th><!-- status -->
          <th scope="col">{{ $t('users.table-heading.id') }}</th>
          <th scope="col">{{ $t('users.table-heading.email') }}</th>
          <th scope="col">{{ $t('users.table-heading.firstname') }}</th>
          <th scope="col">{{ $t('users.table-heading.lastname') }}</th>
          <th class="text-center" scope="col">{{ $t('users.table-heading.source') }}</th>
          <th class="text-center" scope="col">{{ $t('users.table-heading.peers') }}</th>
          <th class="text-center" scope="col">{{ $t('users.table-heading.admin') }}</th>
          <th scope="col"></th><!-- Actions -->
        </tr>
      </thead>
      <tbody>
        <tr v-for="user in users.FilteredAndPaged" :key="user.Identifier">
          <th scope="row">
            <input class="form-check-input" type="checkbox" v-model="user.IsSelected">
          </th>
          <td class="text-center">
            <span v-if="user.Disabled" class="text-danger" :title="$t('users.user-disabled') + ' ' + user.DisabledReason"><i class="fa fa-circle-xmark"></i></span>
            <span v-if="user.Locked" class="text-danger" :title="$t('users.user-locked') + ' ' + user.LockedReason"><i class="fas fa-lock"></i></span>
          </td>
          <td>{{user.Identifier}}</td>
          <td>{{user.Email}}</td>
          <td>{{user.Firstname}}</td>
          <td>{{user.Lastname}}</td>
          <td class="text-center"><span class="badge rounded-pill bg-light">{{user.Source}}</span></td>
          <td class="text-center">{{user.PeerCount}}</td>
          <td class="text-center">
            <span v-if="user.IsAdmin" class="text-danger" :title="$t('users.admin')"><i class="fa fa-check-circle"></i></span>
            <span v-else><i class="fa fa-circle-xmark" :title="$t('users.no-admin')"></i></span>
          </td>
          <td class="text-center">
            <a href="#" :title="$t('users.button-show-user')" @click.prevent="viewedUserId=user.Identifier"><i class="fas fa-eye me-2"></i></a>
            <a href="#" :title="$t('users.button-edit-user')" @click.prevent="editUserId=user.Identifier"><i class="fas fa-cog me-2"></i></a>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
  <hr>
  <div class="mt-3">
    <div class="row">
      <div class="col-6">
        <ul class="pagination pagination-sm">
          <li :class="{disabled:users.pageOffset===0}" class="page-item">
            <a class="page-link" @click="users.previousPage">&laquo;</a>
          </li>

          <li v-for="page in users.pages" :key="page" :class="{active:users.currentPage===page}" class="page-item">
            <a class="page-link" @click="users.gotoPage(page)">{{page}}</a>
          </li>

          <li :class="{disabled:!users.hasNextPage}" class="page-item">
            <a class="page-link" @click="users.nextPage">&raquo;</a>
          </li>
        </ul>
      </div>
      <div class="col-6">
        <div class="form-group row">
          <label class="col-sm-6 col-form-label text-end" for="paginationSelector">{{ $t('general.pagination.size') }}:</label>
          <div class="col-sm-6">
            <select id="paginationSelector" v-model.number="users.pageSize" class="form-select" @click="users.afterPageSizeChange()">
              <option value="10">10</option>
              <option value="25">25</option>
              <option value="50">50</option>
              <option value="100">100</option>
              <option value="999999999">{{ $t('general.pagination.all') }}</option>
            </select>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

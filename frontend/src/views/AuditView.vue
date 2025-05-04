<script setup>
import { onMounted } from "vue";
import {auditStore} from "@/stores/audit";

const audit = auditStore()

onMounted(async () => {
  await audit.LoadEntries()
})

</script>

<template>
  <div class="page-header">
    <h1>{{ $t('audit.headline') }}</h1>
  </div>

  <p class="lead">{{ $t('audit.abstract') }}</p>

  <!-- Entry list -->
  <div class="mt-4 row">
    <div class="col-12 col-lg-6">
      <h3>{{ $t('audit.entries-headline') }}</h3>
    </div>
    <div class="col-12 col-lg-6 text-lg-end">
      <div class="form-group d-inline">
        <div class="input-group mb-3">
          <input v-model="audit.filter" class="form-control" :placeholder="$t('general.search.placeholder')" type="text" @keyup="audit.afterPageSizeChange">
          <button class="input-group-text btn btn-primary" :title="$t('general.search.button')"><i class="fa-solid fa-search"></i></button>
        </div>
      </div>
    </div>
  </div>
  <div class="mt-2 table-responsive">
    <div v-if="audit.Count===0">
      <h4>{{ $t('audit.no-entries.headline') }}</h4>
      <p>{{ $t('audit.no-entries.abstract') }}</p>
    </div>
    <table v-if="audit.Count!==0" id="auditTable" class="table table-sm">
      <thead>
      <tr>
        <th scope="col">{{ $t('audit.table-heading.id') }}</th>
        <th class="text-center" scope="col">{{ $t('audit.table-heading.time') }}</th>
        <th class="text-center" scope="col">{{ $t('audit.table-heading.severity') }}</th>
        <th scope="col">{{ $t('audit.table-heading.user') }}</th>
        <th scope="col">{{ $t('audit.table-heading.origin') }}</th>
        <th scope="col">{{ $t('audit.table-heading.message') }}</th>
      </tr>
      </thead>
      <tbody>
      <tr v-for="entry in audit.FilteredAndPaged" :key="entry.Id">
        <td>{{entry.Id}}</td>
        <td>{{entry.Timestamp}}</td>
        <td class="text-center"><span class="badge rounded-pill" :class="[ entry.Severity === 'low' ? 'bg-light' : entry.Severity === 'medium' ? 'bg-warning' : 'bg-danger']">{{entry.Severity}}</span></td>
        <td>{{entry.ContextUser}}</td>
        <td>{{entry.Origin}}</td>
        <td>{{entry.Message}}</td>
      </tr>
      </tbody>
    </table>
  </div>
  <hr>
  <div class="mt-3">
    <div class="row">
      <div class="col-6">
        <ul class="pagination pagination-sm">
          <li :class="{disabled:audit.pageOffset===0}" class="page-item">
            <a class="page-link" @click="audit.previousPage">&laquo;</a>
          </li>

          <li v-for="page in audit.pages" :key="page" :class="{active:audit.currentPage===page}" class="page-item">
            <a class="page-link" @click="audit.gotoPage(page)">{{page}}</a>
          </li>

          <li :class="{disabled:!audit.hasNextPage}" class="page-item">
            <a class="page-link" @click="audit.nextPage">&raquo;</a>
          </li>
        </ul>
      </div>
      <div class="col-6">
        <div class="form-group row">
          <label class="col-sm-6 col-form-label text-end" for="paginationSelector">{{ $t('general.pagination.size') }}:</label>
          <div class="col-sm-6">
            <select id="paginationSelector" v-model.number="audit.pageSize" class="form-select" @click="audit.afterPageSizeChange()">
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

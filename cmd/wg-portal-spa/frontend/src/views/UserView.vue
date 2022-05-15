<script setup>
import Confirmation from "../components/Confirmation.vue";
</script>

<template>
  <Confirmation></Confirmation>
  <!-- Headline and interface selector -->
  <div class="page-header row">
    <div class="col-12">
      <h1>{{ $t('user.h1') }}</h1>
    </div>
  </div>

  <!-- User list -->
  <div class="mt-4 row">
    <div class="col-12 col-lg-8">
    </div>
    <div class="col-12 col-lg-4 text-lg-end">
      <a class="btn btn-primary" href="#" title="Send mail to selected users"><i class="fa fa-paper-plane"></i></a>
      <a class="btn btn-primary ms-2" href="#" title="Add multiple users"><i class="fa fa-plus me-1"></i><i
          class="fa fa-users"></i></a>
      <a class="btn btn-primary ms-2" href="#" title="Add a user"><i class="fa fa-plus me-1"></i><i
          class="fa fa-user"></i></a>
    </div>
  </div>
  <div class="mt-2 table-responsive">
    <table class="table table-sm" id="userTable">
      <thead>
        <tr>
          <th scope="col">
            <input class="form-check-input" type="checkbox" value="" id="flexCheckDefault" title="Select all">
          </th><!-- select -->
          <th scope="col">{{ $t('login.id') }}</th>
          <th scope="col">{{ $t('login.email') }}</th>
          <th scope="col">{{ $t('login.firstname') }}</th>
          <th scope="col">{{ $t('login.lastname') }}</th>
          <th scope="col" class="text-center">{{ $t('login.source') }}</th>
          <th scope="col" class="text-center">{{ $t('login.peers') }}</th>
          <th scope="col" class="text-center">{{ $t('login.admin') }}</th>
          <th scope="col"></th><!-- Actions -->
        </tr>
      </thead>
      <tbody>
        <tr>
          <th scope="row">
            <input class="form-check-input" type="checkbox" value="" id="flexCheckDefault">
          </th>
          <td>The name</td>
          <td>tester@test.de</td>
          <td>Franz</td>
          <td>Tester</td>
          <td class="text-center"><span class="badge rounded-pill bg-light">LDAP</span></td>
          <td class="text-center">15</td>
          <td class="text-center"><span class="text-danger"><i class="fa fa-check-circle"></i></span></td>
          <td class="text-center">
            <a href="#" title="Show peers"><i class="fa-solid fa-magnifying-glass me-2"></i></a>
            <a href="#" title="Edit peer"><i class="fas fa-cog"></i></a>
          </td>
        </tr>
        <tr>
          <th scope="row">
            <input class="form-check-input" type="checkbox" value="" id="flexCheckDefault">
          </th>
          <td>some unique id</td>
          <td>Volodimirius.longemail@test.de</td>
          <td>Volodimirius</td>
          <td>Longnameinus</td>
          <td class="text-center"><span class="badge rounded-pill bg-light">LDAP</span></td>
          <td class="text-center">3</td>
          <td class="text-center"><span><i class="fa fa-circle-xmark"></i></span></td>
          <td class="text-center">
            <a href="#" title="Show peers"><i class="fa-solid fa-magnifying-glass me-2"></i></a>
            <a href="#" title="Edit peer"><i class="fas fa-cog"></i></a>
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
          <li class="page-item disabled">
            <a class="page-link" href="?page={|{intAdd $.Page -1}|}">&laquo;</a>
          </li>

          <li class="page-item active">
            <a class="page-link" href="?page={|{$i}|}">1</a>
          </li>
          <li class="page-item active">
            <a class="page-link" href="?page={|{$i}|}">2</a>
          </li>

          <li class="page-item disabled">
            <a class="page-link" href="?page={|{intAdd $.Page 1}|}">&raquo;</a>
          </li>
        </ul>
      </div>
      <div class="col-6">
        <div class="form-group row">
          <label for="paginationSelector" class="col-sm-6 col-form-label text-end">{{ $t('interfaces.pagination.size')
          }}:</label>
          <div class="col-sm-6">
            <select class="form-select" id="paginationSelector">
              <option value="configurator.id">25</option>
              <option value="configurator.id">50</option>
              <option value="configurator.id">100</option>
              <option value="configurator.id">{{ $t('interfaces.pagination.all') }}</option>
            </select>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

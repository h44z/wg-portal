<script setup>
</script>

<template>
  <div class="row">
    <div class="col-lg-3"></div>
    <div class="col-lg-6">
      <div class="card mt-5">
        <div class="card-header">{{ $t('login.please') }} <div class="float-end"><a href="/"
              title="Open Landing-Page"><i class="fas fa-times-circle"></i></a></div>
        </div>
        <div class="card-body">
          <form method="post">
            <fieldset>
              <input type="hidden" name="_csrf" value="TODO:CSRF">
              <div class="form-group">
                <label for="inputUsername" class="form-label">{{ $t('login.username') }}</label>
                <div class="input-group mb-3">
                  <span class="input-group-text"><span class="far fa-user p-2"></span></span>
                  <input type="text" name="username" class="form-control" id="inputUsername"
                    aria-describedby="usernameHelp" placeholder="Enter username">
                </div>
                <!--small id="usernameHelp" class="form-text text-muted">We'll never share your email with anyone else.</small-->
              </div>
              <div class="form-group">
                <label for="inputPassword" class="form-label">{{ $t('login.pass') }}</label>
                <div class="input-group mb-3">
                  <span class="input-group-text"><span class="fas fa-lock p-2"></span></span>
                  <input type="password" name="password" class="form-control" id="inputPassword" placeholder="Password">
                </div>
              </div>

              <div class="row mt-5 d-flex">
                <div class="col-lg-4 d-flex mb-2">
                  <button class="btn btn-primary flex-fill" type="submit">{{ $t('login.btn') }}</button>
                </div>
                <div class="col-lg-8 d-flex mb-2">
                  <!-- OpenIdConnect / OAUTH providers -->

                </div>
              </div>
            </fieldset>
          </form>


        </div>
      </div>
    </div>
    <div class="col-lg-4"></div>
  </div>
</template>

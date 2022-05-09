<script setup>
</script>

<template>
  <div class="page-header">
    <h1>WireGuard VPN Portal</h1>
  </div>

  <p class="lead">WireGuard® is an extremely simple yet fast and modern VPN that utilizes state-of-the-art cryptography. It aims to be faster, simpler, leaner, and more useful than IPsec, while avoiding the massive headache. It intends to be considerably more performant than OpenVPN. </p>
  <h3 class="mt-5">More Information</h3>
  <div class="row">
    <div class="col-lg-4">
      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">WireGuard Installation</div>
        <div class="card-body d-flex flex-column">
          <h4 class="card-title">Installation</h4>
          <p class="card-text">Installation instructions for client software can be found on the official WireGuard website.</p>
          <a href="https://www.wireguard.com/install/" title="WireGuard Installation" target="_blank" rel="noopener noreferrer" class="mt-auto btn btn-primary btn-sm">Open Instructions</a>
        </div>
      </div>
    </div>
    <div class="col-lg-4">
      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">About WireGuard</div>
        <div class="card-body d-flex flex-column">
          <h4 class="card-title">About</h4>
          <p class="card-text">WireGuard® is an extremely simple yet fast and modern VPN that utilizes state-of-the-art cryptography.</p>
          <a href="https://www.wireguard.com/" title="WireGuard" target="_blank" rel="noopener noreferrer" class="mt-auto btn btn-primary btn-sm">More details</a>
        </div>
      </div>
    </div>
    <div class="col-lg-4">
      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">About WireGuard Portal</div>
        <div class="card-body d-flex flex-column">
          <h4 class="card-title">WireGuard Portal</h4>
          <p class="card-text">WireGuard Portal is a simple, web based configuration portal for WireGuard.</p>
          <a href="https://github.com/h44z/wg-portal/" title="WireGuard Portal" target="_blank" rel="noopener noreferrer" class="mt-auto btn btn-primary btn-sm">More details</a>
        </div>
      </div>
    </div>
  </div>

  <div class="bg-light p-5">
    <h2 class="display-5">VPN Profiles</h2>
    <p class="lead">You can access and download your personal VPN configurations via your Userprofile.</p>
    <hr class="my-4">
    <p>To find all your configured profiles click on the button below.</p>
    <p class="lead">
      <a href="/user/profile" class="btn btn-primary btn-lg" title="User-Profile">Open My Profile</a>
    </p>
  </div>

  <div class="bg-light p-5 mt-4">
    <h2 class="display-5">Administration Area</h2>
    <p class="lead">In the administration area you can manage WireGuard peers and the server interface as well as users that are allowed to log in to the WireGuard Portal.</p>
    <hr class="my-4">
    <p>To find all your configured profiles click on the button below.</p>
    <p class="lead">
      <a href="/admin/" class="btn btn-primary btn-lg" title="WireGuard Administration">Open WireGuard Administration</a>
      <a href="/admin/users/" class="btn btn-primary btn-lg" title="User Administration">Open User Administration</a>
    </p>
  </div>
</template>

<script setup>
import Modal from "../components/Modal.vue";
import Confirmation from "../components/Confirmation.vue";
</script>

<template>
  <Modal title="Tet" :visible="true" :close-on-backdrop="true">
    <template #default>
      <p>Lorum ipsum</p>
    </template>
    <template #footer>
      <div class="flex-fill text-start">
        <button class="btn btn-danger" type="button">Delete</button>
      </div>
      <button type="button" class="btn btn-secondary">Close</button>
      <button type="button" class="btn btn-primary">Save changes</button>
    </template>
  </Modal>
  <Confirmation></Confirmation>

  <!-- Headline and interface selector -->
  <div class="page-header row">
    <div class="col-12 col-lg-8">
      <h1>Interface Administration</h1>
    </div>
    <div class="col-12 col-lg-4 text-end">
      <div class="form-group">

      </div>
      <div class="form-group">
        <div class="input-group mb-3">
          <button class="input-group-text btn btn-primary" title="Add new interface"><i class="fa-solid fa-plus-circle"></i></button>
          <select class="form-select">
            <option value="configurator.id">No Interface available</option>
          </select>
        </div>
      </div>
    </div>
  </div>

  <!-- No interfaces information -->
  <div class="row">
    <div class="col-lg-12">
      <div class="mt-5">
        <h4>No interfaces found...</h4>
        <p>Click the plus button above to create a new WireGuard interface.</p>
      </div>
    </div>
  </div>

  <!-- Interface overview -->
  <div class="row">
    <div class="col-lg-12">

      <div class="card border-secondary mb-4" style="min-height: 15rem;">
        <div class="card-header">
          <div class="row">
            <div class="col-12 col-lg-8">
              Interface status for <strong>$.Interface.Identifier</strong> (server mode)
            </div>
            <div class="col-12 col-lg-4 text-lg-end">
              <a class="btn-link" href="#" title="Show interface configuration"><i class="fas fa-eye"></i></a>
              <a class="ms-5 btn-link" href="#" title="Download interface configuration"><i class="fas fa-download"></i></a>
              <a class="ms-5 btn-link" href="#" title="Write interface configuration file"><i class="fas fa-save"></i></a>
              <a class="ms-5 btn-link" href="#" title="Edit interface settings"><i class="fas fa-cog"></i></a>
            </div>
          </div>
        </div>
        <div class="card-body d-flex flex-column">
          <div class="row">
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>Public Key:</td>
                  <td>{|{.Interface.PublicKey}|}</td>
                </tr>
                <tr>
                  <td>Public Endpoint:</td>
                  <td>{|{.Interface.PeerDefEndpoint}|}</td>
                </tr>
                <tr>
                  <td>Listening Port:</td>
                  <td>{|{.Interface.ListenPort}|}</td>
                </tr>
                <tr>
                  <td>Enabled Peers:</td>
                  <td>{|{len .InterfacePeers}|}</td>
                </tr>
                <tr>
                  <td>Total Peers:</td>
                  <td>{|{.TotalPeers}|}</td>
                </tr>
                </tbody>
              </table>
            </div>
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>IP Address:</td>
                  <td>{|{.Interface.AddressStr}|}</td>
                </tr>
                <tr>
                  <td>Default allowed IP's:</td>
                  <td>{|{.Interface.PeerDefAllowedIPsStr}|}</td>
                </tr>
                <tr>
                  <td>Default DNS servers:</td>
                  <td>{|{.Interface.PeerDefDnsStr}|}</td>
                </tr>
                <tr>
                  <td>Default MTU:</td>
                  <td>{|{.Interface.Mtu}|}</td>
                </tr>
                <tr>
                  <td>Default Keepalive Interval:</td>
                  <td>{|{.Interface.PeerDefPersistentKeepalive}|}</td>
                </tr>
                </tbody>
              </table>
            </div>

            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>Public Key:</td>
                  <td>{|{.Interface.PublicKey}|}</td>
                </tr>
                <tr>
                  <td>Enabled Endpoints:</td>
                  <td>{|{len .InterfacePeers}|}</td>
                </tr>
                <tr>
                  <td>Total Endpoints:</td>
                  <td>{|{.TotalPeers}|}</td>
                </tr>
                </tbody>
              </table>
            </div>
            <div class="col-sm-6">
              <table class="table table-sm table-borderless device-status-table">
                <tbody>
                <tr>
                  <td>IP Address:</td>
                  <td>{|{.Interface.AddressStr}|}</td>
                </tr>
                <tr>
                  <td>DNS servers:</td>
                  <td>{|{.Interface.DnsStr}|}</td>
                </tr>
                <tr>
                  <td>Default MTU:</td>
                  <td>{|{.Interface.Mtu}|}</td>
                </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Peer list -->
  <div class="mt-4 row">
    <div class="col-12 col-lg-8">
      <h2 class="mt-2">Current VPN Peers/Endpoints</h2>
    </div>
    <div class="col-12 col-lg-4 text-lg-end">
      <a class="btn btn-primary" href="#" title="Send mail to all peers"><i class="fa fa-paper-plane"></i></a>
      <a class="btn btn-primary ms-2" href="#" title="Add multiple peers"><i class="fa fa-plus me-1"></i><i class="fa fa-users"></i></a>
      <a class="btn btn-primary ms-2" href="#" title="Add a peer"><i class="fa fa-plus me-1"></i><i class="fa fa-user"></i></a>
    </div>
  </div>
  <div class="mt-2 table-responsive">
    <table class="table table-sm" id="userTable">
      <thead>
      <tr>
        <th scope="col">
          <input class="form-check-input" type="checkbox" value="" id="flexCheckDefault" title="Select all">
        </th><!-- select -->
        <th scope="col">Name</th>
        <th scope="col">Identifier</th>
        <th scope="col">User</th>
        <th scope="col">IP's</th>
        <th scope="col">Endpoint</th>
        <th scope="col">Handshake</th>
        <th scope="col"></th><!-- Actions -->
      </tr>
      </thead>
      <tbody>
        <tr>
          <th scope="row">
            <input class="form-check-input" type="checkbox" value="" id="flexCheckDefault">
          </th>
          <td>The name</td>
          <td>The identifier </td>
          <td>user (email or id if no email)</td>
          <td>
            <span class="badge rounded-pill bg-light">127.0.0.1</span>
            <span class="badge rounded-pill bg-light">::1</span>
          </td>
          <td>Endpoint.IP</td>
          <td>NEver</td>
          <td class="text-center">
            <a href="#" title="Show peer"><i class="fas fa-eye me-2"></i></a>
            <a href="#" title="Edit peer"><i class="fas fa-cog"></i></a>
          </td>
        </tr>
        <tr>
          <th scope="row">
            <input class="form-check-input" type="checkbox" value="" id="flexCheckDefault">
          </th>
          <td>The name2</td>
          <td>The identifier2 </td>
          <td>user2 (email or id if no email)</td>
          <td>
            <span class="badge rounded-pill bg-light">127.0.0.1</span>
            <span class="badge rounded-pill bg-light">::1</span>
          </td>
          <td>Endpoint.IP</td>
          <td>NEver</td>
          <td class="text-center">
            <a href="#" title="Show peer"><i class="fas fa-eye me-2"></i></a>
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
          <label for="paginationSelector" class="col-sm-6 col-form-label text-end">Pagination size:</label>
          <div class="col-sm-6">
            <select class="form-select" id="paginationSelector">
              <option value="configurator.id">25</option>
              <option value="configurator.id">50</option>
              <option value="configurator.id">100</option>
              <option value="configurator.id">All (slow)</option>
            </select>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

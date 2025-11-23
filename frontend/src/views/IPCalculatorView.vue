<script setup>

import {ref, watch, computed} from "vue";
import isCidr from "is-cidr";
import {isIP} from "is-ip";
import {excludeCidr} from "cidr-tools";
import {useI18n} from 'vue-i18n';

const allowedIp = ref("")
const dissallowedIp = ref("")
const privateIP = ref("10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16")

const {t} = useI18n()

const errorAllowed = ref("")
const errorDissallowed = ref("")

/**
 * Validate a comma-separated list of IP and/or CIDR addresses.
 * @function validateIpAndCidrList
 * @param {string} value - Comma-separated string (e.g. "10.0.0.0/8, 192.168.0.1")
 * @returns {true|string} Returns true if all values are valid, otherwise an error message.
 */
function validateIpAndCidrList(value) {
  const list = value.split(",").map(v => v.trim()).filter(Boolean);
  if (list.length === 0) { 
    return t('calculator.allowed-ip.empty');
  }
  
  for (const addr of list) {
    if (!isIP(addr) && !isCidr(addr)) {
      return t('calculator.dissallowed-ip.invalid', {addr});
    }
  }
  return true;
}

/**
 * Watcher that validates allowed IPs input in real-time.
 * Updates `errorAllowed` whenever `allowedIp` changes.
 */
watch(allowedIp, (newValue) => {
  const result = validateIpAndCidrList(newValue);
  errorAllowed.value = result === true ? "" : result;
});

/**
 * Watcher that validates disallowed IPs input in real-time.
 * Updates `errorDissallowed` whenever `dissallowedIp` changes.
 */
watch(dissallowedIp, (newValue) => {
  if (!allowedIp.value || allowedIp.value.trim() === "") {
    allowedIp.value = "0.0.0.0/0";
  }
  const result = validateIpAndCidrList(newValue);
  errorDissallowed.value = result === true ? "" : result;
});

/**
 * Dynamically computes the resulting "Allowed IPs" list
 * by excluding the disallowed ranges from the allowed ranges.
 * @constant
 * @type {ComputedRef<string>}
 * @returns {string} A comma-separated string of resulting CIDR blocks.
 */
const newAllowedIp = computed(() => {
  if (errorAllowed.value || errorDissallowed.value) return "";

  try {
    const allowedList = allowedIp.value.split(",").map(v => v.trim()).filter(Boolean);
    const disallowedList = dissallowedIp.value.split(",").map(v => v.trim()).filter(Boolean);

    const result = excludeCidr(allowedList, disallowedList);

    return result.join(", ");
  } catch (e) {
    console.error("Allowed IPs calculation error:", e);
    return "";
  }
});

/**
 * Append private IP ranges to disallowed IPs.
 * If any already exist, they are preserved and new ones are appended only if not present.
 * @function addPrivateIPs
 */
function addPrivateIPs() {
  const privateList = privateIP.value.split(",").map(v => v.trim());
  const currentList = dissallowedIp.value.split(",").map(v => v.trim()).filter(Boolean);

  const combined = Array.from(new Set([...currentList, ...privateList]));
  dissallowedIp.value = combined.join(", ");
}

</script>

<template>
  <div class="page-header">
    <h1>{{ $t('calculator.headline') }}</h1>
  </div>

  <p class="lead">{{ $t('calculator.abstract') }}</p>

  <div class="mt-4 row">
    <div class="col-12 col-lg-5">
      <fieldset>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('calculator.allowed-ip.label') }}</label>
          <input class="form-control" v-model="allowedIp" :placeholder="$t('calculator.allowed-ip.placeholder')" :class="{ 'is-invalid': errorAllowed }">
          <div v-if="errorAllowed" class="text-danger mt-1">{{ errorAllowed }}</div>
        </div>
        <div class="form-group">
          <label class="form-label mt-4">{{ $t('calculator.dissallowed-ip.label') }}</label>
          <input class="form-control" v-model="dissallowedIp" :placeholder="$t('calculator.dissallowed-ip.placeholder')" :class="{ 'is-invalid': errorDissallowed }">
          <div v-if="errorDissallowed" class="text-danger mt-1">{{ errorDissallowed }}</div>
        </div>
      </fieldset>
      <fieldset>
        <hr class="mt-4">
        <button class="btn btn-primary mb-4" type="button" @click="addPrivateIPs">{{ $t('calculator.button-exclude-private') }}</button>
      </fieldset>
    </div>
    <div class="col-12 col-lg-2 mt-sm-4">
    </div>
    <div class="col-12 col-lg-5">
      <h1>{{ $t('calculator.headline-allowed-ip') }}</h1>
      <fieldset>
        <div class="form-group">
          <textarea class="form-control" :value="newAllowedIp" rows="6" :placeholder="$t('calculator.new-allowed-ip.placeholder')" readonly></textarea>
        </div>
      </fieldset>
    </div>
  </div>

</template>

<style scoped>

</style>
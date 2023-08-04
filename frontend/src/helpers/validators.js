import isCidr from "is-cidr";
import {isIP} from 'is-ip';

export function validateCIDR(value) {
  return isCidr(value) !== 0
}

export function validateIP(value) {
  return isIP(value)
}

export function validateDomain(value) {
  return true
}
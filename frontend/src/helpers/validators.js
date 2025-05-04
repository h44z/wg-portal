import isCidr from "is-cidr";
import {isIP} from 'is-ip';

export function validateCIDR() {
  return [{
    classes: 'invalid-cidr',
    rule: ({ text }) => isCidr(text) === 0,
    disableAdd: true,
  }]
}

export function validateIP() {
  return [{
    classes: 'invalid-ip',
    rule: ({ text }) => !isIP(text),
    disableAdd: true,
  }]
}

export function validateDomain() {
  return [{
    classes: 'invalid-domain',
    rule: tag => tag.text.length < 3,
    disableAdd: true,
  }]
}
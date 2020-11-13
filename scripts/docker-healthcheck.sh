#!/bin/bash

set -e

goss -g /app/goss/wgportal/goss.yaml validate --format json_oneline

exit 0
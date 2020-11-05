#!/bin/bash

set -e

goss -g /app/goss/pbserv/goss.yaml validate --format json_oneline

exit 0
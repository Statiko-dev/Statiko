#!/bin/sh

set -e

# Perform database migration
/usr/local/bin/smplatform migrate

#!/bin/bash
#
# Copyright 2020 Red Hat Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.
set -ex

# This script generates the nova.conf file and copies the result to the
# ephemeral /var/lib/config-data/merged volume.
#
# Secrets are obtained from ENV variables.
export DB=${DatabaseName:-"manila"}
export DBHOST=${DatabaseHost:?"Please specify a DatabaseHost variable."}
export DBUSER=${DatabaseUser:-"manila"}
export DBPASSWORD=${DatabasePassword:?"Please specify a DatabasePassword variable."}
export MANILAPASSWORD=${ManilaPassword:?"Please specify a ManilaPassword variable."}
export TRANSPORTURL=${TransportURL:-""}

export CUSTOMCONF=${CustomConf:-""}

SVC_CFG=/etc/manila/manila.conf
SVC_CFG_MERGED=/var/lib/config-data/merged/manila.conf

# expect that the common.sh is in the same dir as the calling script
SCRIPTPATH="$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
. ${SCRIPTPATH}/common.sh --source-only

# Copy default service config from container image as base
cp -a ${SVC_CFG} ${SVC_CFG_MERGED}

# Merge all templates from config-data defaults first, then custom
# NOTE: custom.conf files (for both the umbrella Manila CR in config-data/defaults
#       and each custom.conf for each sub-service in config-data/custom) still need
#       to be handled separately below because the "merge_config_dir" function will
#       not merge custom.conf into manila.conf (because the files obviously have
#       different names)
for dir in /var/lib/config-data/default /var/lib/config-data/custom; do
    merge_config_dir ${dir}
done

echo merging /var/lib/config-data/default/custom.conf into ${SVC_CFG_MERGED}
crudini --merge ${SVC_CFG_MERGED} < /var/lib/config-data/default/custom.conf

if [ -n "$CUSTOMCONF" ]; then
    echo merging /var/lib/config-data/custom/${CUSTOMCONF} into ${SVC_CFG_MERGED}
    crudini --merge ${SVC_CFG_MERGED} < /var/lib/config-data/custom/${CUSTOMCONF}
fi

# set secrets
if [ -n "$TRANSPORTURL" ]; then
    crudini --set ${SVC_CFG_MERGED} DEFAULT transport_url $TRANSPORTURL
fi
crudini --set ${SVC_CFG_MERGED} database connection mysql+pymysql://${DBUSER}:${DBPASSWORD}@${DBHOST}/${DB}
crudini --set ${SVC_CFG_MERGED} keystone_authtoken password $MANILAPASSWORD

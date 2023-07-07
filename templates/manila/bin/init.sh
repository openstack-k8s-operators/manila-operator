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
export PASSWORD=${ManilaPassword:?"Please specify a ManilaPassword variable."}
export TRANSPORTURL=${TransportURL:-""}
export LOGGINGCONF=${LoggingConf:-"false"}

export CUSTOMCONF=${CustomConf:-""}

DEFAULT_DIR=/var/lib/config-data/default
CUSTOM_DIR=/var/lib/config-data/custom
MERGED_DIR=/var/lib/config-data/merged
SVC_CFG=/etc/manila/manila.conf
SVC_CFG_MERGED=/var/lib/config-data/merged/manila.conf
SVC_CFG_MERGED_DIR=${MERGED_DIR}/manila.conf.d
SVC_CFG_LOGGING=/etc/manila/logging.conf

mkdir -p ${SVC_CFG_MERGED_DIR}

cp ${DEFAULT_DIR}/* ${MERGED_DIR}

# Save the default service config from container image as manila.conf.sample,
# and create a small manila.conf file that directs people to files in
# manila.conf.d.
cp -a ${SVC_CFG} ${SVC_CFG_MERGED}.sample
cat <<EOF > ${SVC_CFG_MERGED}
# Service configuration snippets are stored in the manila.conf.d subdirectory.
EOF

cp ${DEFAULT_DIR}/manila.conf ${SVC_CFG_MERGED_DIR}/00-default.conf

# Generate 01-deployment-secrets.conf
DEPLOYMENT_SECRETS=${SVC_CFG_MERGED_DIR}/01-deployment-secrets.conf
if [ -n "$TRANSPORTURL" ]; then
    cat <<EOF > ${DEPLOYMENT_SECRETS}
[DEFAULT]
transport_url = ${TRANSPORTURL}

EOF
fi

# TODO: service token
cat <<EOF >> ${DEPLOYMENT_SECRETS}
[database]
connection = mysql+pymysql://${DBUSER}:${DBPASSWORD}@${DBHOST}/${DB}

[keystone_authtoken]
password = ${PASSWORD}

[nova]
password = ${PASSWORD}

[service_user]
password = ${PASSWORD}
EOF

if [ -f ${DEFAULT_DIR}/custom.conf ]; then
    cp ${DEFAULT_DIR}/custom.conf ${SVC_CFG_MERGED_DIR}/02-global.conf
fi

if [ -f ${CUSTOM_DIR}/custom.conf ]; then
    cp ${CUSTOM_DIR}/custom.conf ${SVC_CFG_MERGED_DIR}/03-service.conf
fi

if [ "$LOGGINGCONF" == "true" ]; then
cat <<EOF >> ${SVC_CFG_MERGED_DIR}/03-service.conf

[DEFAULT]
log_config_append=${SVC_CFG_LOGGING}
EOF
fi

SECRET_FILES="$(ls /var/lib/config-data/secret-*/* 2>/dev/null || true)"
if [ -n "${SECRET_FILES}" ]; then
    cat ${SECRET_FILES} > ${SVC_CFG_MERGED_DIR}/04-secrets.conf
fi

# Probes cannot run kolla_set_configs because it uses the 'manila' uid
# and gid and doesn't have permission to make files be owned by root.
# This means the probe must use files in the "merged" location, and the
# files must be readable by 'manila'.
chown -R :manila ${SVC_CFG_MERGED_DIR}

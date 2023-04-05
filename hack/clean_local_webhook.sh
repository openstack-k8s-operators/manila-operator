#!/bin/bash
set -ex

oc delete validatingwebhookconfiguration vmanila.kb.io --ignore-not-found
oc delete mutatingwebhookconfiguration mmanila.kb.io --ignore-not-found

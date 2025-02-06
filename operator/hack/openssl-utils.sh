#!/usr/bin/env bash
# /*
# Copyright 2024 The Grove Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# */

set -o errexit
set -o nounset
set -o pipefail


TLS_ARTIFACT_REQUESTS_DIR=""
TLS_DIR=""

function pki::check_prerequisites() {
  if ! command -v openssl &> /dev/null; then
    echo "openssl is not installed. Please install openssl to continue."
    exit 1
  fi
}

function pki::generate_ca_key_cert() {
  echo "Generating CA key..."
  # generate the CA key
  openssl genrsa -out ${TLS_DIR}/ca.key 4096
  # create the configuration to generate the CA cert
  pki::create_ca_config
  echo "Generating CA certificate..."
  # generate the CA cert
  openssl req \
    -new \
    -x509 \
    -config ${TLS_ARTIFACT_REQUESTS_DIR}/ca.cnf \
    -key ${TLS_DIR}/ca.key \
    -out ${TLS_DIR}/ca.crt \
    -days 365 \
    -batch
}

function pki::create_ca_config() {
  echo "Creating CA configuration at ${TLS_ARTIFACT_REQUESTS_DIR}/ca.cnf..."
  cat >${TLS_ARTIFACT_REQUESTS_DIR}/ca.cnf <<EOF
  [ ca ]
  default_ca = CA_default

  [ CA_default ]
  default_days = 365
  default_md = sha512
  unique_subject = no
  copy_extensions = copy

  # used to create the CA certificate
  [ req ]
  prompt = no
  distinguished_name = dn
  x509_extensions = extensions

  [ dn ]
  CN = Grove CA
  O = Grove

  # https://docs.openssl.org/3.4/man5/x509v3_config/#standard-extensions
  [ extensions ]
  keyUsage = critical,digitalSignature,keyEncipherment,keyCertSign,cRLSign
  basicConstraints = critical,CA:true,pathlen:1
  subjectKeyIdentifier=hash
EOF
}

function pki::generate_server_key_cert() {
  if [[ $# -ne 1 ]]; then
    echo -e "${FUNCNAME[0]} expects a namespace"
    exit 1
  fi
  local namespace="$1"
  echo "Generating server key..."
  openssl genrsa -out ${TLS_DIR}/server.key 4096
  # create the configuration to generate the server cert
  pki::create_server_config ${namespace}
  echo "Generating server CSR..."
  openssl req \
    -new \
    -config ${TLS_ARTIFACT_REQUESTS_DIR}/server.cnf \
    -key ${TLS_DIR}/server.key \
    -out ${TLS_DIR}/server.csr \
    -batch
  echo "Generating server certificate..."
  openssl x509 \
    -req \
    -in ${TLS_DIR}/server.csr \
    -CA ${TLS_DIR}/ca.crt \
    -CAkey ${TLS_DIR}/ca.key \
    -CAcreateserial \
    -out ${TLS_DIR}/server.crt \
    -days 365 \
    -extfile ${TLS_ARTIFACT_REQUESTS_DIR}/server.cnf \
    -extensions extensions
}

function pki::create_server_config() {
  if [[ $# -ne 1 ]]; then
    echo -e "${FUNCNAME[0]} expects a namespace"
    exit 1
  fi
  local namespace="$1"

  echo "Creating server configuration at ${TLS_ARTIFACT_REQUESTS_DIR}/server.cnf..."
  cat >${TLS_ARTIFACT_REQUESTS_DIR}/server.cnf <<EOF
  [ req ]
  prompt = no
  distinguished_name = dn
  req_extensions = extensions

  [ dn ]
  CN = grove-operator
  O = Grove

  [ extensions ]
  keyUsage = critical,digitalSignature,keyEncipherment
  extendedKeyUsage = serverAuth
  basicConstraints = critical,CA:FALSE
  subjectAltName = @sans

  [ sans ]
  DNS.0 = grove-operator
  DNS.1 = grove-operator.${namespace}
  DNS.2 = grove-operator.${namespace}.svc
  DNS.3 = grove-operator.${namespace}.svc.cluster.local
EOF
}

function pki::generate_resources() {
  pki::check_prerequisites
  if [[ $# -ne 2 ]]; then
    echo -e "${FUNCNAME[0]} requires expects a target directory and namespace"
    exit 1
  fi
  TLS_DIR="$1"
  TLS_ARTIFACT_REQUESTS_DIR="${TLS_DIR}/requests"
  mkdir -p ${TLS_ARTIFACT_REQUESTS_DIR}
  local namespace="$2"

  pki::generate_ca_key_cert
  pki::generate_server_key_cert ${namespace}
}

# is_certificate_expired checks if the certificate has expired
function pki::is_certificate_expired() {
  if [[ $# -ne 1 ]]; then
    echo -e "${FUNCNAME[0]} expects a certificate file"
    exit 1
  fi
  local cert_file="$1"
  local expiration_date=$(openssl x509 -enddate -noout -in ${cert_file} | cut -d= -f2)
  local expiration_epoch=$(date -d "${expiration_date}" +%s)
  local current_epoch=$(date +%s)
  if [[ ${current_epoch} -gt ${expiration_epoch} ]]; then
    return 0
  fi
  return 1
}
#!/usr/bin/env bash

source cluster-sync/install.sh

function _kubectl(){
  kubectl "$@"
}

function verify() {
  echo "Verify not needed for external provider"
}


function up() {
  echo "using external provider"
}

function configure_storage() {
  echo "Local storage not needed for external provider..."
}



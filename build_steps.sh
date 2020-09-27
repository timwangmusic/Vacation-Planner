#!/bin/bash

set -eoux pipefail

# Pass the Build Hash as a Env
KEY_PATH="/home/pi/keys/gittoken.json"
BUILD_URL="https://dafae76faf4f.ngrok.io/job/UnW_Can1/$BUILD_NUMBER/console"

function check_dir() {
	echo $PWD
	ls -al
}

function build_docker() {
	docker build -t unwindenv:$BUILD_NUMBER .
	docker images | unwindenv
}

function run_docker() {
	docker run --rm unwindenv:$BUILD_NUMBER
}

function extract_token() {
	token=$(jq '.token' $KEY_PATH)
    token="${token%\"}"
    token="${token#\"}"
	echo $token
}


$1


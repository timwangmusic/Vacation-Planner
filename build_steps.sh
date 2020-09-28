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
	docker images
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

function post_gitStatus_success() {
	echo "Post status after build succeeds"
	git_token=$(extract_token)
	URL="https://api.GitHub.com/repos/sananand007/JBot/statuses/$GIT_COMMIT?access_token=$git_token"
	curl "$URL" \
  		-H "Content-Type: application/json" \
  		-X POST \
  		-d "{
            \"state\": \"success\",
            \"context\": \"continuous-integration/jenkins\",
            \"description\": \"Jenkins\",
            \"target_url\": $BUILD_URL
        }" >> output.txt

}

function push_docker() {
	# Push Docker image to Dockerhub
	echo "Push Step"
}


$1


#!/bin/bash
set -eoux pipefail

# Pass the Build Hash as a Env
KEY_PATH="/home/pi/keys/gittoken.json"
BUILD_URL="https://6aade2b09d3b.ngrok.io/job/UnW_Can1/$BUILD_NUMBER/console"

function check_dir() {
	echo $PWD
	ls -al
}

function build_docker() {
	docker build -t unwindtest:$BUILD_NUMBER .
	docker images
}

function run_docker() {
	docker run --rm unwindtest:$BUILD_NUMBER /bin/sh -c "echo $PWD && \
		go test -v ./..."
}

function extract_token() {
	token=$(jq '.token' $KEY_PATH)
    token="${token%\"}"
    token="${token#\"}"
	echo $token
}

function post_gitStatus_success() {
	# Add the git status
	echo "Post status after build succeeds"
	git_token=$(extract_token)
	URL="https://api.GitHub.com/repos/<>/<>/statuses/$GIT_COMMIT?access_token=$git_token"
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

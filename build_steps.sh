#!/bin/bash
set -eoux pipefail

# Pass the Build Hash as a Env
KEY_PATH="/home/pi/keys/gittoken.json"
PR_BUILDER="Unwind_Build_PullRequest"
PR_BUILD_URL="https://f8e9056a4af3.ngrok.io/job/$PR_BUILDER/$BUILD_NUMBER/console"

function check_dir() {
	echo $PWD
	ls -al
}

function build_docker() {
	docker build -t unwindtest:$BUILD_NUMBER .
	docker images
}

function run_docker() {
	docker run --rm unwindtest:$BUILD_NUMBER /bin/sh -c "ls -al && echo $PWD && \
		go test -v ./..."
}

function run_server() {
	#TODO : this is not active, will look into this in the next PR to run this on the server
	docker run --rm -p 3000:3000 unwindtest:$BUILD_NUMBER 
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
            \"target_url\": $PR_BUILD_URL
        }" >> output.txt

}

function push_docker() {
	# Push Docker image to Dockerhub
	echo "Push Step"
}

$1

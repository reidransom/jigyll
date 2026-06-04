#!/bin/bash -e

docker buildx build --platform linux/arm64,linux/amd64 . \
	-f Dockerfile \
	-t reidransom/jigyll:latest \
	--push

goreleaser release
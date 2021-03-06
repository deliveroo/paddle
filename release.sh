#!/usr/bin/env bash

set -e
set -x

die() { echo "$*" 1>&2 ; exit 1; }

which goreleaser || die "Install goreleaser from http://goreleaser.com"
VERSION=`cat VERSION | tr -d '\n'`
git diff-index --quiet --cached HEAD -- || die "Index dirty, commit first"
go generate ./cli
git add VERSION
git add cli/version.go
git commit -m "Version $VERSION" || echo "Version not changed"
git tag -a v$VERSION -m "Version $VERSION"
git push origin
git push origin v$VERSION
goreleaser --rm-dist

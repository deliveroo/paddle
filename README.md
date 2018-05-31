# Paddle

Paddle is a command line tool for canoe data archival and processing.

## Setup for local development

Make sure you have Go installed on your machine and that you checkout the repo to
the right folder. By default should be:

```
mkdir -p ~/go/src/github.com/deliveroo
cd ~/go/src/github.com/deliveroo
git clone git@github.com:deliveroo/paddle.git
cd paddle
```

Install dependencies:

```
brew install glide
glide i
```

You will need create a `$HOME/.paddle.yaml` that contains the bucket name, e.g:

```
> cat $HOME/.paddle.yaml
bucket: roo-bucket
```

or if you prefer specify `BUCKET` as an environment variable

You will also need to create a `$HOME/.aws/config` or `$HOME/.aws/credentials` so Paddle can connect to AWS, e.g.:

```
> cat $HOME/.aws/credentials
[default]
aws_access_key_id=xxx
aws_secret_access_key=yyy
region=eu-west-1
```

```
$ go build
```

## Testing

```
$ go test ./...
```

## Release

In order to release a new version, set up github export GITHUB_TOKEN=[YOUR_TOKEN]. Make sure that you have goreleaser installed from [goreleaser.com](http://goreleaser.com).

Ensure your git repo is clean. Then update VERSION (no need to commit it, it will be committed automatically), and run:

```
$ ./release.sh
```

To also release a new version of the `paddlecontainer` you have to do the following:

```
$ git clone git@github.com:deliveroo/roo-kube.git
$ cd roo-kube/paddlecontainer
```

Then follow the instructions from here: https://github.com/deliveroo/roo-kube/blob/master/docker-publish-image-README.md#buildingpushing-docker-images

## Usage

```
$ paddle help
```

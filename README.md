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

## Running Unit Tests

```
$ go test ./...
```

## Testing on Staging
There is no dedicated test environment in staging for Paddle, but it is possible to test in staging using another Canoe pipeline that uses Paddle.

1. Download and setup a pipeline repo that can be tested in staging, for example [`rider-planning-pipeline`](https://github.com/deliveroo/rider-planning-pipeline). Make sure you can build at least one of the pipeline steps listed in the definition `yml` file. The step should also contain inputs from a previous step if the paddle `get` command needs to be tested.
2. Modify the paddle template.go `podTemplate` to use a temporary paddlecontainer tag. e.g `localtest001`. The `latest` tag cannot be used as it is pulled into production images.
    ```
    ...
    image: "219541440308.dkr.ecr.eu-west-1.amazonaws.com/paddlecontainer:localtest001"
    ...
    ```
3. Copy the paddle build that needs to be tested to root of pipeline repo. Build using:
    ```
    go build
    ```
4. Copy a linux+amd64 paddle build that needs to be tested to the root of a new directory. Build for linux+amd64 using:
    ```
    GOOS=linux GOARCH=amd64 go build
    ```
5. Create a new Dockerfile in the same directory with the following content:
    ```
    FROM ubuntu

    RUN apt-get update
    RUN apt-get install -y ca-certificates curl wget

    COPY ./paddle /usr/bin/paddle
    ```
6. From a shell, build and push a Docker image using the Dockerfile. Ensure local AWS credentials are setup to enable login.

    **The image _must_ be built using a tag other than `latest` otherwise the image used in production will be overwritten!** In the example below, `localtest001` has been used to match the change made above.
    ```
    $(aws ecr get-login --profile k8s_production --no-include-email --region eu-west-1)

    docker build -t 219541440308.dkr.ecr.eu-west-1.amazonaws.com/paddlecontainer:localtest001 .

    docker push 219541440308.dkr.ecr.eu-west-1.amazonaws.com/paddlecontainer:localtest001
    ```
7. Modify the testing step(s) in the pipeline `yml` file with the test paddle container tag e.g. `localtest001` and make any other amendements as necessary.
8. Re-run the pipeline to test the new version of paddle.
9. If successful, follow the process below to release a new version of paddle.

## Release

In order to release a new version, set up github export GITHUB_TOKEN=[YOUR_TOKEN]. Make sure that you have goreleaser installed from [goreleaser.com](http://goreleaser.com).

Ensure your git repo is clean, new PRs have already merged and your branch is set to master. Then update VERSION (no need to commit it, it will be committed automatically), and run:

```
$ ./release.sh
```

If the final `goreleaser` part of this process fails (e.g. due to a missing GITHUB_TOKEN or `goreleaser` install), the version will have likely been incremented and pushed to origin already. In this case, run the final `goreleaser` step manually:
```
goreleaser --rm-dist
```

## Usage

```
$ paddle help
```

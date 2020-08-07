# podcalypse

Podcalypse is a simple k8s tool that kills pods at a given constant rate, provided they match a label.

This is useful when you want to check that your system (e.g. your load balancer) is configured to tolerate
a given level of disruption (usually caused by rolling upgrades).

It's not meant to be a generic chaos-monkey tool, but rather a more focused stress-testing tool.

## Deploy

This project currently has no releases; you need to build it yourself. The easiest way is to use
the `ko` project and do:

```
KO_DOCKER_REPO=your/docker/registry/image/name ko apply --strict -f deploy.yaml
```

this will compile the Go code in this repo, push a docker image, splice that image name into the
deploy.yaml file and finally apply it to your current k8s cluster context.

## Usage

Podcalypse will periodically kill all running pods that have the followin label:

```
labels:
  mkm.pub/podcalipse: "true"
```

just apply this to the pods you want to test and run the podcalypse controller.

## Configure

The main knob to tune is the `PODCALYPSE_RATE` environment variable which controls how often (per second) will a pod be killed.


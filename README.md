# Check Prometheus from Nagios

<!-- vim-markdown-toc GFM -->

* [What](#what)
* [TL;DR](#tldr)
* [Why](#why)
* [How to build](#how-to-build)
* [How to run](#how-to-run)
* [Usage](#usage)
* [Debug](#debug)
* [Label](#label)
* [Method](#method)
* [Basic auth](#basic-auth)

<!-- vim-markdown-toc -->

## What

A Nagios plugin for querying Prometheus.

## TL;DR

```
$ docker run -it claranet/nagitheus:latest -h
```

## Why

This tool has been inspired by the upstream provided shell script to be found [here](https://github.com/prometheus/nagios_plugins). But unfortunately this shell script is deficient in several ways:

1. It actually works :)
2. No need to specify if vector or scalar
3. It doesn't stop at the first result but iterates over whole vector
4. Ability to print desired label
5. Go binary: no need of specific software on the nagios monitoring

## How to build

build:
```
$ go build nagitheus.go
or from mac to linux
env GOOS=linux GOARCH=amd64 go build nagitheus.go
````
run:
```
 $ ./nagitheus -H "https://prometheus.example.com" -q "PrometheusQueryNoSpaces" -w 2  -c 2 -u username -p PASSWORD -m le  -l label
```
## How to run
```
$ go run nagitheus.go -H 'https://prometheus.mgt.domain.com' -q "(kubelet_volume_stats_used_bytes/kubelet_volume_stats_capacity_bytes*100)>2" -w 2  -c 5  -m ge -u UN -p PW -l persistentvolumeclaim
WARNING prometheus-kube-prometheus-db-prometheus-kube-prometheus-0 is 2.2607424766047886 CRITICAL prometheus-kube-prometheus-db-prometheus-kube-prometheus-0 is 5.625835543270624
exit status 2
```
## Usage

```
  -H string
    	Host to query (Required, i.e. https://example.prometheus.com)
  -q string
    	Prometheus query (Required)
  -w string
    	Warning treshold (Required)
  -c string
    	Critical treshold (Required)
  -d string
    	Print whole prometheus result to output (Optional) (default "no")
  -l string
    	Label to print (Optional) (default "none")
  -m string
    	Comparison method (Optional) (default "ge")
  -u string
    	Username (Optional)
  -p string
    	Password (Optional)

```
This software will perform a request on the prometheus server. Required flags are the Host, Query, Warning and Critical.

## Debug

`-d yes` will print to outputn the whole response from Prometheus (best used from command line and not from Nagios):
```
Prometheus response: {
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {
          "endpoint": "http-metrics",
          "exported_namespace": "aux",
          "instance": "10.42.0.2:10255",
          "job": "kubelet",
          "namespace": "kube-system",
          "persistentvolumeclaim": "prometheus-kube-prometheus-db-prometheus-kube-prometheus-0",
          "service": "kubelet"
        },
        "value": [
          1521551995.114,
          "2.2607424766047886"
        ]
      },
      {
        "metric": {
          "endpoint": "http-metrics",
          "exported_namespace": "aux",
          "instance": "10.42.0.4:10255",
          "job": "kubelet",
          "namespace": "kube-system",
          "persistentvolumeclaim": "prometheus-kube-prometheus-db-prometheus-kube-prometheus-0",
          "service": "kubelet"
        },
        "value": [
          1521551995.114,
          "5.625835543270624"
        ]
      }
    ]
  }
}
```
## Label

`-l labelname` takes a label that you want to print toghether with Status and value:
```
WARNING prometheus-kube-prometheus-db-prometheus-kube-prometheus-0 is 2.2607424766047886 CRITICAL prometheus-kube-prometheus-db-prometheus-kube-prometheus-0 is 5.625835543270624
```
Without the label the result would be
```
WARNING is 2.2607424766047886 CRITICAL is 5.625835543270624
```

## Method

`-m ge OR gt OR le OR lt` tells the check how to compare the result with the critical and warning flags 

## Basic auth
`-u username -p password` when both are set the request will be performed with basic auth


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

This tool has been inspired by the upstream provided shell script to be found [here](https://github.com/prometheus/nagios_plugins). But unfortunately this shell script is deficient in several ways.
Nagitheus instead:

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
 $ ./nagitheus -H "https://prometheus.example.com" -q "Prometheus Query" -w 2  -c 2 -u username -p PASSWORD -m le  -l label
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
  -f string
      Format message with go template (Optional)
  -m string
      Comparison method (Optional) (default "ge")
  -u string
      Username (Optional)
  -p string
      Password (Optional)
  -t string
      Token (Optional)
  --critical-on-missing
      Return CRITICAL if query results are missing (Optional) (default "no")
  --max-chars
      Max. count of characters to print
  --print-details
      Prints all returned values on multiline result
  --value-mapping string
    	Mapping result metrics for output (Optional, json i.e. '{"0":"down","1":"up"}')
  --value-unit string
    	Unit of the value for output (Optional, i.e. '%')
  -v  Prints nagitheus version

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
## Output
If the result contains multiple values, the output returns a summary and multiline results.

## Label

`-l labelname` takes a label that you want to print toghether with Status and value:
```
CRITICAL 1 persistentvolumeclaim critical, 1 persistentvolumeclaim warning, 0 persistentvolumeclaim ok :
------
CRITICAL persistentvolumeclaim prometheus-kube-prometheus-db-prometheus-kube-prometheus-0 is 5.625835543270624
WARNING persistentvolumeclaim prometheus-kube-prometheus-db-prometheus-kube-prometheus-0 is 2.2607424766047886
```
Without the label the result would be:
```
CRITICAL 1 item critical, 1 item warning, 0 item ok :
------
WARNING value is 2.2607424766047886
CRITICAL value is 5.625835543270624
```

## Format
`-f formatstring` take a go template string to print status the way you want. Some variables are available for display.
* `Label`
* `Value`
* `Metric` Map of metrics returned from prometheus that you can access with `{{.Metric.<metric_name>}}`

Example:
```
-f '{{.Metric.node}}: {{.Metric.condition}} is {{.Value}}' --value-mapping '{"1":"KO","0":"OK"}'
```
Will output
```
WARNING 0 condition critical, 1 condition warning, 2 condition ok :
------
WARNING kube-node-1: FrequentContainerdRestart is KO
OK kube-node-1: CorruptDockerOverlay2 is OK
OK kube-node-2: CorruptDockerOverlay2 is OK
OK kube-node-2: FrequentContainerdRestart is OK
```

## Method

`-m ge OR gt OR le OR lt` tells the check how to compare the result with the critical and warning flags 

## Basic auth
`-u username -p password` when both are set the request will be performed with basic auth

## Token
`-t token` the request will be performed adding the token to headers

For example in Openshift you need to do
```
TOKEN=$(oc whoami -t)
-t $TOKEN
```

## Value Mapping
`-value-mapping <json key-value pairs>` allow mapping of the result values to defined strings for output, e.g. 0=>DOWN, 1=> UP:
```
./nagitheus -H 'https://prometheus.mgt.domain.com' -q 'up{job="prometheus"}' -c 1 -w 1 -m lt -l job -value-mapping '{"0":"DOWN","1":"UP"}'
```
returns
```
OK job prometheus is UP
```

## Value Unit
`-value-unit <unit>` allows adding a unit to the output result values, e.g. '%', 'GB':
```
WARNING persistentvolumeclaim prometheus-kube-prometheus-db-prometheus-kube-prometheus-0 is 2.2607424766047886 %
CRITICAL persistentvolumeclaim prometheus-kube-prometheus-db-prometheus-kube-prometheus-0 is 5.625835543270624 %
```

## Version
`-v` returns the version of nagitheus

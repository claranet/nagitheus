# Check Prometheus from Nagios

## How To

build:
```
$ go build nagitheus.go`
````
run:
```
 $ ./nagitheus -H "https://prometheus.aux.spryker.userwerk.gcp.cloud.de.clara.net" -q "((kubelet_volume_stats_used_bytes)/kubelet_volume_stats_capacity_bytes)*100>2" -w 2.7  -c 2.3 -u claradm -p PASSWORD -m le -d yes
  -H string
    	Host to query (Required, i.e. https://example.prometheus.com)
  -c string
    	Critical treshold (Required)
  -d string
    	Print prometheus result to output (Optional) (default "no")
  -l string
    	Label to print (Optional) (default "none")
  -m string
    	Comparison method (Optional) (default "ge")
  -p string
    	Password (Optional)
  -q string
    	Prometheus query (Required)
  -u string
    	Username (Optional)
  -w string
    	Warning treshold (Required)`
```



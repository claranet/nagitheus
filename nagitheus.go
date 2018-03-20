// HOW TO:
// go build nagitheus.go
// ./nagitheus -H "https://prometheus.aux.spryker.userwerk.gcp.cloud.de.clara.net" -q "((kubelet_volume_stats_used_bytes)/kubelet_volume_stats_capacity_bytes)*100>2" -w 2.7  -c 2.3 -u claradm -p PASSWORD -m le - d yes

package main

import (
    "flag"
    "fmt"
    "net/http"
    "crypto/tls"
    "io/ioutil"
    "bytes"
    "encoding/json"
    "os"
    "strconv"
    "time"
)

var exit_status int
const (
	OK  = 0
	WARNING = 1
	CRITICAL = 2
	UNKNOWN = 3
)

var NagiosMessage string
var NagiosStatus int

// this structure is what promethes gives back when queried.
// The Metric struct is not fixed, can vary according to what labels are returned
// here there are some lables that are often present
type PrometheusStruct struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func main() {
    host := flag.String("H", "", "Host to query (Required, i.e. https://example.prometheus.com)")
    query := flag.String("q", "", "Prometheus query (Required)")
    warning := flag.String("w", "", "Warning treshold (Required)")
    critical := flag.String("c", "", "Critical treshold (Required)")
    username := flag.String("u", "", "Username (Optional)")
    password := flag.String("p", "", "Password (Optional)")
    label := flag.String("l", "all", "Label to print (Optional)")
    method := flag.String("m", "ge", "Comparison method (Optional)")
    debug := flag.String("d", "no", "Print prometheus result to output (Optional)")
    flag.Usage = Usage
    flag.Parse()

    //check flags
    flag.VisitAll(check_set)
    // query prometheus
    response := execute_query(*host,*query,*username,*password)
    // print response (DEBUGGING)
    if (*debug == "yes") {print_response(response)}
    // anaylze response
    analyze_response(response, *warning, *critical, *method, *label)
}

func check_set (argument *flag.Flag) {
    if (argument.Value.String() == "" && argument.Name != "u" && argument.Name != "p" && argument.Name != "d") {
        NagiosMessage = "Please set value for : "+ argument.Name
//        Usage()
        exit_func(UNKNOWN, NagiosMessage)
     }
}

func execute_query(host string, query string, username string, password string) []byte {
    url := host+"/api/v1/query?query="+"("+query+")"
    // because of Monitoring Master we skip verify
    tr := &http.Transport{
	    TLSClientConfig: &tls.Config{InsecureSkipVerify : true},
    }

    client := &http.Client{
        Timeout: time.Second * 10,
        Transport: tr,
    }
    req, err := http.NewRequest("GET", url, nil)
    if (username != "" && password != "") {
        req.SetBasicAuth(username,password)
    }
    resp, err := client.Do(req)
    if err != nil {
        resp.Body.Close()
        exit_func(UNKNOWN, err.Error())
    }
    if (resp.StatusCode != 200) {
        NagiosMessage = resp.Status
        exit_func(UNKNOWN, resp.Status)
    }

    body, err1 := ioutil.ReadAll(resp.Body)
    if err1 != nil {
        resp.Body.Close()
        exit_func(UNKNOWN, err1.Error())
    }
    resp.Body.Close()
    return (body)
}

// this is only for debugging
func print_response(response []byte) {
    var prometheus_response bytes.Buffer
    err := json.Indent(&prometheus_response, response, "", "  ")
    if err != nil {
        fmt.Printf("JSON parse error: ", string(response))
    }
    fmt.Println("Prometheus response:", string(prometheus_response.Bytes()))
}

func analyze_response(response []byte, warning string, critical string, method string, label string) {
    // convert because prometheus response can be float
    w,err := strconv.ParseFloat(warning,64)
    c,err := strconv.ParseFloat(critical,64)

    // convert []byte to json to access it more easily
    json_resp := PrometheusStruct{}
    err = json.Unmarshal(response,&json_resp)
    if err != nil {
        exit_func(UNKNOWN, err.Error())
    }
    result := json_resp.Data.Result
    if (len(result) == 0 ) {
        exit_func(OK,"The check did not return any result") // for example when check is count or sum....
    }

    for _, result := range json_resp.Data.Result {
        value := result.Value[1].(string)
        float_value, _ := strconv.ParseFloat(result.Value[1].(string),64)
        metrics := result.Metric
        //metrics, _ := json.Marshal(result.Metric)
        fmt.Println("Label is:", metrics[label])
        switch  method {
	    case "ge":
            if (set_status_message(float_value, c, "CRITICAL", metrics, value, greaterequal)) {break}
            set_status_message(float_value, w, "WARNING", metrics, value, greaterequal)
	    case "le":
            if (set_status_message(float_value, c, "CRITICAL", metrics, value, lowerequal)) {break}
            set_status_message(float_value, w, "WARNING", metrics, value, lowerequal)
	    case "lt":
            if (set_status_message(float_value, c, "CRITICAL", metrics, value, lowerthan)) {break}
            set_status_message(float_value, w, "WARNING", metrics, value, lowerthan)
	    case "gt":
            if (set_status_message(float_value, c, "CRITICAL", metrics, value, greaterthan)) {break}
            set_status_message(float_value, w, "WARNING", metrics, value, greaterthan)
	    }

    }
    exit_func(NagiosStatus, NagiosMessage)
}

func exit_func (status int, message string) {
    fmt.Printf("%s \n", message)
    os.Exit(status)
}

func set_status_message (float_value float64, compare float64, mess string, metrics map[string]string, value string, f fn) bool{
    if (f(float_value, compare)) {
        NagiosMessage = NagiosMessage+mess+" has value "+value+"\n"
        if ((NagiosStatus == OK || NagiosStatus == WARNING) && mess == "CRITICAL") {
           NagiosStatus = CRITICAL
        }
        if (NagiosStatus == OK && mess == "WARNING") {
           NagiosStatus = WARNING
        }
        return true
    }
    return false
}

type fn func(x float64, y float64) bool
func greaterthan (x float64, y float64) bool {
    return x > y
}
func lowerthan (x float64, y float64) bool {
    return x < y
}
func greaterequal (x float64, y float64) bool {
    return x >= y
}
func lowerequal (x float64, y float64) bool {
    return x <= y
}


func Usage() {
     fmt.Printf("How to: \n ")
     fmt.Printf("$ go build nagitheus.go \n ")
     fmt.Printf("$ ./nagitheus -H \"https://prometheus.aux.spryker.userwerk.gcp.cloud.de.clara.net\" -q \"((kubelet_volume_stats_used_bytes)/kubelet_volume_stats_capacity_bytes)*100>2\" -w 2.7  -c 2.3 -u claradm -p PASSWORD -m le -d yes\n")
     flag.PrintDefaults()
}

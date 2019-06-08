/*
Copyright 2018 Claranet GmbH

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var exit_status int

const (
	OK       = 0
	WARNING  = 1
	CRITICAL = 2
	UNKNOWN  = 3
)

var NagiosMessage struct {
	critical string
	warning  string
}
var NagiosStatus int

type Comparison struct {
	x float64
	y float64
}

func (c *Comparison) GT() bool {
	return c.x > c.y
}
func (c *Comparison) LT() bool {
	return c.x < c.y
}
func (c *Comparison) GE() bool {
	return c.x >= c.y
}
func (c *Comparison) LE() bool {
	return c.x <= c.y
}

// this structure is what promethes gives back when queried.
// The Metric map is not fixed, can vary according to what labels are returned
type PrometheusStruct struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
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
	var labels arrayFlags
	flag.Var(&labels, "l", "Labels to print (Optional)")
	method := flag.String("m", "ge", "Comparison method (Optional)")
	debug := flag.String("d", "no", "Print prometheus result to output (Optional)")
	flag.Usage = Usage
	flag.Parse()

	//check flags
	flag.VisitAll(check_set)
	// query prometheus
	response := execute_query(*host, *query, *username, *password)
	// print response (DEBUGGING)
	if *debug == "yes" {
		print_response(response)
	}
	// anaylze response
	analyze_response(response, *warning, *critical, strings.ToUpper(*method), labels)
}

func check_set(argument *flag.Flag) {
	if argument.Value.String() == "" && argument.Name != "u" && argument.Name != "p" {
		Message := "Please set value for : " + argument.Name
		Usage()
		exit_func(UNKNOWN, Message)
	}
	if argument.Name == "m" {
		method := strings.ToUpper(argument.Value.String())
		Message := "Wrong method. Please set a valid method : GT, LT, GE, LE"
		f := reflect.ValueOf(&Comparison{}).MethodByName(method)
		if !f.IsValid() {
			Usage()
			exit_func(UNKNOWN, Message)
		}
	}
}

func execute_query(host string, query string, username string, password string) []byte {
	query_encoded := url.QueryEscape(query)
	url_complete := host + "/api/v1/query?query=" + "(" + query_encoded + ")"

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Timeout:   time.Second * 10,
		Transport: tr,
	}

	req, err := http.NewRequest("GET", url_complete, nil)
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		exit_func(UNKNOWN, err.Error())
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		exit_func(CRITICAL, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		exit_func(UNKNOWN, err.Error())
	}
	resp.Body.Close()
	return (body)
}

// this is only for debugging via command line to print all response
func print_response(response []byte) {
	var prometheus_response bytes.Buffer
	err := json.Indent(&prometheus_response, response, "", "  ")
	if err != nil {
		fmt.Printf("JSON parse error: ", string(response))
	}
	fmt.Println("Prometheus response:", string(prometheus_response.Bytes()))
}

func analyze_response(response []byte, warning string, critical string, method string, labels []string) {
	// convert because prometheus response can be float
	w, err_w := strconv.ParseFloat(warning, 64)
	c, err_c := strconv.ParseFloat(critical, 64)
	if err_w != nil {
		exit_func(UNKNOWN, "Wrong warning threshold format: "+err_w.Error())
	}
	if err_c != nil {
		exit_func(UNKNOWN, "Wrong critical threshold format: "+err_c.Error())
	}
	// convert []byte to json to access it more easily
	json_resp := PrometheusStruct{}
	err := json.Unmarshal(response, &json_resp)
	if err != nil {
		exit_func(UNKNOWN, err.Error())
	}
	result := json_resp.Data.Result
	if len(result) == 0 {
		exit_func(OK, "OK - The query did not return any result") // for example when check is count or because query returns "no data"
	}

	for _, result := range json_resp.Data.Result {
		value := result.Value[1].(string)
		metrics := result.Metric
		if !set_status_message(c, "CRITICAL", metrics, value, method, labels) {
			set_status_message(w, "WARNING", metrics, value, method, labels)
		}
	}
	if NagiosMessage.critical == "" && NagiosMessage.warning == "" {
		exit_func(NagiosStatus, "OK")
	}
	exit_func(NagiosStatus, NagiosMessage.critical+NagiosMessage.warning)
}

func exit_func(status int, message string) {
	fmt.Printf("%s \n", message)
	os.Exit(status)
}

func set_status_message(compare float64, mess string, metrics map[string]string, value string, method string, labels []string) bool {

	aggregated_labels := ""
	for _, label := range labels {
		if label_value, ok := metrics[label]; ok {
			aggregated_labels = aggregated_labels + label + ":" + label_value + " "
		}
	}
	float_value, _ := strconv.ParseFloat(value, 64)
	c := Comparison{float_value, compare}                                  // structure with result value and comparison (w or c)
	fn := reflect.ValueOf(&c).MethodByName(method).Call([]reflect.Value{}) // call the function with name method
	if fn[0].Bool() {                                                      // get the result of the function called above
		if mess == "CRITICAL" {
			NagiosMessage.critical = NagiosMessage.critical + mess + " " + aggregated_labels + " is " + value + " "
			if NagiosStatus == OK || NagiosStatus == WARNING {
				NagiosStatus = CRITICAL
			}
		} else {
			NagiosMessage.warning = NagiosMessage.warning + mess + " " + aggregated_labels + " is " + value + " "
			if NagiosStatus == OK {
				NagiosStatus = WARNING
			}
		}
		return true
	}
	return false
}

func Usage() {
	fmt.Printf("How to: \n ")
	fmt.Printf("$ go build nagitheus.go \n ")
	fmt.Printf("$ ./nagitheus -H \"https://prometheus.example.com\" -q \"query\" -w 2  -c 3 -u User -p PASSWORD \n")
	flag.PrintDefaults()
}

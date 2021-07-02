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
	"text/template"
	"time"
)

var exit_status int

const (
	OK               = 0
	WARNING          = 1
	CRITICAL         = 2
	UNKNOWN          = 3
	NagitheusVersion = "1.6.0"
)

var NagiosMessage struct {
	critical string
	warning  string
	ok       string
	summary  string
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

var valueMapping map[string]string

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

type TemplateData struct {
	Label  string
	Value  string
	Metric map[string]string
}

func main() {
	host := flag.String("H", "", "Host to query (Required, i.e. https://example.prometheus.com)")
	query := flag.String("q", "", "Prometheus query (Required)")
	warning := flag.String("w", "", "Warning treshold (Required)")
	critical := flag.String("c", "", "Critical treshold (Required)")
	username := flag.String("u", "", "Username (Optional)")
	password := flag.String("p", "", "Password (Optional)")
	token := flag.String("t", "", "Token (Optional)")
	label := flag.String("l", "none", "Label to print (Optional)")
	format := flag.String("f", "", "Format message with go template (Optional)")
	method := flag.String("m", "ge", "Comparison method (Optional)")
	max_chars := flag.String("max-chars", "", "Max. count of characters to print")
	debug := flag.String("d", "no", "Print prometheus result to output (Optional)")
	detailed_print := flag.Bool("print-details", false, "Prints all returned values on multiline result")
	on_missing := flag.String("critical-on-missing", "no", "Return CRITICAL if query results are missing (Optional)")
	value_mapping := flag.String("value-mapping", "", "Mapping result metrics for output (Optional, json i.e. '{\"0\":\"down\",\"1\":\"up\"}')")
	value_unit := flag.String("value-unit", "", "Unit of the value for output (Optional, i.e. '%')")
	version := flag.Bool("v", false, "Prints nagitheus version")
	flag.Usage = Usage
	flag.Parse()

	// nagitheus version
	if *version {
		fmt.Println("Nagitheus version :", NagitheusVersion)
		os.Exit(0)
	}
	// check flags
	flag.VisitAll(check_set)
	// query prometheus
	response := execute_query(*host, *query, *username, *password, *token)
	// print response (DEBUGGING)
	if *debug == "yes" {
		print_response(response)
	}
	// load value mapping if given
	if len(*value_mapping) > 0 {
		json.Unmarshal([]byte(*value_mapping), &valueMapping)
	}

	if len(*format) == 0 {
		*format = "{{.Label}} is {{.Value}}"
	}
	tmpl, err := template.New("format").Parse(*format)
	if err != nil {
		exit_func(UNKNOWN, err.Error(), 0)
	}
	// anaylze response
	analyze_response(response, *warning, *critical, strings.ToUpper(*method), *label, *on_missing, *detailed_print, *max_chars, valueMapping, *value_unit, tmpl)
}

func check_set(argument *flag.Flag) {
	if argument.Value.String() == "" && argument.Name != "u" && argument.Name != "p" && argument.Name != "t" &&
		argument.Name != "value-mapping" && argument.Name != "value-unit" && argument.Name != "max-chars" && argument.Name != "f" {
		Message := "Please set value for : " + argument.Name
		Usage()
		exit_func(UNKNOWN, Message, 0)
	}
}

func execute_query(host string, query string, username string, password string, token string) []byte {
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
	if token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		exit_func(UNKNOWN, err.Error(), 0)
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		exit_func(CRITICAL, resp.Status, 0)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		exit_func(UNKNOWN, err.Error(), 0)
	}
	resp.Body.Close()
	return (body)
}

// this is only for debugging via command line to print all response
func print_response(response []byte) {
	var prometheus_response bytes.Buffer
	err := json.Indent(&prometheus_response, response, "", "  ")
	if err != nil {
		fmt.Print("JSON parse error: ", string(response))
	}
	fmt.Println("Prometheus response:", string(prometheus_response.Bytes()))
}

func analyze_response(response []byte, warning string, critical string, method string, label string,
	on_missing string, detailed_print bool, max_chars string, valueMapping map[string]string, value_unit string, tmpl *template.Template) {
	var count_crit int
	var count_warn int
	var count_ok int

	max_chars_int, _ := strconv.ParseInt(max_chars, 10, 64)

	// convert []byte to json to access it more easily
	json_resp := PrometheusStruct{}
	err := json.Unmarshal(response, &json_resp)
	if err != nil {
		exit_func(UNKNOWN, err.Error(), 0)
	}
	result := json_resp.Data.Result
	// Missing query result: for example when check is count or because query returns "no data"
	if len(result) == 0 && on_missing == "no" {
		exit_func(OK, "OK - The query did not return any result", max_chars_int)
	} else if len(result) == 0 && on_missing == "yes" {
		exit_func(CRITICAL, "CRITICAL - The query did not return any result", max_chars_int)
	}

	for _, result := range json_resp.Data.Result {
		value := result.Value[1].(string)
		metrics := result.Metric
		if set_status_message(critical, "CRITICAL", metrics, value, method, label, valueMapping, value_unit, tmpl) {
			count_crit++
		} else if set_status_message(warning, "WARNING", metrics, value, method, label, valueMapping, value_unit, tmpl) {
			count_warn++
		} else {
			count_ok++
		}
	}

	// if there's only one item in the result return one line
	// else return a summary and multilines if detailed print is activated
	if count_crit+count_warn == 0 && count_ok > 1 && detailed_print == true {
		if label == "none" {
			label = "item"
		}
		NagiosMessage.summary = "OK " + strconv.Itoa(count_ok) + " " + label + " ok :\n------\n"
	} else if count_crit+count_warn+count_ok > 1 && detailed_print == true {
		switch NagiosStatus {
		case 0:
			NagiosMessage.summary = "OK "
		case 1:
			NagiosMessage.summary = "WARNING "
		case 2:
			NagiosMessage.summary = "CRITICAL "
		default:
			NagiosMessage.summary = "UNKNOWN "
		}
		if label == "none" {
			label = "item"
		}
		NagiosMessage.summary = NagiosMessage.summary + strconv.Itoa(count_crit) + " " + label + " critical, " + strconv.Itoa(count_warn) + " " + label + " warning, " + strconv.Itoa(count_ok) + " " + label + " ok :\n------\n"
	} else if count_crit+count_warn > 1 && detailed_print == false {
		exit_func(NagiosStatus, strings.ReplaceAll(NagiosMessage.critical+NagiosMessage.warning, "\n", " "), max_chars_int)
	} else if count_crit+count_warn == 0 && count_ok > 1 && detailed_print == false {
		exit_func(NagiosStatus, "OK", max_chars_int)
	}
	exit_func(NagiosStatus, strings.TrimSuffix(NagiosMessage.summary+NagiosMessage.critical+NagiosMessage.warning+NagiosMessage.ok, "\n"), max_chars_int)

}

func exit_func(status int, message string, max_chars int64) {
	if max_chars > 0 && int64(len(message)) > max_chars {
		fmt.Printf("%s\n", message[:max_chars])
	} else {
		fmt.Printf("%s\n", message)
	}
	os.Exit(status)
}

func set_status_message(compare string, mess string, metrics map[string]string, value string, method string, label string, valueMapping map[string]string, value_unit string, tmpl *template.Template) bool {
	// convert because prometheus response can be float
	float_compare, _ := strconv.ParseFloat(compare, 64)
	float_value, _ := strconv.ParseFloat(value, 64)

	// if value mapping exist, replace it for output
	mapped_value := valueMapping[value]
	if len(mapped_value) > 0 {
		value = mapped_value
	}

	// prepare label and its value for output
	label_value := "value"
	if len(metrics[label]) > 0 {
		label_value = label + " " + metrics[label]
	}
	if len(value_unit) > 0 {
		value = value + " " + value_unit
	}

	tmpl_data := TemplateData{label_value, value, metrics}
	var tmpl_msg bytes.Buffer
	err := tmpl.Execute(&tmpl_msg, tmpl_data)
	if err != nil {
		exit_func(UNKNOWN, err.Error(), 0)
	}

	c := Comparison{float_value, float_compare}                            // structure with result value and comparison (w or c)
	fn := reflect.ValueOf(&c).MethodByName(method).Call([]reflect.Value{}) // call the function with name method
	if fn[0].Bool() {                                                      // get the result of the function called above
		if mess == "CRITICAL" {
			NagiosMessage.critical = NagiosMessage.critical + mess + " " + tmpl_msg.String() + "\n"
			if NagiosStatus == OK || NagiosStatus == WARNING {
				NagiosStatus = CRITICAL
			}
		} else {
			NagiosMessage.warning = NagiosMessage.warning + mess + " " + tmpl_msg.String() + "\n"
			if NagiosStatus == OK {
				NagiosStatus = WARNING
			}
		}
		return true
	}
	if mess == "WARNING" {
		NagiosMessage.ok = NagiosMessage.ok + "OK" + " " + tmpl_msg.String() + "\n"
	}
	return false
}

func Usage() {
	fmt.Printf("How to: \n ")
	fmt.Printf("$ go build nagitheus.go \n ")
	fmt.Printf("$ ./nagitheus -H \"https://prometheus.example.com\" -q \"query\" -w 2 -c 3 -u User -p PASSWORD \n\n")
	flag.PrintDefaults()
}

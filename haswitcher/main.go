package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
	"os"
	"os/exec"
	"encoding/json"
	"regexp"
	"text/template"
	"bytes"
	"flag"
)

var configfile string
func init() {
	flag.StringVar(&configfile, "config", "config.json", "path to config.json")
	flag.Parse()
}

type Configuration struct {
	Proxies       []string
	Username      string
	Password      string
	CheckInterval int
	CheckTimeout  int
	ListenAddress string
	StateCommand  string
	StateArgs     []string
	StateRegex    string
	SwitchCommand string
	SwitchArgs    []string
	Chdir         string
}

type CommandLineArgs struct {
	NewIpAddress string
}

type State struct {
	ActiveIp           string
	PreviouslyActiveIp string
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Available endpoints: /state, /ping, /switch")
}

func handlerPing(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Available endpoints: /state, /ping, /switch")
}

func handlerSwitch(w http.ResponseWriter, r *http.Request) {
	cfg, _ := getConfiguration(configfile)
	active := getSwitchState(cfg)
	fmt.Fprintf(w, "Active: %s\n", active)
	newActive := getAlternativeIp(cfg, active)
	done := switchState(cfg, newActive)
	fmt.Fprintf(w, "New Active: %s\n", newActive)
	fmt.Fprintf(w, "Command Result\n")
	fmt.Fprintf(w, "%s", done)
}

func handlerState(w http.ResponseWriter, r *http.Request) {
	cfg, _ := getConfiguration(configfile)
	active := getSwitchState(cfg)
	fmt.Fprintf(w, "Active: %s", active)
}

func findStringSubmatchMap(r *regexp.Regexp, s string) map[string]string {
	captures := make(map[string]string)

	match := r.FindStringSubmatch(s)
	if match == nil {
		return captures
	}

	for i, name := range r.SubexpNames() {
		if i == 0 {
			continue
		}
		captures[name] = match[i]
	}
	return captures
}

func checkServers(ticker *time.Ticker, quit <-chan struct{}, cfg Configuration) {
	for {
		select {
		case <-ticker.C:
			for _, ipaddr := range cfg.Proxies {
				_, err := getHAProxyData(ipaddr, cfg.Username, cfg.Password,
					time.Duration(time.Duration(cfg.CheckTimeout) * time.Second))
				if err != nil {
					//active := getSwitchState(cfg)
					//if active == ipaddr {
					//    //TODO switch state
					//    //TODO: log unresponsive HAProxy
					//}
					fmt.Println(err)
				} else {
					//fmt.Printf("'%s'", data)
				}
			}
		//TODO: compare the two data, get better one, switch if needed
		case <-quit:
			ticker.Stop()
			return
		}
	}
}

func getAlternativeIp(cfg Configuration, activeIp string) string {
	for _, ipAddr := range cfg.Proxies {
		if activeIp != ipAddr {
			return ipAddr
		}
	}
	return ""
}

func getHAProxyData(addr string, user string, pass string, timeout time.Duration) (string, error) {
	client := &http.Client{
		Timeout: timeout,
	}
	req, err := http.NewRequest("GET", "http://" + addr + ":1936/stats;csv", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(user, pass)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return string(data), err
}

func getSwitchState(cfg Configuration) string {
	raw, _ := exec.Command(cfg.StateCommand, cfg.StateArgs...).Output()
	out := string(raw)
	myExp := regexp.MustCompile(cfg.StateRegex)
	result := findStringSubmatchMap(myExp, out)
	return result["active_ip"]
}

func switchState(cfg Configuration, newActive string) string {
	currentlyActive := getSwitchState(cfg)
	if currentlyActive == newActive {
		return "";
	}
	context := CommandLineArgs{ NewIpAddress: newActive }
	args := compileCommandLineArgs(cfg.SwitchArgs, context)
	raw, _ := exec.Command(cfg.StateCommand, args...).Output()
	out := string(raw)
	return out
}

func getConfiguration(file string) (Configuration, error) {
	filestream, _ := os.Open(file)
	decoder := json.NewDecoder(filestream)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)
	return configuration, err
}

func compileCommandLineArgs(args []string, values CommandLineArgs) []string {
	parsed := make([]string, 0)
	for _, arg := range args {
		var buffer bytes.Buffer
		tmpl, err := template.New(arg).Parse(arg)
		tmpl.Execute(&buffer, values)
		if err == nil {
			if(len(buffer.String()) > 0) {
				parsed = append(parsed, buffer.String())
			}
		} else {
			fmt.Println(err)
		}
	}
	return parsed
}

func main() {
	cfg, err := getConfiguration(configfile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Chdir(cfg.Chdir)
	ticker := time.NewTicker(time.Duration(cfg.CheckInterval) * time.Second)
	quit := make(chan struct{})

	go checkServers(ticker, quit, cfg)

	http.HandleFunc("/", handler)
	http.HandleFunc("/ping", handlerPing)
	http.HandleFunc("/switch", handlerSwitch)
	http.HandleFunc("/state", handlerState)
	//TODO: handle /ping
	http.ListenAndServe(cfg.ListenAddress, nil)
}

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hi")
}

func checkServers(ticker *time.Ticker, quit <-chan struct{}) {
	for {
		select {
		case <- ticker.C:
			fmt.Println("tick")
			data, err := getHAProxyData("136.243.65.135", "haproxy", "stats", time.Duration(1 * time.Second))
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("'%s'", data)
			}
		case <- quit:
			ticker.Stop()
			return
		}
	}
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

func main() {
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})

	go checkServers(ticker, quit)

	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}

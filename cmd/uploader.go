//+build !test

package main

/*
	Utility program to upload new files and remove deleted ones.

	Should everything happen in a single request?  Or should this utility
	upload game and image caches and have the server respond with files it's
	missing?

	URL will be something like `/update`. Put the key will be in the header
	instead of adding it to the URL.
*/

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	ss "github.com/zorchenhimer/steam-screenshots"
)

func main() {
	config, err := ReadConfig("upload-config.json")
	if err != nil {
		fmt.Println("Error reading config: ", err)
		return
	}

	reqUrl := fmt.Sprintf("%s/api/get-cache", config.Server)
	fmt.Println("request url: ", reqUrl)

	req, err := http.NewRequest("POST", reqUrl, nil)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	req.Header.Add("api-key", config.Key)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(resp.Status)
	if resp.Body == nil {
		fmt.Println("nil body")
		os.Exit(1)
	}

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//fmt.Printf("body: %q\n", raw)

	serverCache, err := ss.ParseImageCache(raw)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("image cache from server:")
	for key, _ := range serverCache.Games {
		fmt.Printf("  %s\n", key)
	}

	localCache, err := ss.FullScan(config.RemoteDirectory, true)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//removed := []string{}
	for key, val := range localCache.Games {
		fmt.Printf("[%s] %s\n", key, val)
	}

	//fmt.Println(resp.Body)
	//fmt.Println(config)
}

type Configuration struct {
	Server          string // Server IP/URL and Port with preceding "http://" or "https://"
	Key             string // Upload key.  This needs to be kept private.
	RemoteDirectory string // steam's "remote" directory
}

func ReadConfig(filename string) (*Configuration, error) {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	config := &Configuration{}
	err = json.Unmarshal(raw, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (cfg Configuration) String() string {
	return "Server: " + cfg.Server + " Key: " + cfg.Key
}

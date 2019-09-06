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
	"path/filepath"

	ss "github.com/zorchenhimer/steam-screenshots"
)

var config *Configuration
var err error

func main() {
	config, err = ReadConfig("upload-config.json")
	if err != nil {
		fmt.Println("Error reading config: ", err)
		return
	}

	raw, err := apiRequest("get-cache")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	serverCache, err := ss.ParseImageCache(raw)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("image cache from server:")
	for key, _ := range serverCache.Games {
		fmt.Printf("  %s\n", key)
	}

	//rawgames, err := apiRequest("get-games")
	//if err != nil {
	//	fmt.Println(err)
	//	os.Exit(1)
	//}

	//gamecache, err := ss.ParseGames(rawgames)
	//if err != nil {
	//	fmt.Println(err)
	//	os.Exit(1)
	//}
	//_ = gamecache

	fmt.Println("Running full scan")
	localCache, err := ss.FullScan(config.RemoteDirectory)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	removed := map[string][]string{}
	added := map[string][]ss.ImageMeta{}
	for key, val := range localCache.Games {
		//fmt.Printf("[%s] %s\n", key, val)
		for _, img := range val {
			if !serverCache.Contains(key, img.Name) {
				added[key] = val
			}
		}
	}

	for key, val := range serverCache.Games {
		for _, img := range val {
			if !localCache.Contains(key, img.Name) {
				if _, ok := removed[key]; !ok {
					removed[key] = []string{}
				}
				removed[key] = append(removed[key], img.Name)
			}
		}
	}

	fmt.Println("removed files:", removed)
	fmt.Println("added files:", added)
	toRemove := 0
	if len(removed) > 0 {
		fmt.Println("images removed")
		for _, files := range removed {
			toRemove += len(files)
		}
	}
	fmt.Printf("images to remove: %d\n", toRemove)

	toUpload := 0
	if len(added) > 0 {
		fmt.Println("images added")
		for gamekey, files := range added {
			toUpload += len(files)
			for _, f := range files {
				fmt.Printf("file to upload: [%s] %s\n", gamekey, f)
				err := uploadImage(gamekey, f)
				if err != nil {
					fmt.Printf("error uploading image: %s\n", err)
				}
			}
		}
	}
	//fmt.Printf("images to add: %d\n", toUpload)

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

func apiRequest(endpoint string) ([]byte, error) {
	reqUrl := fmt.Sprintf("%s/api/%s", config.Server, endpoint)
	fmt.Println("request url: ", reqUrl)

	req, err := http.NewRequest("POST", reqUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("api-key", config.Key)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	//fmt.Println(resp.Status)
	if resp.Body == nil {
		return nil, fmt.Errorf("nil body")
	}

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// TODO: thumbnail
func uploadImage(game string, meta ss.ImageMeta) error {
	reqUrl := fmt.Sprintf("%s/api/add-image", config.Server)

	fname := filepath.Join(config.RemoteDirectory, game, "screenshots", meta.Name)
	//fmt.Printf("filename to upload: %s\n", fname)
	imgFile, err := os.Open(fname)
	if err != nil {
		return fmt.Errorf("Unable to open %q for uploading: %s", fname, err)
	}

	req, err := http.NewRequest("POST", reqUrl, imgFile)
	if err != nil {
		return fmt.Errorf("Unable to create request: %s", err)
	}

	req.Header.Add("api-key", config.Key)
	req.Header.Add("filename", meta.Name)
	req.Header.Add("game-id", game)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Error sending request: %s\n", err)
	}

	//fmt.Println(resp.Status)
	if resp.StatusCode != 200 {
		fmt.Printf("Non 200 status code returned: %d\n", resp.Status)
	}

	//if resp.Body != nil {
	//	raw, err := ioutil.ReadAll(resp.Body)
	//	if err != nil {
	//		return fmt.Errorf("Unable to read body: %s", err)
	//	}
	//	fmt.Println(raw)
	//}

	return nil
}

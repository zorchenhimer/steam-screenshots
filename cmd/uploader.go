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
	//"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexflint/go-arg"

	ss "github.com/zorchenhimer/steam-screenshots"
)

var (
	config *Configuration
	err error
	client *http.Client
)

type ImageCache map[string]map[string]*ss.ImageMeta

type Arguments struct {
	SettingsFile string `arg:"-c,--config" default:"upload-config.json"`
}

func main() {
	args := &Arguments{}
	arg.MustParse(args)

	config, err = ReadConfig(args.SettingsFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	client = &http.Client{}

	// Check if we should run once or continuously
	if config.Interval > 0 {
		fmt.Printf("Running uploader in continuous mode with interval of %d seconds\n", config.Interval)
		for {
			if err := run(); err != nil {
				fmt.Printf("Upload error: %v\n", err)
			}
			fmt.Printf("Sleeping for %d seconds...\n", config.Interval)
			time.Sleep(time.Duration(config.Interval) * time.Second)
		}
	} else {
		// Single run mode
		if err := run(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

func run() error {

	raw, err := apiRequest("get-cache", nil)
	if err != nil {
		return err
	}

	remote := ImageCache{}
	err = json.Unmarshal(raw, &remote)
	//serverCache, err := ss.ParseImageCache(raw)
	if err != nil {
		return err
	}

	local, err := scanForImages(config.RemoteDirectory)
	if err != nil {
		return err
	}

	fmt.Println("remote count:", len(remote))
	fmt.Println("local count:", len(local))

	added := make(map[string][]string)

	for appid, files := range local {
		if _, exists := remote[appid]; !exists {
			added[appid] = files
			continue
		}

		for _, lfile := range files {
			if _, exists := remote[appid][lfile]; exists {
				continue
			}

			added[appid] = append(added[appid], lfile)
		}
	}

	fmt.Println("new files:")
	for appid, files := range added {
		fmt.Println(" ", appid)
		for _, f := range files {
			fmt.Println("   ", f)
			err = uploadFile(appid, f)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func uploadFile(appid, filename string) error {
	file, err := os.Open(filepath.Join(config.RemoteDirectory, appid, "screenshots", filename))
	if err != nil {
		return err
	}
	defer file.Close()

	reqUrl := fmt.Sprintf("%s/api/upload/%s/%s", config.Server, appid, filename)
	fmt.Println("request url: ", reqUrl)

	req, err := http.NewRequest("PUT", reqUrl, file)
	req.Header.Add("api-key", config.Key)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return nil
}

type Configuration struct {
	Server          string // Server IP/URL and Port with preceding "http://" or "https://"
	Key             string // Upload key.  This needs to be kept private.
	RemoteDirectory string // steam's "remote" directory
	Interval        int    // Interval in seconds between upload checks (0 = run once)
}

func ReadConfig(filename string) (*Configuration, error) {
	raw, err := os.ReadFile(filename)
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

func apiRequest(endpoint string, headers map[string]string) ([]byte, error) {
	reqUrl := fmt.Sprintf("%s/api/%s", config.Server, endpoint)
	fmt.Println("request url: ", reqUrl)

	req, err := http.NewRequest("POST", reqUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("api-key", config.Key)
	for key, val := range headers {
		req.Header.Add(key, val)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	//fmt.Println(resp.Status)
	if resp.Body == nil {
		return nil, fmt.Errorf("nil body")
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func scanForImages(root string) (map[string][]string, error) {
	rootdir, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	images := make(map[string][]string)

	for _, dir := range rootdir {
		if !dir.IsDir() || strings.HasPrefix(dir.Name(), ".") {
			continue
		}

		dname := dir.Name()
		images[dname] = []string{}

		files, err := os.ReadDir(filepath.Join(root, dname, "screenshots"))
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			switch strings.ToLower(filepath.Ext(file.Name())) {
			case ".jpg", ".jpeg", ".png":
				images[dname] = append(images[dname], file.Name())
			default:
				// nope
			}
		}
	}

	return images, nil
}


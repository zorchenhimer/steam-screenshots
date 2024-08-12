package steamscreenshots

/*
	Broken:
		non-directories in root being added as games

	TODO:
		clean up directory scan code (why is it duplicated?)
		add ability to turn off directory scanning on server
*/

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	//"path/filepath"
	"regexp"
	//"strings"
	"syscall"
	"time"
)

type Settings struct {
	ImageDirectory  string
	Address         string
	AppidOverrides  []struct {
		Appid string `json:"id"`
		Name  string `json:"name"`
	}
	RefreshInterval int    // In minutes
	ApiKey          string // This will be regenerated if it is empty.
	ApiWhitelist    []string
}

var re_gamename = regexp.MustCompile(`<td itemprop="name">(.+?)</td>`)

var (
	gitCommit string
	version   string
)

// Structure of json from steam's servers
type steamapps struct {
	Applist struct {
		Apps []struct {
			Appid uint64 `json:"appid"`
			Name  string `json:"name"`
		} `json:"apps"`
	} `json:"applist"`
}

type Server struct {
	// stats stuff
	startTime time.Time
	lastScan  time.Time

	lastUpdate *time.Time

	settings Settings

	Games      *GameList
	ImageCache *GameImages

	SettingsFile string
	StaticFiles fs.FS
}

func NewServer(settingsFile string) (*Server, error) {
	s := &Server{
		SettingsFile: settingsFile,
		StaticFiles: &staticFiles{},
	}

	if err := s.loadSettings(settingsFile); err != nil {
		return nil, fmt.Errorf("Error loading settings: %w", err)
	}

	fmt.Println("Whitelisted API addresses:")
	for _, val := range s.settings.ApiWhitelist {
		fmt.Println("   ", val)
	}

	if err := init_templates(); err != nil {
		return nil, fmt.Errorf("Error loading templates: %w", err)
	}

	if len(gitCommit) == 0 {
		gitCommit = "Missing commit hash"
	}

	if len(version) == 0 {
		version = "Missing version info"
	}
	fmt.Printf("%s@%s\n", version, gitCommit)

	s.startTime = time.Now()

	return s, nil
}

func (s *Server) Run() error {
	fmt.Println("Starting server")

	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", s.handler_main)
	mux.HandleFunc("/game/{appid}/{$}", s.handler_game)
	mux.HandleFunc("/thumb/{appid}/{filename}", s.handler_thumb)
	mux.HandleFunc("/img/{appid}/{filename}", s.handler_image)
	//mux.HandleFunc("/banner/{appid}", s.handler_banner)
	mux.HandleFunc("/static/{filename}", s.handler_static)
	mux.HandleFunc("/static/{subdir}/{filename}", s.handler_static)
	mux.HandleFunc("/debug/", s.handler_debug)
	//mux.HandleFunc("/api/get-cache", s.handler_api_cache)
	//mux.HandleFunc("/api/get-games", s.handler_api_games)
	//mux.HandleFunc("/api/add-image", s.handler_api_addImage)

	server := &http.Server{
		Addr:           s.settings.Address,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	var err error
	s.ImageCache, err = LoadImageCache("image.cache", s.settings.ImageDirectory)
	if err != nil {
		return fmt.Errorf("error loading image cache: %w", err)
	}

	go func(s *Server) {
		for {
			err := s.ImageCache.Scan()
			if err != nil {
				fmt.Println("Error scanning for images:", err)
			}
			time.Sleep(2*time.Minute)
		}
	}(s)

	// Generate a new API key if it's empty
	if s.settings.ApiKey == "" {
		out := ""
		large := big.NewInt(int64(1 << 60))
		large = large.Add(large, large)
		for len(out) < 50 {
			num, err := rand.Int(rand.Reader, large)
			if err != nil {
				panic("Error generating session key: " + err.Error())
			}
			out = fmt.Sprintf("%s%X", out, num)
		}
		s.settings.ApiKey = out
		fmt.Println("New API key generated: " + s.settings.ApiKey)
		if err = s.saveSettings("settings.json"); err != nil {
			panic(fmt.Sprintf("unable to save settings: %v", err))
		}
	} else {
		fmt.Printf("using API key in config: %q\n", s.settings.ApiKey)
	}

	fmt.Println("Listening on address: " + s.settings.Address)
	fmt.Println("Fisnished startup.")

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("Listen: ", err)
		}
	}()

	fmt.Println("Started")

	<-done
	fmt.Println("Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		fmt.Println("Clean shutdown failed: ", err)
	}

	fmt.Println("goodbye")
	return nil
}

func SliceContains(s []string, val string) bool {
	for _, v := range s {
		if v == val {
			return true
		}
	}
	return false
}

func (s *Server) saveSettings(filename string) error {
	raw, err := json.MarshalIndent(s.settings, "", "    ")
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, raw, 0600)
	if err != nil {
		return err
	}

	return os.Chmod(filename, 0600)
}

func (s *Server) loadSettings(filename string) error {
	settingsFile, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Error reading settings file: %s", err)
	}

	err = json.Unmarshal(settingsFile, &s.settings)
	if err != nil {
		return fmt.Errorf("Error unmarshaling settings: %s", err)
	}

	fmt.Println("Settings loaded")

	if s.settings.RefreshInterval < 1 {
		s.settings.RefreshInterval = 1
	}

	// TODO: make this filename configurable
	s.Games, err = LoadGameList("games.cache")
	return err
}

func (s *Server) getGameName(appid string) (string, error) {
	if appid == ".stfolder" {
		return appid, nil
	}

	//fmt.Printf("Getting name for appid %q\n", appid)
	if name := s.Games.Get(appid); name != appid {
		return name, nil
	}

	// Large appid, must be a non-steam game.  This could have some edge cases
	// as non-steam games' appids are CRCs.
	if len(appid) > 18 {
		return s.Games.Set(appid, fmt.Sprintf("Non-Steam game (%s)", appid)), nil
	}

	if err := s.updateGamesJson(); err == nil {
		if name := s.Games.Get(appid); name != appid {
			return name, nil
		}
	}
	return appid, nil
}

// Update the local cache of appids from steam's servers.
func (s *Server) updateGamesJson() error {
	if s.lastUpdate != nil && time.Since(*s.lastUpdate).Minutes() < 30 {
		//return fmt.Errorf("Cache still good.")
		fmt.Println("Not updating games list; cache still good.")
		return nil
	}

	now := time.Now()
	//fmt.Printf("time.Now(): {}\n", now)
	s.lastUpdate = &now

	fmt.Println("Updating games list")
	resp, err := http.Get("http://api.steampowered.com/ISteamApps/GetAppList/v2")
	if err != nil {
		return fmt.Errorf("Unable to get appid list from steam: %s", err)
	}
	defer resp.Body.Close()

	js, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Unable to read appid json: %s", err)
	}

	alist := &steamapps{}
	if err := json.Unmarshal(js, alist); err != nil {
		return fmt.Errorf("Unable to unmarshal json: %s", err)
	}

	for _, a := range alist.Applist.Apps {
		id := fmt.Sprintf("%d", a.Appid)
		s.Games.Set(id, a.Name)
	}

	for _, ovr := range s.settings.AppidOverrides {
		s.Games.Set(ovr.Appid, ovr.Name)
		fmt.Printf("Setting override for [%s]: %q\n", ovr.Appid, ovr.Name)
	}

	// save games.cache
	games := s.Games.GetMap()
	marshaled, err := json.MarshalIndent(games, "", "\t")
	if err != nil {
		return fmt.Errorf("Unable to marshal game json: %s", err)
	}

	err = os.WriteFile("games.cache", marshaled, 0644)
	if err != nil {
		return fmt.Errorf("Unable to save games.cache: %s", err)
	}

	fmt.Printf("Finished updating games list.  Appids: %d\n", len(games))
	return nil
}

// Returns a filename
func (s *Server) getGameBanner(appid string) (string, error) {
	//appstr := fmt.Sprintf("%d", appid)
	if exist := exists("banners/" + appid + ".jpg"); exist {
		return "banners/" + appid + ".jpg", nil
	}

	resp, err := http.Get("http://cdn.akamai.steamstatic.com/steam/apps/" + appid + "/header.jpg")
	if err != nil {
		return "", fmt.Errorf("Unable to DL header: %s", err)
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		// Game not found.  Use unknown.

		raw, err := os.ReadFile("banners/unknown.jpg")
		if err != nil {
			return "", fmt.Errorf("Unable to read unknown.jpg")
		}

		if err = os.WriteFile("banners/"+appid+".jpg", raw, 0777); err != nil {
			return "", fmt.Errorf("Unable to save file: %s", err)
		}

		return "banners/" + appid + ".jpg", nil
	}

	defer resp.Body.Close()

	file, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Unable to read file: %s", err)
	}

	if err = os.WriteFile("banners/"+appid+".jpg", file, 0777); err != nil {
		return "", fmt.Errorf("Unable to save file: %s", err)
	}

	return "banners/" + appid + ".jpg", nil
}

// exists returns whether the given file or directory exists or not.
// Taken from https://stackoverflow.com/a/10510783
func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}

	return fi.IsDir()
}

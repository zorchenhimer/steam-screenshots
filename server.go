package steamscreenshots

/*

	Broken:
		non-directories in root being added as games

	TODO:
		fsnotify
		clean up directory scan code (why is it duplicated?)
		add ability to turn off directory scanning on server
*/

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

type Settings struct {
	RemoteDirectory string
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
}

func (s *Server) Run() {
	fmt.Println("Starting server")

	if len(gitCommit) == 0 {
		gitCommit = "Missing commit hash"
	}

	if len(version) == 0 {
		version = "Missing version info"
	}
	fmt.Printf("%s@%s\n", version, gitCommit)

	s.startTime = time.Now()
	s.Games = NewGameList()

	if err := s.loadSettings("settings.json"); err != nil {
		fmt.Printf("Error loading settings: %s\n", err)
		return
	}

	if err := init_templates(); err != nil {
		fmt.Printf("Error loading templates: %s\n", err)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handler_main)
	mux.HandleFunc("/thumb/", s.handler_thumb)
	mux.HandleFunc("/img/", s.handler_image)
	mux.HandleFunc("/banner/", s.handler_banner)
	mux.HandleFunc("/static/", s.handler_static)
	mux.HandleFunc("/debug/", s.handler_debug)
	mux.HandleFunc("/api/get-cache", s.handler_api_cache)
	mux.HandleFunc("/api/get-games", s.handler_api_games)
	mux.HandleFunc("/api/add-image", s.handler_api_addImage)

	server := &http.Server{
		Addr:           s.settings.Address,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	var err error
	s.ImageCache, err = LoadImageCache("image.cache")
	if err != nil {
		fmt.Println("Unable to load image.cache: ", err)

		s.ImageCache = NewGameImages()
		err = s.scan(true)
		if err != nil {
			fmt.Println("Initial scan error: ", err)
			return
		}
	} else {
		fmt.Println("Refreshing RemoteDirectory...")
		if err = s.scan(true); err != nil {
			fmt.Println("Error refreshing RemoteDirectory: ", err)
			return
		}
	}
	fmt.Println("Initial scan OK")

	// Fire and forget.  TODO: graceful shutdown
	go func() {
		for {
			time.Sleep(time.Minute * time.Duration(s.settings.RefreshInterval))
			if err := s.scan(false); err != nil {
				fmt.Printf("Error scanning: %s", err)
			}
		}
	}()

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

	//go func(s *Server) {
	//	for {
	//		s.ImageCache.Save("image.cache")
	//		time.Sleep(10 * time.Minute)
	//	}
	//}(s)

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
}

// TODO: use GameImages.FullScan for this?
func (s *Server) scan(printOutput bool) error {
	s.lastScan = time.Now()

	if printOutput {
		fmt.Printf("Scanning %q\n", s.settings.RemoteDirectory)
	}

	dir, err := filepath.Glob(filepath.Join(s.settings.RemoteDirectory, "*"))
	if err != nil {
		return fmt.Errorf("Unable to glob RemoteDirectory: %s", err)
	}
	tmpTree := make(map[string][]string)

	for _, d := range dir {
		base := filepath.Base(d)

		// Ignore dotfiles
		if strings.HasPrefix(base, ".") {
			continue
		}

		if !isDir(d) {
			fmt.Printf("%q is not a directory\n", d)
			continue
		}

		if printOutput {
			fmt.Printf("[%s] %s\n", base, s.Games.Get(base))
		}

		jpg, err := filepath.Glob(filepath.Join(d, "screenshots", "*.jpg"))
		if err != nil {
			fmt.Printf("JPG glob error in %q: %s", d, err)
			continue
		}
		tmpTree[base] = jpg

		// TODO: merge ImageCache.ScanPath() and ImagePath.RefreshPath(), possibly removing the jpg glob above as well.
		err = s.ImageCache.ScanPath(d)
		if err != nil {
			fmt.Println(err)
		}
	}

	// Write cache to disk after it's updated in-memory so failing this doesn't skip updating.
	if err := s.ImageCache.Save("image.cache"); err != nil {
		return fmt.Errorf("Unable to save image cache: %s\n", err)
	}

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

	err = ioutil.WriteFile(filename, raw, 0600)
	if err != nil {
		return err
	}

	return os.Chmod(filename, 0600)
}

// FIXME: pass the filename in here as an argument
func (s *Server) loadSettings(filename string) error {
	settingsFile, err := ioutil.ReadFile(filename)
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

	return s.loadGames()
}

func (s *Server) loadGames() error {
	if ex := exists("games.cache"); !ex {
		fmt.Println("games.cache doesn't exist.  Getting a new one.")
		if err := s.updateGamesJson(); err != nil {
			return fmt.Errorf("Unable update game list: %s", err)
		}
	}

	gamesFile, err := ioutil.ReadFile("games.cache")
	if err != nil {
		return fmt.Errorf("Error reading games file: %s", err)
	}

	var games GameIDs
	err = json.Unmarshal(gamesFile, &games)
	if err != nil {
		return fmt.Errorf("Error unmarshaling games: %s", err)
	}

	s.Games.Update(games)
	//fmt.Println("Number of games loaded: ", Games.Length())
	return nil
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

	// TODO: rate limiting/cache age
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

	js, err := ioutil.ReadAll(resp.Body)
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
	marshaled, err := json.Marshal(games)
	if err != nil {
		return fmt.Errorf("Unable to marshal game json: %s", err)
	}

	err = ioutil.WriteFile("games.cache", marshaled, 0644)
	if err != nil {
		return fmt.Errorf("Unable to save games.cache: %s", err)
	}

	fmt.Printf("Finished updating games list.  Appids: %d\n", len(games))
	return nil
}

// Returns a filename
func (s *Server) getGameBanner(appid uint64) (string, error) {
	appstr := fmt.Sprintf("%d", appid)
	if exist := exists("banners/" + appstr + ".jpg"); exist {
		return "banners/" + appstr + ".jpg", nil
	}

	resp, err := http.Get("http://cdn.akamai.steamstatic.com/steam/apps/" + appstr + "/header.jpg")
	if err != nil {
		return "", fmt.Errorf("Unable to DL header: %s", err)
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		// Game not found.  Use unknown.

		raw, err := ioutil.ReadFile("banners/unknown.jpg")
		if err != nil {
			return "", fmt.Errorf("Unable to read unknown.jpg")
		}

		if err = ioutil.WriteFile("banners/"+appstr+".jpg", raw, 0777); err != nil {
			return "", fmt.Errorf("Unable to save file: %s", err)
		}

		return "banners/" + appstr + ".jpg", nil
	}

	defer resp.Body.Close()

	file, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Unable to read file: %s", err)
	}

	if err = ioutil.WriteFile("banners/"+appstr+".jpg", file, 0777); err != nil {
		return "", fmt.Errorf("Unable to save file: %s", err)
	}

	return "banners/" + appstr + ".jpg", nil
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

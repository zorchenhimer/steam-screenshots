package main

import (
    "encoding/json"
    "fmt"
    //"html"
    "io/ioutil"
    "net/http"
    "os"
    "path/filepath"
    "regexp"
    //"strconv"
    "strings"
    "sync"
    "time"

)

var dataTree map[string][]string
var DataLock *sync.Mutex = &sync.Mutex{}

type Settings struct {
    RemoteDirectory string
    Address         string
    AppidOverrides  []struct {
        Appid       string  `json:"id"`
        Name        string  `json:"name"`
    }
}

var LastUpdate  *time.Time

//var dtLock sync.Mutex

var s Settings
var re_gamename = regexp.MustCompile(`<td itemprop="name">(.+?)</td>`)

// stats stuff
var lastScan time.Time
var startTime time.Time

// Structure of json from steam's servers
type steamapps struct {
    Applist struct {
        Apps    []struct {
            Appid   uint64  `json:"appid"`
            Name    string  `json:"name"`
        }   `json:"apps"`
    }   `json:"applist"`
}

func main() {
    startTime = time.Now()

    Games = NewGameList()
    if err := loadSettings(); err != nil {
        fmt.Printf("Error loading settings: %s\n", err)
        return
    }

    if err := init_templates(); err != nil {
        fmt.Printf("Error loading templates: %s\n", err)
        return
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/", handler_main)
    mux.HandleFunc("/thumb/", handler_thumb)
    mux.HandleFunc("/img/", handler_image)
    mux.HandleFunc("/banner/", handler_banner)
    mux.HandleFunc("/static/", handler_static)
    mux.HandleFunc("/debug/", handler_debug)

    server := &http.Server{
        Addr:           s.Address,
        Handler:        mux,
        ReadTimeout:    10 * time.Second,
        WriteTimeout:   10 * time.Second,
        MaxHeaderBytes: 1 << 20,
    }

    var err error
    ImageCache, err = Load("image.cache")
    if err != nil {
        fmt.Println("Unable to load image.cache: ", err)

        ImageCache = NewGameImages()
        err = InitialScan()
        if err != nil {
            fmt.Println("Initial scan error: ", err)
            return
        }
    } else {
        fmt.Println("Refreshing RemoteDirectory...")
        if err = RefreshScan(true); err != nil {
            fmt.Println("Error refreshing RemoteDirectory: ", err)
            return
        }
    }
    fmt.Println("Initial scan OK")

    // Needs to be wrapped in an anon func because it returns an error.
    _ = time.AfterFunc(time.Minute, func(){RefreshScan(false)})

    fmt.Println("Listening on address: " + s.Address)
    fmt.Println("Fisnished startup.")
    server.ListenAndServe()
}

func InitialScan() error {
    lastScan = time.Now()
    dir, err := filepath.Glob(filepath.Join(s.RemoteDirectory, "*"))
    if err != nil {
        return fmt.Errorf("Unable to glob RemoteDirectory: %s", err)
    }
    tmpTree := make(map[string][]string)

    fmt.Println("Scanning RemoteDirectory...")
    for _, d := range dir {
        base := filepath.Base(d)
        if strings.HasPrefix(base, ".") {
            continue
        }
        fmt.Printf("[%s] %s\n", base, Games.Get(base))

        disc, err := discoverDir(d)
        if err != nil {
            fmt.Println(err)
            continue
        }
        tmpTree[base] = disc

        err = ImageCache.ScanPath(d)
        if err != nil {
            return err
        }
    }

    if err := ImageCache.Save("image.cache"); err != nil {
        return fmt.Errorf("Unable to save image cache: %s\n", err)
    }

    DataLock.Lock()
    dataTree = tmpTree
    DataLock.Unlock()

    return nil
}

func RefreshScan(printProgress bool) error {
    defer func() {
        _ = time.AfterFunc(time.Minute, func() {RefreshScan(false)})
    }()

    lastScan = time.Now()
    dir, err := filepath.Glob(filepath.Join(s.RemoteDirectory, "*"))
    if err != nil {
        fmt.Print("Unable to glob RemoteDirectory: ", err)
        return fmt.Errorf("Unable to glob RemoteDirectory: %s", err)
    }
    tmpTree := make(map[string][]string)

    for _, d := range dir {
        base := filepath.Base(d)
        if strings.HasPrefix(base, ".") {
            continue
        }
        if printProgress {
            fmt.Printf("[%s] %s\n", base, Games.Get(base))
        }

        disc, err := discoverDir(d)
        if err != nil {
            fmt.Println(err)
            continue
        }
        tmpTree[base] = disc

        err = ImageCache.RefreshPath(d)
        if err != nil {
            fmt.Println(err)
            continue
        }
    }

    if err := ImageCache.Save("image.cache"); err != nil {
        return fmt.Errorf("Unable to save image cache: %s\n", err)
    }

    DataLock.Lock()
    dataTree = tmpTree
    DataLock.Unlock()

    return nil
}

// TODO: remove the need for this.  Put it in GameImages.RefreshPath() or something.
// Discover things in a single directory
func discoverDir(dir string) ([]string, error) {
    found := []string{}
    jpg, err := filepath.Glob(filepath.Join(dir, "screenshots", "*.jpg"))
    if err != nil {
        return nil, fmt.Errorf("JPG glob error in %q: %s", dir, err)
    }
    found = append(found, jpg...)

    return found, nil
}

func SliceContains(s []string, val string) bool {
    for _, v := range s {
        if v == val {
            return true
        }
    }
    return false
}

func GetKeys(m map[string][]string) []string {
    keys := []string{}
    for k, _ := range m {
        keys = append(keys, k)
    }

    return keys
}

func loadSettings() error {
    settingsFilename := "settings.json"
    if len(os.Args) > 1 {
        settingsFilename = os.Args[1]
    }

    settingsFile, err := ioutil.ReadFile(settingsFilename)
    if err != nil {
        return fmt.Errorf("Error reading settings file: %s", err)
    }

    err = json.Unmarshal(settingsFile, &s)
    if err != nil {
        return fmt.Errorf("Error unmarshaling settings: %s", err)
    }

    fmt.Println("Settings loaded")

    //updateGamesJson("")
    return loadGames()
}

func loadGames() error {
    if ex := exists("games.cache"); !ex {
        fmt.Println("games.cache doesn't exist.  Getting a new one.")
        if err := updateGamesJson(); err != nil {
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

    Games.Update(games)
    //fmt.Println("Number of games loaded: ", Games.Length())
    return nil
}

func getGameName(appid string) (string, error) {
    if appid == ".stfolder" {
        return appid, nil
    }

    //fmt.Printf("Getting name for appid %q\n", appid)
    if name := Games.Get(appid); name != appid {
        return name, nil
    }

    // Large appid, must be a non-steam game.  This could have some edge cases
    // as non-steam games' appids are CRCs.
    if len(appid) > 18 {
        return Games.Set(appid, fmt.Sprintf("Non-Steam game (%s)", appid)), nil
    }

    // TODO: rate limiting/cache age
    if err := updateGamesJson(); err == nil {
        if name := Games.Get(appid); name != appid {
            return name, nil
        }
    }
    return appid, nil
}

// Update the local cache of appids from steam's servers.
func updateGamesJson() error {
    if LastUpdate != nil && time.Since(*LastUpdate).Minutes() < 30 {
        //return fmt.Errorf("Cache still good.")
        fmt.Println("Not updating games list; cache still good.")
        return nil
    }

    now := time.Now()
    //fmt.Printf("time.Now(): {}\n", now)
    LastUpdate = &now

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
        Games.Set(id, a.Name)

    }

    for _, ovr := range s.AppidOverrides {
        Games.Set(ovr.Appid, ovr.Name)
        fmt.Printf("Setting override for [%s]: %q\n", ovr.Appid, ovr.Name)
    }

    // save games.cache
    games := Games.GetMap()
    marshaled, err := json.Marshal(games)
    if err != nil {
        return fmt.Errorf("Unable to marshal game json: %s", err)
    }

    err = ioutil.WriteFile("games.cache", marshaled, 0777)
    if err != nil {
        return fmt.Errorf("Unable to save games.cache: %s", err)
    }

    fmt.Printf("Finished updating games list.  Appids: %d\n", len(games))
    return nil
}

// Returns a filename
func getGameBanner(appid uint64) (string, error) {
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

        if err = ioutil.WriteFile("banners/" + appstr + ".jpg", raw, 0777); err != nil {
            return "", fmt.Errorf("Unable to save file: %s", err)
        }

        return "banners/" + appstr + ".jpg", nil
    }

    defer resp.Body.Close()

    file, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("Unable to read file: %s", err)
    }

    if err = ioutil.WriteFile("banners/" + appstr + ".jpg", file, 0777); err != nil {
        return "", fmt.Errorf("Unable to save file: %s", err)
    }

    return "banners/" + appstr + ".jpg", nil
}

// exists returns whether the given file or directory exists or not.
// Taken from https://stackoverflow.com/a/10510783
func exists(path string) bool {
    _, err := os.Stat(path)
    if err == nil { return true }
    if os.IsNotExist(err) { return false }
    return true
}

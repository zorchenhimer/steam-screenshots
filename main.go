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

type Settings struct {
    RemoteDirectory string
    Address         string
}

type GameIDs map[string]string
var Games *GameList
var LastUpdate  *time.Time

var s Settings
var re_gamename = regexp.MustCompile(`<td itemprop="name">(.+?)</td>`)

// Structure of json from steam's servers
type steamapps struct {
    Applist struct {
        Apps    []struct {
            Appid   uint64  `json:"appid"`
            Name    string  `json:"name"`
        }   `json:"apps"`
    }   `json:"applist"`
}

type GameList struct {
    games   GameIDs
    m       sync.Mutex
}

func NewGameList() *GameList {
    return &GameList{
        games:  make(map[string]string),
    }
}

func (g *GameList) Get(id string) string {
    g.m.Lock()
    defer g.m.Unlock()

    if val, ok := g.games[id]; ok {
        return val
    }
    return id
}

func (g *GameList) Set(id, val string) string {
    g.m.Lock()
    defer g.m.Unlock()

    g.games[id] = val
    return val
}

func (g *GameList) Update(list GameIDs) {
    g.m.Lock()
    defer g.m.Unlock()

    for key, val := range list {
        g.games[key] = val
    }
}

func (g GameList) GetMap() GameIDs {
    g.m.Lock()
    defer g.m.Unlock()

    retList := GameIDs{}
    for key, val := range g.games {
        retList[key] = val
    }
    return retList
}

func (g GameList) Length() int {
    return len(g.games)
}

func main() {
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

    server := &http.Server{
        Addr:           s.Address,
        Handler:        mux,
        ReadTimeout:    10 * time.Second,
        WriteTimeout:   10 * time.Second,
        MaxHeaderBytes: 1 << 20,
    }

    fmt.Println("Fisnished startup.")
    server.ListenAndServe()
}

// Returns a list of folders that have screenshot directories
func discover() (map[string][]string, error) {
    loadSettings()

    dir, err := filepath.Glob(filepath.Join(s.RemoteDirectory, "*"))
    if err != nil {
        return nil, fmt.Errorf("Error Globbing: %s", err)
    }

    found := map[string][]string{}

    for _, d := range dir {
        if strings.HasPrefix(filepath.Base(d), ".") {
            continue
        }

        dfound := []string{}
        jpg, err := filepath.Glob(filepath.Join(d, "screenshots", "*.jpg"))
        if err == nil {
            dfound = append(dfound, jpg...)
        }

        png, err := filepath.Glob(filepath.Join(d, "screenshots", "*.png"))
        if err == nil {
            dfound = append(dfound, png...)
        }

        if len(dfound) > 0 {
            found[filepath.Base(d)] = dfound
        }
    }

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

    return loadGames()
}

func loadGames() error {
    if ex := exists("games.json"); !ex {
        fmt.Println("games.json doesn't exist.  Getting a new one.")
        if err := updateGamesJson(); err != nil {
            return fmt.Errorf("Unable update game list: %", err)
        }
    }

    gamesFile, err := ioutil.ReadFile("games.json")
    if err != nil {
        return fmt.Errorf("Error reading games file: %s", err)
    }

    var games GameIDs
    err = json.Unmarshal(gamesFile, &games)
    if err != nil {
        return fmt.Errorf("Error unmarshaling games: %s", err)
    }

    Games.Update(games)
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

    // save games.json
    games := Games.GetMap()
    marshaled, err := json.MarshalIndent(games, "", "  ")
    if err != nil {
        return fmt.Errorf("Unable to marshal game json: %s", err)
    }

    err = ioutil.WriteFile("games.json", marshaled, 0777)
    if err != nil {
        return fmt.Errorf("Unable to save games.json: %s", err)
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

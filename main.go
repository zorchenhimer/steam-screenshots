package main

import (
    "encoding/json"
    "fmt"
    "html"
    "io/ioutil"
    "net/http"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"
)

type Settings struct {
    RemoteDirectory string
    Address         string
}

type GameIDs map[string]string
var games GameIDs

var s Settings
var re_gamename = regexp.MustCompile(`<td itemprop="name">(.+?)</td>`)

func main() {
    loadSettings()

    if err := init_templates(); err != nil {
        fmt.Printf("Error loading templates: %s\n", err)
        return
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/", handler_main)
    mux.HandleFunc("/thumb/", handler_thumb)
    mux.HandleFunc("/img/", handler_image)

    server := &http.Server{
        Addr:           s.Address,
        Handler:        mux,
        ReadTimeout:    10 * time.Second,
        WriteTimeout:   10 * time.Second,
        MaxHeaderBytes: 1 << 20,
    }

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
    gamesFile, err := ioutil.ReadFile("games.json")
    if err != nil {
        return fmt.Errorf("Error reading games file: %s", err)
    }

    err = json.Unmarshal(gamesFile, &games)
    if err != nil {
        return fmt.Errorf("Error unmarshaling games: %s", err)
    }

    return nil
}

func getGameName(appid string) (string, error) {
    if appid == ".stfolder" {
        return appid, nil
    }
    if name, ok := games[appid]; ok {
        return name, nil
    }

    resp, err := http.Get("https://steamdb.info/app/" + appid)
    if err != nil {
        return appid, fmt.Errorf("Unable to get appid from steamdb: %s", err)
    }

    page, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return appid, fmt.Errorf("Unable to read steamdb response: %s", err)
    }

    match := re_gamename.FindSubmatch(page)
    if len(match) != 2 {
        return appid, fmt.Errorf("Unable to find game name")
    }

    name := html.UnescapeString(string(match[1]))
    unc, err := strconv.Unquote(name)
    if err == nil {
        name = unc
    }
    games[appid] = name
    fmt.Printf("Loaded new appid: [%s] %q\n", appid, name)

    marshaled, err := json.MarshalIndent(games, "", "  ")
    if err != nil {
        return name, fmt.Errorf("Unable to marshal game")
    }

    err = ioutil.WriteFile("games.json", marshaled, 0777)
    if err != nil {
        return name, fmt.Errorf("Unable to save games.json: %s", err)
    }

    return name, nil
}

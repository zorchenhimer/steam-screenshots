package main

import (
    "fmt"
    "net/http"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
)

type StringSliceNoCase []string
func (p StringSliceNoCase) Len() int           { return len(p) }
func (p StringSliceNoCase) Less(i, j int) bool { return strings.ToLower(p[i]) < strings.ToLower(p[j]) }
func (p StringSliceNoCase) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

var dataTree map[string][]string

func SortKeysByValue(m map[string]string) []string {
    vals := []string{}
    for _, v := range m {
        vals = append(vals, v)
    }

    sort.Strings(vals)
    sorted := []string{}
    for _, s := range vals {
        for k, v := range m {
            if v == s {
                sorted = append(sorted, k)
            }
        }
    }

    return sorted
}

func handler_main(w http.ResponseWriter, r *http.Request) {
    // Uncomment this for debugging HTML stuff.
    //if err := init_templates(); err != nil {
    //    fmt.Fprintf(w, "Error reloading templates: %s", err)
    //    return
    //}

    //root, err := discover()
    //if err != nil {
    //    fmt.Fprintf(w, "Error discovering: %s", err)
    //    return
    //}

    //keys := GetKeys(root)
    keys := GetKeys(dataTree)

    if r.URL.Path != "/" {
        trimmed := strings.Trim(r.URL.Path, "/")
        if SliceContains(keys, trimmed) {
            files := dataTree[trimmed]
            sort.Strings(files)
            pretty, err := getGameName(trimmed)
            if err != nil {
                fmt.Printf("Error getting name for %s: %s\n", trimmed, err)
            }

            d := TemplateData{}
            d.Title = pretty
            d.Header = map[string]string{
                "Text":     pretty,
                "Count":    fmt.Sprintf("%d", len(files)),
            }
            d.Body = []map[string]string{}

            for idx, filename := range files {
                base := filepath.Base(filename)
                clearclass := ""
                if idx % 3 == 0 {
                    clearclass = " clearme"
                    //fmt.Printf("Clearme on %q\n", base)
                }
                
                d.Body = append(d.Body, map[string]string{
                    "ImageTarget":  "/img/" + trimmed + "/" + base,
                    "ThumbnailSrc": "/thumb/" + trimmed + "/" + base,
                    "Text":         base,
                    "Clear":        clearclass,
                })
            }

            err = renderTemplate(w, "list", &d)
            if err != nil {
                fmt.Println(err)
            }
        }
    } else {
        //sort.Strings(keys)
        gameNames := map[string]string{}

        d := TemplateData{}
        d.Body = []map[string]string{}
        for _, k := range keys {
            pretty, err := getGameName(k)
            if err != nil {
                fmt.Printf("Error getting name for %s: %s\n", k, err)
            }
            gameNames[pretty] = k
        }

        gameKeys := []string{}
        for k, _ := range gameNames {
            gameKeys = append(gameKeys, k)
        }

        sort.Sort(StringSliceNoCase(gameKeys))

        for idx, pretty := range gameKeys {
            clearclass := ""
            if idx % 3 == 0 {
                clearclass = " clearme"
                //fmt.Printf("Clearme on %q\n", pretty)
            }
            appid := gameNames[pretty]
            d.Body = append(d.Body, map[string]string{
                "Target":   "/" + appid + "/",
                "Pretty":   pretty,
                "Count":    fmt.Sprintf("%d", len(dataTree[appid])),
                "Clear":    clearclass,
            })
        }

        err := renderTemplate(w, "main", &d)
        if err != nil {
            fmt.Println(err)
        }
    }
}

// FIXME: sanitize this shit!
func handler_thumb(w http.ResponseWriter, r *http.Request) {
    split := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(split) != 3 {
        fmt.Fprintf(w, "[split error] %s", split)
        return
    }

    fullPath := filepath.Join(
        s.RemoteDirectory,
        split[1],
        "screenshots",
        "thumbnails",
        split[2])

    http.ServeFile(w, r, fullPath)
}

func handler_image(w http.ResponseWriter, r *http.Request) {
    split := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(split) != 3 {
        fmt.Fprintf(w, "[split error] %s\n", split)
        return
    }

    fullPath := filepath.Join(
        s.RemoteDirectory,
        split[1],
        "screenshots",
        split[2])

    http.ServeFile(w, r, fullPath)
}

func handler_banner(w http.ResponseWriter, r *http.Request) {
    split := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

    if len(split) != 2 {
        fmt.Fprintf(w, "[split error] %s\n", split)
        return
    }

    appidbase := split[1]
    if idx := strings.LastIndex(appidbase, "."); idx > -1 {
        appidbase = appidbase[:idx]
    }

    appid, err := strconv.ParseUint(appidbase, 10, 64)
    if err != nil {
        fmt.Printf("[handle_banner] Invalid appid: %s\n", split[1])
        http.ServeFile(w, r, "banners/unknown.jpg")
        return
    }

    fullPath := fmt.Sprintf("banners/%d.jpg", appid)
    if ex := exists(fullPath); ex {
        http.ServeFile(w, r, fullPath)
    } else {
        bannerPath, err := getGameBanner(appid)
        if err != nil {
            fmt.Printf("[handle_banner] Unable to get banner: %s\n", err)
            http.ServeFile(w, r, "banners/unknown.jpg")
            return
        }

        http.ServeFile(w, r, bannerPath)
    }
}

func handler_static(w http.ResponseWriter, r *http.Request) {
    split := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

    if len(split) != 2 {
        fmt.Printf("[handler_static] split error: %s\n", split)
        return
    }

    fullPath := fmt.Sprintf("static/%s", split[1])
    if ex := exists(fullPath); ex {
        http.ServeFile(w, r, fullPath)
    } else {
        fmt.Printf("[handler_static] 404 on file %q\n", fullPath)
    }
}

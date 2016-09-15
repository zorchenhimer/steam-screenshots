package main

import (
    "fmt"
    "net/http"
    "path/filepath"
    "sort"
    "strings"
)

type StringSliceNoCase []string
func (p StringSliceNoCase) Len() int           { return len(p) }
func (p StringSliceNoCase) Less(i, j int) bool { return strings.ToLower(p[i]) < strings.ToLower(p[j]) }
func (p StringSliceNoCase) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

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
    root, err := discover()
    if err != nil {
        fmt.Fprintf(w, "Error discovering: %s", err)
        return
    }

    keys := GetKeys(root)

    if r.URL.Path != "/" {
        trimmed := strings.Trim(r.URL.Path, "/")
        if SliceContains(keys, trimmed) {
            files := root[trimmed]
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

            for _, filename := range files {
                base := filepath.Base(filename)
                
                d.Body = append(d.Body, map[string]string{
                    "ImageTarget":  "/img/" + trimmed + "/" + base,
                    "ThumbnailSrc": "/thumb/" + trimmed + "/" + base,
                    "Text":         base,
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

        for _, pretty := range gameKeys {
            appid := gameNames[pretty]
            d.Body = append(d.Body, map[string]string{
                "Target":   "/" + appid + "/",
                "Pretty":   pretty,
                "Count":    fmt.Sprintf("%d", len(root[appid])),
            })
        }

        err := renderTemplate(w, "main", &d)
        if err != nil {
            fmt.Println(err)
        }
    }
}

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
        fmt.Fprintf(w, "[split error] %s", split)
        return
    }

    fullPath := filepath.Join(
        s.RemoteDirectory,
        split[1],
        "screenshots",
        split[2])

    http.ServeFile(w, r, fullPath)
}

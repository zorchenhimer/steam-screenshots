package steamscreenshots

// Cache image dimensions in a file

import (
    //"bufio"
    "encoding/json"
    "errors"
    "fmt"
    "image/jpeg"
    //"io"
    "io/ioutil"
    "os"
    "path/filepath"
    //"strconv"
    //"strings"
    "sync"
    "time"
)

//var ImageCache *GameImages

type GameImages struct {
    Games  map[string][]ImageMeta    // appid key
    lock *sync.Mutex
}

type ImageMeta struct {
    Name    string      // filename base
    ModTime time.Time
    Width   int
    Height  int
}

func (i ImageMeta) String() string {
    return fmt.Sprintf("%s; %s; (%d, %d)", i.Name, i.ModTime, i.Width, i.Height)
}

func NewGameImages() *GameImages {
    return &GameImages{
        Games: make(map[string][]ImageMeta),
        lock: &sync.Mutex{},
    }
}

var (
    NoMoreImages error = errors.New("No more images")
    MismatchError error = errors.New("Mismatched key/val lengths")
    NotImplementedError error = errors.New("Not implemented")
)

// Initial scan stuff
func (gi *GameImages) ScanPath(path string) (error) {
    appid := filepath.Base(path)
    dir, err := filepath.Glob(filepath.Join(path, "screenshots", "*.jpg"))
    if err != nil {
        return err
    }

    gi.Games[appid] = []ImageMeta{}

    for _, f := range dir {
        meta, err := readImage(f)
        if err != nil {
            fmt.Println(err)
            continue
        }

        gi.lock.Lock()
        gi.Games[appid] = append(gi.Games[appid], *meta)
        gi.lock.Unlock()
    }

    return nil
}

func (gi *GameImages) RefreshPath(path string) error {
    appid := filepath.Base(path)
    dir, err := filepath.Glob(filepath.Join(path, "screeshots", "*.jpg"))
    if err != nil {
        return err
    }

    // Make sure it's in the cache
    gi.lock.Lock()
    meta, ok := gi.Games[appid]
    gi.lock.Unlock()
    if !ok {
        // Add it if it isn't
        return gi.ScanPath(path)
    }

    OUTER:
    for _, f := range dir {
        fi, err := os.Stat(f)
        if err != nil {
            // Remove image if it no longer exists
            delete(gi.Games, appid)
            continue
        }

        base := filepath.Base(f)
        for _, m := range meta {
            if m.Name == base {
                if m.ModTime.Before(fi.ModTime()) {
                    if newMeta, err := readImage(f); err != nil {
                        fmt.Println(err)
                    } else {
                        gi.lock.Lock()
                        m = *newMeta
                        gi.lock.Unlock()
                    }
                }
                continue OUTER
            }
        }
    }
    return nil
}

func readImage(fullpath string) (*ImageMeta, error) {
    file, err := os.Open(fullpath)
    if err != nil {
        return nil, fmt.Errorf("Unable to open %q: %s", fullpath, err)
    }
    defer file.Close()

    img, err := jpeg.Decode(file)
    if err != nil {
        return nil, fmt.Errorf("Unable to decode %q: %s", fullpath, err)
    }
    pt := img.Bounds().Max

    fi, err := os.Stat(fullpath)
    if err != nil {
        return nil, fmt.Errorf("Unable to stat %q: %s", fullpath, err)
    }

    return &ImageMeta{Name: filepath.Base(fullpath), Width: pt.X, Height: pt.Y, ModTime: fi.ModTime()}, nil
}

func (gi *GameImages) Save(filename string) (error) {
    gi.lock.Lock()
    raw, err := json.Marshal(gi)
    gi.lock.Unlock()
    if err != nil {
        return err
    }

    if err = ioutil.WriteFile(filename, raw, 664); err != nil {
        return err
    }

    return nil
}

func Load(filename string) (*GameImages, error) {
    raw, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, err
    }

    i := NewGameImages()
    if err := json.Unmarshal(raw, i); err != nil {
        return nil, err
    }

    return i, nil
}

func (gi *GameImages) Dump() {
    fmt.Println("Duming ImageCache")
    for g, ms := range gi.Games { 
        fmt.Println(g)
        for _, m := range ms {
            fmt.Println("  ", m)
        }
    }
}

func (gi *GameImages) Length() int {
    gi.lock.Lock()
    defer gi.lock.Unlock()
    return len(gi.Games)
}

func (gi *GameImages) Count(appid string) int {
    gi.lock.Lock()
    defer gi.lock.Unlock()
    return len(gi.Games[appid])
}

type Metadata struct {
    Src     string `json:"src"`
    Width   int `json:"w"`
    Height  int `json:"h"`
}

func (gi *GameImages) GetMetadata(appid string) []Metadata {
    images := []Metadata{}

    theGame, ok := gi.Games[appid]
    if !ok {
        fmt.Printf("[GetMetadata] Unable to find game with appid %s\n", appid)
        return nil
    }

    gi.lock.Lock()
    for _, meta := range theGame {
        images = append(images, Metadata{
            Src:    fmt.Sprintf("/img/%s/%s", appid, meta.Name),
            Width:  meta.Width,
            Height: meta.Height,
        })
    }
    gi.lock.Unlock()

    return images
}

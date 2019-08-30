package steamscreenshots

// Cache image dimensions in a file

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	NoMoreImages        error = errors.New("No more images")
	MismatchError       error = errors.New("Mismatched key/val lengths")
	NotImplementedError error = errors.New("Not implemented")
)

type GameImages struct {
	Games   map[string][]ImageMeta // appid key
	Updated time.Time
	lock    *sync.RWMutex
}

type ImageMeta struct {
	Name   string // filename base
	Width  int
	Height int
}

func (i ImageMeta) String() string {
	return fmt.Sprintf("[%s (%d, %d)]", i.Name, i.Width, i.Height)
}

func NewGameImages() *GameImages {
	return &GameImages{
		Games: make(map[string][]ImageMeta),
		lock:  &sync.RWMutex{},
	}
}

// Load cached image metadata from the given filename.
func LoadImageCache(filename string) (*GameImages, error) {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return ParseImageCache(raw)
}

func ParseImageCache(raw []byte) (*GameImages, error) {
	i := NewGameImages()
	if err := json.Unmarshal(raw, i); err != nil {
		return nil, err
	}

	return i, nil
}

// Initial scan stuff
func (gi *GameImages) ScanNewPath(path string) error {
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

// ScanDirectory scans an entire directory tree, starting from the root.
func FullScan(directory string, printOutput bool) (*GameImages, error) {
	gi := NewGameImages()
	gi.Updated = time.Now()

	if printOutput {
		fmt.Printf("Scanning %q\n", directory)
	}

	dir, err := filepath.Glob(filepath.Join(directory, "*"))
	if err != nil {
		return nil, fmt.Errorf("Unable to glob RemoteDirectory: %s", err)
	}
	gi.Games = make(map[string][]ImageMeta)

	for _, d := range dir {
		base := filepath.Base(d)

		// Ignore dotfiles
		if strings.HasPrefix(base, ".") {
			continue
		}

		if printOutput {
			fmt.Printf("[%s] %s\n", base, "??") //s.Games.Get(base))
		}

		jpgdir, err := filepath.Glob(filepath.Join(d, "screenshots", "*.jpg"))
		if err != nil {
			fmt.Printf("JPG glob error in %q: %s", d, err)
			continue
		}

		// iterate through directory.
		metas := []ImageMeta{}
		for _, img := range jpgdir {
			fmt.Printf("  reading meta for %s\n", img)
			m, err := readImage(img)
			if err != nil {
				fmt.Println(err)
				continue
			}
			metas = append(metas, *m)
		}
		gi.Games[base] = metas

		// TODO: merge ImageCache.ScanPath() and ImagePath.RefreshPath(), possibly removing the jpg glob above as well.
		err = gi.ScanPath(d)
		if err != nil {
			fmt.Println(err)
		}
	}

	// Update in-memory cache
	//gi.Update(tmpTree)

	return gi, nil
}

func (gi *GameImages) ScanPath(path string) error {
	appid := filepath.Base(path)

	dir, err := filepath.Glob(filepath.Join(path, "screenshots", "*.jpg"))
	if err != nil {
		return err
	}

	// Make sure it's in the cache
	gi.lock.Lock()
	meta, ok := gi.Games[appid]
	gi.lock.Unlock()
	if !ok {
		// Add it if it isn't
		return gi.ScanNewPath(path)
	}

	for _, f := range dir {

		// Remove image if it no longer exists.  Improper read permissions is treated as "not existing" here.
		_, err := os.Stat(f)
		if err != nil {
			delete(gi.Games, appid)
			continue
		}

		// foreach image in directory
		found := false
		base := filepath.Base(f)
		for _, m := range meta {

			// match image filename
			if m.Name == base {
				found = true
				break
			}
		}

		// Add new images to the list
		if !found {
			if newImage, err := readImage(f); err != nil {
				fmt.Println(err)
			} else {
				gi.lock.Lock()
				gi.Games[appid] = append(gi.Games[appid], *newImage)
				gi.lock.Unlock()
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

	_, err = os.Stat(fullpath)
	if err != nil {
		return nil, fmt.Errorf("Unable to stat %q: %s", fullpath, err)
	}

	return &ImageMeta{
		Name:   filepath.Base(fullpath),
		Width:  pt.X,
		Height: pt.Y,
	}, nil
}

func (gi *GameImages) Save(filename string) error {
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
	Src    string `json:"src"`
	Width  int    `json:"w"`
	Height int    `json:"h"`
}

func (gi *GameImages) GetMetadata(appid string) []Metadata {
	images := []Metadata{}

	theGame, ok := gi.Games[appid]
	if !ok {
		fmt.Printf("[GetMetadata] Unable to find game with appid %s\n", appid)
		return nil
	}

	gi.lock.RLock()
	for _, meta := range theGame {
		images = append(images, Metadata{
			Src:    fmt.Sprintf("/img/%s/%s", appid, meta.Name),
			Width:  meta.Width,
			Height: meta.Height,
		})
	}
	gi.lock.RUnlock()

	return images
}

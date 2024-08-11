package steamscreenshots

// Cache image dimensions in a file

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// For making our own thumbnails
	"github.com/nfnt/resize"
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

	isDirty bool
}

func (gi GameImages) String() string {
	gi.lock.RLock()
	defer gi.lock.RUnlock()

	b := strings.Builder{}
	for game, meta := range gi.Games {
		b.WriteString(fmt.Sprintf("[%s] %d\n", game, len(meta)))
	}
	return b.String()
}

type ImageMeta struct {
	Name   string // filename base
	Width  int
	Height int
	// TODO: add modified date.
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
	raw, err := os.ReadFile(filename)
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

func (gi *GameImages) addImageMeta(game string, meta ImageMeta) {
	gi.lock.Lock()
	defer gi.lock.Unlock()

	gi.isDirty = true

	if _, ok := gi.Games[game]; !ok {
		gi.Games[game] = []ImageMeta{}
	}
	gi.Games[game] = append(gi.Games[game], meta)
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
		gi.isDirty = true
		gi.lock.Unlock()
	}

	return nil
}

// ScanDirectory scans an entire directory tree, starting from the root.
func FullScan(directory string) (*GameImages, error) {
	gi := NewGameImages()
	gi.Updated = time.Now()

	//if games != nil {
	//	fmt.Printf("Scanning %q\n", directory)
	//}

	dir, err := filepath.Glob(filepath.Join(directory, "*"))
	if err != nil {
		return nil, fmt.Errorf("Unable to glob RemoteDirectory: %s", err)
	}
	gi.Games = make(map[string][]ImageMeta)

	grCount := 8

	wg := &sync.WaitGroup{}
	imgChan := make(chan string, 1000)
	for i := 0; i < grCount; i++ {
		fmt.Printf("starting goroutine %d\n", i)
		go func(c chan string) {
			wg.Add(1)
			fmt.Println("Started goroutine")

			for filename := <-c; filename != ""; filename = <-c {
				//fmt.Println(filename)
				meta, err := readImage(filename)
				if err != nil {
					fmt.Println(err)
					//panic(err)
					continue
				}
				gameId := gameIdFromPath(filename)
				gi.addImageMeta(gameId, *meta)
			}

			wg.Done()
		}(imgChan)
	}
	fmt.Println("gouroutines started")

	for idxdir, d := range dir {
		base := filepath.Base(d)

		// Ignore dotfiles
		if strings.HasPrefix(base, ".") {
			continue
		}
		fmt.Printf("[%d/%d] adding %q\n", idxdir, len(dir), base)

		jpgdir, err := filepath.Glob(filepath.Join(d, "screenshots", "*.jpg"))
		if err != nil {
			fmt.Printf("JPG glob error in %q: %s", d, err)
			continue
		}
		for _, img := range jpgdir {
			imgChan <- img
		}
	}

	fmt.Println("waiting for scan to finish")
	for i := 0; i < grCount; i++ {
		imgChan <- ""
	}
	wg.Wait()

	// Update in-memory cache
	//gi.Update(tmpTree)

	return gi, nil
}

// takes the base remote folder as the path
func (gi *GameImages) RemoveMissing(path string) {

	gi.lock.RLock()
	defer gi.lock.RUnlock()

	// Range over current cache, remove missing items
	for appid, items := range gi.Games {
		// look for appid folder
		_ = items
		apppath := filepath.Join(path, appid)
		_, err := os.Stat(apppath)
		if err != nil {
			gi.lock.RUnlock()
			gi.lock.Lock()

			fmt.Printf("Game with appID %s no longer exists!\n", appid)
			delete(gi.Games, appid)

			gi.isDirty = true

			gi.lock.Unlock()
			gi.lock.RLock()
			continue
		}

		ssdir, err := filepath.Glob(filepath.Join(path, appid, "screenshots", "*.jpg"))
		if err != nil {
			fmt.Printf("Unable to glob screenshot directory for appid %s: %s", appid, err)
			continue
		}

		_ = ssdir
		//fmt.Println("globbed images:")
		//for _, i := range ssdir {
		//	fmt.Println(i)
		//}

		//for _, image := range items {
		//	for _, i := range ssdir {
		//		fmt.Println(i)
		//	}
		//}
	}

	fmt.Println("Finished RemoveMissing()")
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
		//// Remove image if it no longer exists.  Improper read permissions is
		//// treated as "not existing" here.
		//_, err := os.Stat(f)
		//if err != nil {
		//	fmt.Printf("%q no longer exists", f)
		//	delete(gi.Games, appid)
		//	continue
		//}

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
				gi.isDirty = true
				gi.lock.Unlock()
			}
		}
	}
	return nil
}

func (gi *GameImages) Contains(game, img string) bool {
	gi.lock.RLock()
	defer gi.lock.RUnlock()

	imglst, ok := gi.Games[game]
	if !ok {
		return false
	}

	for _, i := range imglst {
		if i.Name == img {
			return true
		}
	}
	return false
}

func readImage(fullpath string) (*ImageMeta, error) {
	file, err := os.Open(fullpath)
	if err != nil {
		return nil, fmt.Errorf("Unable to open %q: %s", fullpath, err)
	}
	defer file.Close()

	// DecodeConfig should read the dimensions of the file without
	// reading the whole file into memory.
	img, err := jpeg.DecodeConfig(file)
	if err != nil {
		return nil, fmt.Errorf("Unable to decode %q: %s", fullpath, err)
	}

	return &ImageMeta{
		Name:   filepath.Base(fullpath),
		Width:  img.Width,
		Height: img.Height,
	}, nil
}

func readRawImage(raw []byte) (*ImageMeta, error) {
	reader := bytes.NewReader(raw)
	config, err := jpeg.DecodeConfig(reader)
	if err != nil {
		return nil, err
	}

	return &ImageMeta{Width: config.Width, Height: config.Height}, nil
}

func (gi *GameImages) Save(filename string) error {
	gi.lock.Lock()
	defer gi.lock.Unlock()
	fmt.Println("Saving image cache")

	if !gi.isDirty {
		return nil
	}

	//if exists(filename) {
	//	_, err := os.Stat(filename)
	//	if err != nil {
	//		return fmt.Errorf("Error Stat()'ing %q: %s", filename, err)
	//	}

	//	// only update if the cache is old
	//	//if fi.ModTime().After(gi.Updated) {
	//	//	return nil
	//	//}
	//}

	gi.Updated = time.Now()
	//fmt.Println("gi.Games: ", gi.Games)
	raw, err := json.MarshalIndent(gi, "", "  ")
	if err != nil {
		return err
	}

	if err = os.WriteFile(filename, raw, 0644); err != nil {
		return err
	}

	gi.isDirty = false
	return nil
}

func (gi *GameImages) Dirty() {
	gi.isDirty = true
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

func (gi *GameImages) GetGames() []string {
	gi.lock.RLock()
	defer gi.lock.RUnlock()

	ret := []string{}
	for key, _ := range gi.Games {
		ret = append(ret, key)
	}
	return ret
}

func (gi *GameImages) GetMetadata(appid string) []Metadata {
	images := []Metadata{}

	theGame, ok := gi.Games[appid]
	if !ok {
		fmt.Printf("[GetMetadata] Unable to find game with appid %s\n", appid)
		return nil
	}

	gi.lock.RLock()
	defer gi.lock.RUnlock()
	for _, meta := range theGame {
		images = append(images, Metadata{
			Src:    fmt.Sprintf("/img/%s/%s", appid, meta.Name),
			Width:  meta.Width,
			Height: meta.Height,
		})
	}

	return images
}

func gameIdFromPath(path string) string {
	return filepath.Base(filepath.Clean(filepath.Join(filepath.Dir(path), "..")))
}

// Save an image and produce a thumbnail
// fullpath is expected to be the full path of the full
// image, not the thumbnail.
func saveImage(fullpath string, raw []byte) error {
	dir := filepath.Dir(fullpath)

	// TODO: config the file perms?
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return fmt.Errorf("Error creating directory for image: %s", err)
	}

	err = os.MkdirAll(filepath.Join(dir, "thumbnails"), 0777)
	if err != nil {
		return fmt.Errorf("Error creating directory for image thumbnail: %s", err)
	}

	err = os.WriteFile(fullpath, raw, 0777)
	if err != nil {
		return fmt.Errorf("Unable to write full image: %s", err)
	}

	rawReader := bytes.NewReader(raw)

	// We need the whole image here.
	imgRaw, err := jpeg.Decode(rawReader)
	if err != nil {
		return fmt.Errorf("Error decoding JPEG: %s", err)
	}

	thumb := resize.Thumbnail(200, 200, imgRaw, resize.NearestNeighbor)
	thumbFile, err := os.Create(filepath.Join(dir, "thumbnails", filepath.Base(fullpath)))
	if err != nil {
		return fmt.Errorf("Error opening thumbnail file for writing: %s", err)
	}
	defer thumbFile.Close()

	err = jpeg.Encode(thumbFile, thumb, &jpeg.Options{Quality: 75})
	if err != nil {
		return fmt.Errorf("Error writing JPEG thumbnail: %s", err)
	}
	return nil
}

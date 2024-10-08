package steamscreenshots

import (
	"os"
	"fmt"
	"sync"
	"time"
	"image"
	"image/jpeg"
	_ "image/png"
	"path/filepath"
	"errors"
	"encoding/json"
	"slices"
	"strings"

	"golang.org/x/image/draw"
)

const (
	ThumbWidth  int = 200
	//ThumbHeight int = 112
)

type GameImages struct {
	Games   map[string]map[string]*ImageMeta // appid & filename base
	Updated time.Time
	Root    string
	lock    *sync.RWMutex

	isDirty bool
	filename string
}

type ImageMeta struct {
	Width  int
	Height int
	ModTime time.Time
}

// Used in TemplateData
type Metadata struct {
	Src    string `json:"src"`
	Width  int    `json:"w"`
	Height int    `json:"h"`
}

func LoadImageCache(filename, rootDir string) (*GameImages, error) {
	_, err := os.Stat(filename)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		gi := NewGameImages()
		gi.filename = filename
		gi.Root = rootDir
		return gi, nil
	}

	rawfile, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	gi := NewGameImages()
	err = json.Unmarshal(rawfile, gi)
	if err != nil {
		return nil, fmt.Errorf("Unable to unmarshal: %w", err)
	}

	gi.filename = filename
	gi.Root = rootDir
	return gi, nil
}

func NewGameImages() *GameImages {
	return &GameImages{
		Games: make(map[string]map[string]*ImageMeta),
		lock:  &sync.RWMutex{},
	}
}

var supportedImageFormats []string = []string{
	".jpeg",
	".jpg",
	".png",
}

func (s *Server) imageAdder() {
	for {
		img := <- s.newImages
		meta, err := s.ImageCache.AddImage(img.AppId, img.Filename)
		if err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Printf("adding image [%s] %s\n", img.AppId, img.Filename)
		s.ImageCache.lock.Lock()
		if _, ok := s.ImageCache.Games[img.AppId]; !ok {
			s.ImageCache.Games[img.AppId] = make(map[string]*ImageMeta)
		}
		s.ImageCache.Games[img.AppId][img.Filename] = meta
		s.ImageCache.lock.Unlock()
	}
}

func (gi *GameImages) AddImage(appid, filename string) (*ImageMeta, error) {
	fname := filename
	dname := appid

	fmt.Printf("AddImage(%q, %q)\n", appid, filename)

	if !slices.Contains(supportedImageFormats, filepath.Ext(fname)) {
		fmt.Println("Unsupported image format:", filepath.Ext(fname))
		return nil, nil
	}

	imgFile, err := os.Open(filepath.Join(gi.Root, dname, fname))
	if err != nil {
		return nil, err
	}

	cfg, _, err := image.DecodeConfig(imgFile)
	imgFile.Close()
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(filepath.Join(gi.Root, dname, fname))
	if err != nil {
		return nil, err
	}

	meta := &ImageMeta{
		Width:   cfg.Width,
		Height:  cfg.Height,
		ModTime: info.ModTime(),
	}

	// make sure thumbnail exists
	// TODO: make sure this has a .jpg extension
	_, err = os.Stat(filepath.Join(gi.Root, dname, "thumbnails", fname))
	if err == nil {
		return meta, nil
	}

	imgFile, err = os.Open(filepath.Join(gi.Root, dname, fname))
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(imgFile)
	imgFile.Close()
	if err != nil {
		return nil, err
	}

	ratio := float64(img.Bounds().Max.Y) / float64(img.Bounds().Max.X)
	height := int(float64(ThumbWidth) * ratio)
	thumbImg := image.NewRGBA(image.Rect(0, 0, ThumbWidth, height))
	draw.ApproxBiLinear.Scale(thumbImg, thumbImg.Bounds(), img, img.Bounds(), draw.Over, nil)

	err = os.MkdirAll(filepath.Join(gi.Root, dname, "thumbnails"), 0755)
	if err != nil {
		return nil, err
	}

	// TODO: make sure this has a .jpg extension
	thumbFile, err := os.Create(filepath.Join(gi.Root, dname, "thumbnails", fname))
	if err != nil {
		return nil, err
	}

	err = jpeg.Encode(thumbFile, thumbImg, nil)
	thumbFile.Close()
	if err != nil {
		return nil, err
	}

	return meta, nil
}

func (gi *GameImages) Scan() error {
	fmt.Println("starting scan of", gi.Root)
	start := time.Now()
	defer func() { fmt.Println("finished scan in", time.Since(start)) }()

	dirs, err := os.ReadDir(gi.Root)
	if err != nil {
		return err
	}

	foundGames := make(map[string]any)

	// Range over the game directories
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		dname := dir.Name()
		foundGames[dname] = nil
		//fmt.Println(dname)
		files, err := os.ReadDir(filepath.Join(gi.Root, dname))
		if err != nil {
			return fmt.Errorf("error reading %s: %w", filepath.Join(gi.Root, dname), err)
		}

		dmap := make(map[string]*ImageMeta)

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			meta, err := gi.AddImage(dname, file.Name())
			if err != nil {
				return err
			}

			if meta != nil {
				dmap[file.Name()] = meta
			}
		}

		gi.lock.Lock()

		// TODO: delete thumbnail files for images that no longer exist?
		gi.Games[dname] = dmap
		gi.Updated = time.Now()

		gi.lock.Unlock()
	}

	if gi.filename == "" {
		return nil
	}

	gi.lock.Lock()
	defer gi.lock.Unlock()

	for game, _ := range gi.Games {
		if _, exists := foundGames[game]; !exists {
			fmt.Println("game id", game, "no longer exists")
			delete(gi.Games, game)
		}
	}

	cachefile, err := os.Create(gi.filename)
	if err != nil {
		return err
	}
	defer cachefile.Close()

	enc := json.NewEncoder(cachefile)
	enc.SetIndent("", "\t")
	return enc.Encode(gi)
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
	for filename, meta := range theGame {
		images = append(images, Metadata{
			// FIXME: oh god why
			Src:    fmt.Sprintf("/img/%s/%s", appid, filename),
			Width:  meta.Width,
			Height: meta.Height,
		})
	}

	slices.SortFunc(images, func(a, b Metadata) int {
		return strings.Compare(a.Src, b.Src)
	})

	return images
}

// Number of games
func (gi *GameImages) Length() int {
	gi.lock.Lock()
	defer gi.lock.Unlock()
	return len(gi.Games)
}

// Number of images for a given AppId
func (gi *GameImages) Count(appid string) int {
	gi.lock.Lock()
	defer gi.lock.Unlock()

	return len(gi.Games[appid])
}

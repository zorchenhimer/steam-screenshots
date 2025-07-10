package steamscreenshots

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

func (s *Server) handler_game(w http.ResponseWriter, r *http.Request) {
	appid := r.PathValue("appid")

	if _, exists := s.ImageCache.Games[appid]; !exists {
		http.NotFound(w, r)
		return
	}

	imageMeta := s.ImageCache.GetMetadata(appid)

	files := []string{}
	for _, m := range imageMeta {
		files = append(files, m.Src)
	}

	sort.Strings(files)
	pretty, err := s.getGameName(appid)
	if err != nil {
		fmt.Printf("Error getting name for %s: %s\n", appid, err)
	}

	d := TemplateData{}
	d.Title = pretty
	d.Header = map[string]string{
		"Text":  pretty,
		"Count": fmt.Sprintf("%d", len(files)),
	}
	d.Body = []map[string]template.JS{}
	d.ImageMetadata = imageMeta

	for idx, filename := range files {
		base := filepath.Base(filename)
		clearclass := ""
		if idx%3 == 0 {
			clearclass = " clearme"
			//fmt.Printf("Clearme on %q\n", base)
		}

		d.Body = append(d.Body, map[string]template.JS{
			"ImageTarget":  template.JS("/img/" + appid + "/" + base),
			"ThumbnailSrc": template.JS("/thumb/" + appid + "/" + base),
			"Text":         template.JS(base),
			"Clear":        template.JS(clearclass),
			"Idx":          template.JS(fmt.Sprintf("%d", idx)),
		})
	}

	err = renderTemplate(w, "list", &d)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) handler_main(w http.ResponseWriter, r *http.Request) {
	keys := s.ImageCache.GetGames()
	gameNames := map[string]string{}

	d := TemplateData{}
	d.Body = []map[string]template.JS{}
	for _, k := range keys {
		pretty, err := s.getGameName(k)
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
		if idx%3 == 0 {
			clearclass = " clearme"
		}
		appid := gameNames[pretty]
		d.Body = append(d.Body, map[string]template.JS{
			"Target": template.JS(appid + "/"),
			"Pretty": template.JS(pretty),
			"Count":  template.JS(fmt.Sprintf("%d", s.ImageCache.Count(appid))),
			"Clear":  template.JS(clearclass),
		})
	}

	err := renderTemplate(w, "main", &d)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) handler_thumb(w http.ResponseWriter, r *http.Request) {
	appid    := r.PathValue("appid")
	filename := r.PathValue("filename")

	if _, exists := s.ImageCache.Games[appid][filename]; !exists {
		http.NotFound(w, r)
		return
	}

	fullPath := filepath.Join(
		s.settings.ImageDirectory,
		appid,
		"thumbnails",
		filename,
	)

	http.ServeFile(w, r, fullPath)
}

func (s *Server) handler_image(w http.ResponseWriter, r *http.Request) {
	appid    := r.PathValue("appid")
	filename := r.PathValue("filename")

	if filename == "banner.jpg" {
		if _, exists := s.ImageCache.Games[appid]; exists {
			bannerpath, err := s.getGameBanner(appid)
			if err != nil {
				http.ServeFileFS(w, r, s.StaticFiles, "banners/unknown.jpg")
				return
			}
			http.ServeFile(w, r, bannerpath)
			return
		} else {
			fmt.Println("appid", appid, "doesn't exist")
			http.NotFound(w, r)
			return
		}
	}

	if _, exists := s.ImageCache.Games[appid][filename]; !exists {
		http.NotFound(w, r)
		return
	}

	fullPath := filepath.Join(
		s.settings.ImageDirectory,
		appid,
		filename,
	)

	http.ServeFile(w, r, fullPath)
}

func (s *Server) handler_static(w http.ResponseWriter, r *http.Request) {
	subdir   := r.PathValue("subdir")
	filename := r.PathValue("filename")

	var relpath string
	if subdir != "" {
		relpath = filepath.Join("static", subdir, filename)
	} else {
		relpath = filepath.Join("static", filename)
	}

	http.ServeFileFS(w, r, s.StaticFiles, relpath)
}

func (s *Server) handler_debug(w http.ResponseWriter, r *http.Request) {
	d := TemplateData{}
	d.Body = []map[string]template.JS{}

	if len(gitCommit) == 0 {
		gitCommit = "Missing commit hash"
	}

	if len(version) == 0 {
		version = "Missing version info"
	}

	tmp := []string{
		fmt.Sprintf("Last scan: %s", time.Since(s.lastScan)),
		fmt.Sprintf("Uptime: %s", time.Since(s.startTime)),
		fmt.Sprintf("Game cache count: %d", s.Games.Length()),
		fmt.Sprintf("Game count: %d", s.ImageCache.Length()),
		fmt.Sprintf("Version: %s", version),
		fmt.Sprintf("Commit: %s", gitCommit),
	}

	for _, s := range tmp {
		d.Body = append(d.Body, map[string]template.JS{
			"Data": template.JS(s),
		})
	}

	err := renderTemplate(w, "debug", &d)
	if err != nil {
		fmt.Println(err)
	}
}

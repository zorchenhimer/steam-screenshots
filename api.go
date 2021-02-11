package steamscreenshots

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
)

func (s *Server) handler_api_cache(w http.ResponseWriter, r *http.Request) {
	if !s.checkApiKey(w, r) {
		return
	}
	fmt.Println("serving image.cache")

	raw, err := json.Marshal(s.ImageCache)
	if err != nil {
		fmt.Println(err)

		sendApiError(w, ApiError{
			Code:    http.StatusInternalServerError,
			Message: "JSON Marshal error",
		})
		return
	}

	w.WriteHeader(200)
	w.Write(raw)
}

func (s *Server) handler_api_games(w http.ResponseWriter, r *http.Request) {
	if !s.checkApiKey(w, r) {
		return
	}

	fmt.Println("serving games.cache")
	http.ServeFile(w, r, "games.cache")
}

func (s *Server) handler_api_addImage(w http.ResponseWriter, r *http.Request) {
	if !s.checkApiKey(w, r) {
		return
	}

	//fmt.Printf("Request:\n%v\n\n", r)
	//fmt.Println("method:", r.Method)

	if r.Method != "POST" {
		sendApiError(w, ApiError{
			Code:    http.StatusBadRequest,
			Message: "Non-POST request",
		})
	}

	if err := r.ParseForm(); err != nil {
		fmt.Println(err)
		sendApiError(w, ApiError{
			Code:    http.StatusBadRequest,
			Message: "ParseForm() error",
		})
	}

	gameId := r.Header.Get("game-id")
	imgName := r.Header.Get("filename")

	if gameId == "" {
		sendApiError(w, ApiError{
			Code:    http.StatusBadRequest,
			Message: "Missing game-id",
		})
	}

	if imgName == "" {
		sendApiError(w, ApiError{
			Code:    http.StatusBadRequest,
			Message: "Missing filename",
		})
	}

	rawImage, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("Error reading data: %s\n", err)
		sendApiError(w, ApiError{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Error reading image raw data: %s", err),
		})
	}

	fmt.Printf("data size: %d\n", len(rawImage))
	if len(rawImage) == 0 {
		fmt.Println("Zero-length image!")
		return
	}

	fullpath := filepath.Join(s.settings.RemoteDirectory, gameId, "screenshots", imgName)

	meta, err := readRawImage(rawImage)
	if err != nil {
		fmt.Printf("Error reading raw image: %s\n", err)
		return
	}

	err = saveImage(fullpath, rawImage)
	if err != nil {
		fmt.Printf("Error saving image: %s\n", err)
		return
	}

	meta.Name = imgName

	// Add image to cache
	s.ImageCache.addImageMeta(gameId, *meta)
	s.ImageCache.Save("image.cache")
}

func (s *Server) removeImages(w http.ResponseWriter, r *http.Request) {
	if !s.checkApiKey(w, r) {
		return
	}
}

// checkApiKey returns True if the key is valid
func (s *Server) checkApiKey(w http.ResponseWriter, r *http.Request) bool {
	key := r.Header.Get("api-key")
	if key != s.settings.ApiKey {
		fmt.Printf("invalid or missing api key: %q\n", key)
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}
	return true
}

type ApiError struct {
	Code    int
	Message string
}

func sendApiError(w http.ResponseWriter, errmsg ApiError) {
	encoded, err := json.Marshal(errmsg)
	if err != nil {
		panic(err)
	}

	w.WriteHeader(errmsg.Code)
	w.Write(encoded)
}

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

	http.ServeFile(w, r, "image.cache")
}

func (s *Server) handler_api_games(w http.ResponseWriter, r *http.Request) {
	if !s.checkApiKey(w, r) {
		return
	}

	http.ServeFile(w, r, "games.cache")
}

func (s *Server) handler_api_addImage(w http.ResponseWriter, r *http.Request) {
	if !s.checkApiKey(w, r) {
		return
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
		sendApiError(w, ApiError{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Error reading image raw data: %s", err),
		})
	}

	//fmt.Printf("image %q read sucessfully\n", imgName)
	fullpath := filepath.Join(s.settings.RemoteDirectory, gameId, "screenshots", imgName)
	//fmt.Printf("Saving image to %q\n", fullpath)

	err = saveImage(fullpath, rawImage)
	if err != nil {
		fmt.Printf("Error saving image: %s\n", err)
		return
	}

	meta, err := readRawImage(rawImage)
	if err != nil {
		fmt.Printf("Error reading raw image: %s\n", err)
		return
	}

	meta.Name = imgName

	// Add image to cache
	s.ImageCache.addImageMeta(gameId, *meta)
	s.ImageCache.Save("image.cache")
	//fmt.Println(s.ImageCache)
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

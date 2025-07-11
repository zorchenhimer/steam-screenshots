package steamscreenshots

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (s *Server) handler_api_cache(w http.ResponseWriter, r *http.Request) {
	if !s.checkApiKey(w, r) {
		return
	}
	fmt.Println("serving image.cache")

	raw, err := json.Marshal(s.ImageCache.Games)
	if err != nil {
		fmt.Println(err)

		sendApiError(w, ApiError{
			Code:    http.StatusInternalServerError,
			Message: "JSON Marshal error",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(raw)
}

func (s *Server) handler_api_upload(w http.ResponseWriter, r *http.Request) {
	if !s.checkApiKey(w, r) {
		return
	}

	appid    := r.PathValue("appid")
	filename := r.PathValue("filename")
	if appid == "" || filename == "" {
		fmt.Println("appid or filename missing")
		sendApiError(w, ApiError{
			Code: http.StatusBadRequest,
			Message: "appid or filename missing",
		})
		return
	}

	fullname := filepath.Join(s.settings.ImageDirectory, appid, filename)
	err := os.MkdirAll(filepath.Dir(fullname), 0755)
	if err != nil {
		fmt.Println(err)
		sendApiError(w, ApiError{
			Code: http.StatusBadRequest,
			Message: fmt.Sprintf("unable to create appid folder %s: %s", appid, err.Error()),
		})
		return
	}

	output, err := os.Create(fullname)
	if err != nil {
		fmt.Println(err)
		sendApiError(w, ApiError{
			Code: http.StatusBadRequest,
			Message: fmt.Sprintf("unable to create image file: %s", err.Error()),
		})
		return
	}
	defer output.Close()

	_, err = io.Copy(output, r.Body)
	if err != nil {
		fmt.Println(err)
		sendApiError(w, ApiError{
			Code: http.StatusBadRequest,
			Message: fmt.Sprintf("unable to read image: %s", err.Error()),
		})
		output.Close()
		err = os.Remove(fullname)
		if err != nil {
			fmt.Println("unable to remove incomplete file %s: %s", fullname, err.Error())
		}
		return
	}

	fmt.Printf("[%s] %s uploaded\n", appid, filename)
	s.newImages <- NewImage{AppId: appid, Filename: filename}
}

// checkApiKey returns True if the key is valid
func (s *Server) checkApiKey(w http.ResponseWriter, r *http.Request) bool {
	if s.settings.ApiWhitelist == nil || len(s.settings.ApiWhitelist) == 0 {
		fmt.Println("No IP addresses in API Whitelist")
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}

	found := false
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		fmt.Printf("Error splitting host and port for %q: %s\n", r.RemoteAddr, err)
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}

	// Check if the host is directly in the whitelist
	if slices.Contains(s.settings.ApiWhitelist, host) {
		found = true
	}

	// Check if any whitelist entry is a hostname that resolves to the request IP
	if !found {
		for _, entry := range s.settings.ApiWhitelist {
			// Skip if entry looks like an IP address
			if net.ParseIP(entry) != nil {
				continue
			}

			// Try to resolve the hostname
			addrs, err := net.LookupHost(entry)
			if err != nil {
				continue
			}

			// Check if any resolved IP matches the request host
			if slices.Contains(addrs, host) {
				found = true
				break
			}
		}
	}


	if !found && host == "127.0.0.1" {
		realIp := r.Header.Get("X-Real-Ip")
		if realIp != ""  && slices.Contains(s.settings.ApiWhitelist, realIp) {
			found = true
		}
	}

	if !found {
		fmt.Printf("IP/hostname %q not in API whitelist\n", host)
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}

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

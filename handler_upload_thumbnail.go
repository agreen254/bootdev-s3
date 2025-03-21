package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

var allowedMIMES = []string{"image/jpeg", "image/png"}

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to extract data", err)
		return
	}

	thumb, thumbHeaders, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to extract data", err)
		return
	}

	mediaType, _, err := mime.ParseMediaType(thumbHeaders.Header.Get("Content-Type"))
	if !slices.Contains(allowedMIMES, mediaType) {
		respondWithError(w, http.StatusBadRequest, "Invalid MIME type", err)
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "An unexpected error occurred", err)
		return
	}

	tmp := strings.Split(mediaType, "/")
	ext := tmp[len(tmp)-1]

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to locate video", err)
		return
	}
	if userID != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "You are not the video owner", nil)
		return
	}

	assetPath := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%s.%s", videoIDString, ext))
	fullPath := fmt.Sprintf("http://localhost:%s/%s", cfg.port, assetPath)
	file, err := os.Create(assetPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create file", err)
		return
	}
	defer file.Close()

	_, err = io.Copy(file, thumb)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create file", err)
		return
	}

	video.ThumbnailURL = &fullPath
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

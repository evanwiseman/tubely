package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

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

	// Parse form data
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	// Get image data from the form
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()
	mediaType := header.Header.Get("Content-Type")

	// Save thumbnail to a file in assets
	fileExtension := strings.ReplaceAll(mediaType, "image/", "")
	fp := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%v.%v", videoID, fileExtension))
	fd, err := os.Create(fp)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
	}
	io.Copy(fd, file)

	thumbnailURL := fmt.Sprintf(
		"http://localhost:%v/assets/%v.%v",
		cfg.port, videoID, fileExtension,
	)

	// Get the video metadata & validate the user
	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Image metadata not found", err)
		return
	}
	if metadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unable to authorize user", err)
		return
	}

	video := database.Video{
		ID:           metadata.ID,
		CreatedAt:    metadata.CreatedAt,
		UpdatedAt:    time.Now(),
		ThumbnailURL: &thumbnailURL,
		VideoURL:     metadata.VideoURL,
		CreateVideoParams: database.CreateVideoParams{
			Title:       metadata.Title,
			Description: metadata.Description,
			UserID:      metadata.UserID,
		},
	}
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Unable to find video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, video)
}

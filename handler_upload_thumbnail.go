package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
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
	// Set max upload memory 1MB
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	// Get the videoID
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	// Get the JWT Token
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	// Validate the JWT Token
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)
	// Get the video metadata & validate the user
	metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Image metadata not found", err)
		return
	}
	if metadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User not authorized", err)
		return
	}

	// Parse thumbnail
	thumbnailFile, thumbnailHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer thumbnailFile.Close()

	// Get the media type and validate it's jpeg or png
	mediaType, _, err := mime.ParseMediaType(thumbnailHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid media type", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "unsupported media type", errors.New("unsupported type"))
		return
	}

	// Get the file extension
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		respondWithError(w, http.StatusBadRequest, "invalid media type", err)
	}
	fileExtension := parts[1]

	// Save thumbnail to a file in assets
	bytes := make([]byte, 32)
	_, err = rand.Read(bytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to fill bytes", err)
		return
	}

	// Create the file at name
	key := base64.RawURLEncoding.EncodeToString(bytes)
	name := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%v.%v", key, fileExtension))
	assetsFile, err := os.Create(name)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}

	// Copy the file
	if _, err = io.Copy(assetsFile, thumbnailFile); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to copy file", err)
		return
	}

	// Update the video with the new thumbnailURL
	thumbnailURL := fmt.Sprintf("http://localhost:%v/%v", cfg.port, name)
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
	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusNotFound, "Unable to update video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, video)
}

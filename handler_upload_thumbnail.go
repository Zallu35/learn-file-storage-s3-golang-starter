package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
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

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)
	fileData, headerData, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer fileData.Close()
	mType := headerData.Header.Get("Content-Type")
	if mType == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for thumbnail", nil)
		return
	}
	imageData, err := io.ReadAll(fileData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error reading data", err)
		return
	}
	vidMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching video", err)
		return
	}
	if userID != vidMetadata.UserID {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	encodedImage := base64.StdEncoding.EncodeToString(imageData)
	dataURL := "data:" + mType + ";base64," + encodedImage
	vidMetadata.ThumbnailURL = &dataURL

	err = cfg.db.UpdateVideo(vidMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, vidMetadata)
}

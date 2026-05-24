package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	mType, _, err := mime.ParseMediaType(headerData.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type", nil)
		return
	}
	if mType != "image/jpeg" && mType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid file format, must be jpeg or png", nil)
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
	rawName := make([]byte, 32)
	rand.Read(rawName)
	fileName := base64.RawURLEncoding.EncodeToString(rawName)
	fileTypeSplit := strings.Split(mType, "/")
	vidWithExt := fileName + "." + fileTypeSplit[1]
	fPath := filepath.Join(cfg.assetsRoot, vidWithExt)
	thumbFile, err := os.Create(fPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating file", err)
	}
	io.Copy(thumbFile, fileData)
	locURL := "http://localhost:8091/assets/" + vidWithExt
	vidMetadata.ThumbnailURL = &locURL

	err = cfg.db.UpdateVideo(vidMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, vidMetadata)
}

package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)
	defer r.Body.Close()

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
	fmt.Println("uploading video", videoID, "by user", userID)
	vidMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching video", err)
		return
	}
	if userID != vidMetadata.UserID {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	fileData, headerData, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer fileData.Close()
	mType, _, err := mime.ParseMediaType(headerData.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Improper Content-Type", nil)
		return
	}
	if mType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type, must be mp4", nil)
		return
	}

	tmpData, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create file", err)
		return
	}
	defer os.Remove(tmpData.Name())
	defer tmpData.Close()

	_, err = io.Copy(tmpData, fileData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error transferring video data", err)
		return
	}

	_, err = tmpData.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error resetting offset", err)
	}

	rawName := make([]byte, 32)
	_, err = rand.Read(rawName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating name", err)
		return
	}
	fileName := base64.RawURLEncoding.EncodeToString(rawName)
	objectInput := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileName,
		Body:        tmpData,
		ContentType: &mType,
	}
	_, err = cfg.s3Client.PutObject(r.Context(), &objectInput)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading", err)
		return
	}

	s3URL := "https://" + cfg.s3Bucket + ".s3." + cfg.s3Region + ".amazonaws.com/" + fileName
	vidMetadata.VideoURL = &s3URL
	err = cfg.db.UpdateVideo(vidMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, vidMetadata)
}

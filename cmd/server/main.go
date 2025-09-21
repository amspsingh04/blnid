package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool
var UploadDir = "./uploads"

func main() {
	// Connect to Postgres
	dsn := "postgres://bnid_user:bnid_pass@localhost:5432/bnid?sslmode=disable"
	var err error
	Pool, err = pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer Pool.Close()

	err = Pool.Ping(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to Postgres successfully!")

	// Ensure upload directory exists
	if _, err := os.Stat(UploadDir); os.IsNotExist(err) {
		os.Mkdir(UploadDir, 0755)
	}

	// Gin router
	r := gin.Default()

	// Single file upload
	r.POST("/upload", handleUpload)

	// List files
	r.GET("/files", listFiles)

	// Run server
	r.Run(":8080")
}

func handleUpload(c *gin.Context) {
	userID := 1 // For demo, we use test user

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File not provided"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot open file"})
		return
	}
	defer src.Close()

	// Compute SHA-256 hash
	hash := sha256.New()
	tee := io.TeeReader(src, hash)
	tmpFile := filepath.Join(UploadDir, file.Filename)
	out, err := os.Create(tmpFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot save file"})
		return
	}
	defer out.Close()
	io.Copy(out, tee)
	fileHash := fmt.Sprintf("%x", hash.Sum(nil))

	// Check if file already exists in DB
	var existingID int
	err = Pool.QueryRow(context.Background(), "SELECT id FROM files WHERE hash=$1", fileHash).Scan(&existingID)
	if err == nil {
		// File exists, remove duplicate saved copy
		os.Remove(tmpFile)
		c.JSON(http.StatusOK, gin.H{"message": "File already exists", "file_id": existingID})
		return
	}

	// Insert metadata into DB
	_, err = Pool.Exec(context.Background(), `
		INSERT INTO files (owner_id, filename, mime_type, size, hash)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, file.Filename, file.Header.Get("Content-Type"), file.Size, fileHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot insert file metadata"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully", "hash": fileHash})
}

func listFiles(c *gin.Context) {
	userID := 1 // demo user

	rows, err := Pool.Query(context.Background(), "SELECT id, filename, mime_type, size, upload_date FROM files WHERE owner_id=$1", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot fetch files"})
		return
	}
	defer rows.Close()

	files := []gin.H{}
	for rows.Next() {
		var id int
		var filename, mimeType string
		var size int64
		var uploadDate string
		rows.Scan(&id, &filename, &mimeType, &size, &uploadDate)
		files = append(files, gin.H{
			"id":          id,
			"filename":    filename,
			"mime_type":   mimeType,
			"size":        size,
			"upload_date": uploadDate,
		})
	}

	c.JSON(http.StatusOK, files)
}

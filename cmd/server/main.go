package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

// Config
const (
	UploadsDir    = "./uploads"
	ObjectsSubdir = "objects" // uploads/objects/<sha256>
	ListenAddr    = ":8080"
	// demo user (replace with real auth in future)
	DemoUserID = 1
)

type FileRecord struct {
	ID         int64  `json:"id"`
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	MimeType   string `json:"mime_type"`
	UploadDate string `json:"upload_date"`
	Hash       string `json:"hash"`
	OwnerID    int64  `json:"owner_id"`
}

func main() {
	// DB connect
	dsn := "postgres://bnid_user:password@localhost:5432/bnid?sslmode=disable"
	var err error
	Pool, err = pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatal("db connect:", err)
	}
	defer Pool.Close()

	if err = Pool.Ping(context.Background()); err != nil {
		log.Fatal("db ping:", err)
	}
	log.Println("Connected to Postgres successfully!")

	// ensure upload dirs
	if err := os.MkdirAll(filepath.Join(UploadsDir, ObjectsSubdir), 0o755); err != nil {
		log.Fatal("cannot create uploads dir:", err)
	}

	// Gin router
	r := gin.Default()

	// CORS for local dev
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Routes
	r.POST("/upload", uploadHandler)
	r.GET("/files", listFilesHandler)
	r.GET("/files/:id/download", downloadHandler)
	r.DELETE("/files/:id", deleteHandler)

	log.Printf("Listening and serving HTTP on %s\n", ListenAddr)
	if err := r.Run(ListenAddr); err != nil {
		log.Fatal(err)
	}
}

// uploadHandler handles a single multipart/form-data file field named "file".
// It supports deduplication by saving object once to uploads/objects/<sha256>.
func uploadHandler(c *gin.Context) {
	// demo auth: fixed user
	userID := DemoUserID

	// Read form file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required (form field 'file')"})
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot open uploaded file"})
		return
	}
	defer src.Close()

	// Read first 512 bytes for MIME sniffing, and also compute hash while saving to temp
	hasher := sha256.New()
	tee := io.TeeReader(src, hasher)

	// Write to a temporary file first
	tmpPath := filepath.Join(UploadsDir, fmt.Sprintf("tmp-%d-%s", time.Now().UnixNano(), filepath.Base(fileHeader.Filename)))
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create temp file"})
		return
	}

	// We need the first 512 bytes for DetectContentType. Read into buffer while copying.
	buf := make([]byte, 512)
	n, _ := io.ReadFull(tee, buf)
	if n < 0 {
		n = 0
	}

	// copy the first chunk to tmpFile
	if _, err := tmpFile.Write(buf[:n]); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot write temp file"})
		return
	}

	// copy the rest
	if _, err := io.Copy(tmpFile, tee); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot write temp file (copy)"})
		return
	}

	// finalize
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot close temp file"})
		return
	}

	fileHash := hex.EncodeToString(hasher.Sum(nil))
	objectPath := filepath.Join(UploadsDir, ObjectsSubdir, fileHash)

	// MIME detection
	detectedMime := http.DetectContentType(buf[:n])

	// Check dedupe: does object already exist on disk or as DB entry?
	// Check if object file exists on disk
	if _, err := os.Stat(objectPath); os.IsNotExist(err) {
		// object not present on disk: move temp to object path
		if err := os.Rename(tmpPath, objectPath); err != nil {
			// fallback: copy
			in, err := os.Open(tmpPath)
			if err != nil {
				os.Remove(tmpPath)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot place object file"})
				return
			}
			out, err := os.Create(objectPath)
			if err != nil {
				in.Close()
				os.Remove(tmpPath)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create object file"})
				return
			}
			if _, err := io.Copy(out, in); err != nil {
				in.Close()
				out.Close()
				os.Remove(tmpPath)
				os.Remove(objectPath)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot copy object file"})
				return
			}
			in.Close()
			out.Close()
			os.Remove(tmpPath)
		}
	} else {
		// object exists, remove temp copy
		os.Remove(tmpPath)
	}

	// Insert metadata row in files table
	var insertedID int64
	err = Pool.QueryRow(context.Background(), `
		INSERT INTO files (owner_id, filename, mime_type, size, hash, upload_date, is_public, download_count)
		VALUES ($1, $2, $3, $4, $5, now(), false, 0) RETURNING id
	`, userID, fileHeader.Filename, detectedMime, fileHeader.Size, fileHash).Scan(&insertedID)
	if err != nil {
		// If unique constraints etc cause error, respond accordingly
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot insert file metadata", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File uploaded successfully",
		"id":      insertedID,
		"hash":    fileHash,
		"mime":    detectedMime,
		"size":    fileHeader.Size,
	})
}

// listFilesHandler returns files for demo user
// listFilesHandler returns files for demo user
func listFilesHandler(c *gin.Context) {
	userID := DemoUserID

	rows, err := Pool.Query(context.Background(), `
		SELECT id, filename, mime_type, size,
		       COALESCE(to_char(upload_date, 'YYYY-MM-DD"T"HH24:MI:SS'), '') as upload_date,
		       hash
		FROM files WHERE owner_id = $1 ORDER BY upload_date DESC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot query files", "details": err.Error()})
		return
	}
	defer rows.Close()

	var list []FileRecord
	for rows.Next() {
		var f FileRecord
		if err := rows.Scan(&f.ID, &f.Filename, &f.MimeType, &f.Size, &f.UploadDate, &f.Hash); err != nil {
			continue
		}
		list = append(list, f)
	}

	// Always return a JSON array (never null)
	if list == nil {
		list = []FileRecord{}
	}

	c.JSON(http.StatusOK, list)
}

// deleteHandler deletes a file record and removes object if last reference
func deleteHandler(c *gin.Context) {
	userID := DemoUserID
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// verify owner and get hash
	var owner int64
	var hash string
	err = Pool.QueryRow(context.Background(), `SELECT owner_id, hash FROM files WHERE id=$1`, id).Scan(&owner, &hash)
	if err != nil {
		// If not found, return 204 so frontend refreshes cleanly
		c.JSON(http.StatusNoContent, gin.H{})
		return
	}

	if owner != int64(userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner can delete"})
		return
	}

	// delete file record
	_, err = Pool.Exec(context.Background(), `DELETE FROM files WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot delete file record"})
		return
	}

	// check remaining references
	var count int
	err = Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM files WHERE hash=$1`, hash).Scan(&count)
	if err == nil && count == 0 {
		// remove object file
		objectPath := filepath.Join(UploadsDir, ObjectsSubdir, hash)
		_ = os.Remove(objectPath)
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// downloadHandler serves the deduplicated object file for a given file ID
func downloadHandler(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var filename, hash, mime string
	row := Pool.QueryRow(context.Background(), `SELECT filename, hash, mime_type FROM files WHERE id=$1`, id)
	if err := row.Scan(&filename, &hash, &mime); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	objectPath := filepath.Join(UploadsDir, ObjectsSubdir, hash)
	finfo, err := os.Stat(objectPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "object not found on server"})
		return
	}

	// increment download_count (best-effort, ignore errors)
	_, _ = Pool.Exec(context.Background(), `UPDATE files SET download_count = download_count + 1 WHERE id=$1`, id)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Content-Length", fmt.Sprintf("%d", finfo.Size()))
	if mime != "" {
		c.Header("Content-Type", mime)
	} else {
		c.Header("Content-Type", "application/octet-stream")
	}

	c.File(objectPath)
}

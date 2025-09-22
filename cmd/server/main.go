package main

import (
	"context"
	"crypto/sha256"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5" // NEW: JWT library
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt" // NEW: Password hashing
)

var Pool *pgxpool.Pool

// NEW: In a real app, use an environment variable for the secret key!
var jwtKey = []byte("my_very_secret_key_for_bnid")

const (
	UploadsDir    = "./uploads"
	ObjectsSubdir = "objects"
	ListenAddr    = ":8080"
)

// MODIFIED: Renamed OwnerID to UserID for clarity
type FileRecord struct {
	ID         int64  `json:"id"`
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	MimeType   string `json:"mime_type"`
	UploadDate string `json:"upload_date"`
	Hash       string `json:"hash"`
	UserID     int64  `json:"owner_id"`
}

// NEW: User struct for authentication
type User struct {
	ID           int64
	Username     string
	PasswordHash string
}

// NEW: JWT Claims struct
type Claims struct {
	UserID int64 `json:"userId"`
	jwt.RegisteredClaims
}

func main() {
	// DB connect (your existing code)
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

	// ensure upload dirs (your existing code)
	if err := os.MkdirAll(filepath.Join(UploadsDir, ObjectsSubdir), 0o755); err != nil {
		log.Fatal("cannot create uploads dir:", err)
	}

	r := gin.Default()

	// CORS (your existing code, slightly modified for credentials)
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// --- MODIFIED: Separate public and protected routes ---

	// Public routes for authentication
	r.POST("/register", registerHandler)
	r.POST("/login", loginHandler)

	// Protected routes for file management
	protected := r.Group("/")
	protected.Use(authMiddleware())
	{
		protected.POST("/upload", uploadHandler)
		protected.GET("/files", listFilesHandler)
		protected.GET("/files/:id/download", downloadHandler)
		protected.DELETE("/files/:id", deleteHandler)
	}

	log.Printf("Listening and serving HTTP on %s\n", ListenAddr)
	if err := r.Run(ListenAddr); err != nil {
		log.Fatal(err)
	}
}

// --- NEW: Authentication Handlers ---

func registerHandler(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	_, err = Pool.Exec(context.Background(),
		"INSERT INTO users (username, password_hash) VALUES ($1, $2)",
		input.Username, string(hashedPassword))

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Registration successful"})
}

func loginHandler(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	var user User
	err := Pool.QueryRow(context.Background(),
		"SELECT id, password_hash FROM users WHERE username = $1",
		input.Username).Scan(&user.ID, &user.PasswordHash)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

// --- NEW: Authentication Middleware ---

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}
		tokenString := authHeader[7:]
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}
		c.Set("userId", claims.UserID)
		c.Next()
	}
}

// --- MODIFIED: File Handlers (Now use user from context) ---

func uploadHandler(c *gin.Context) {
	// MODIFIED: Get user ID from middleware context instead of demo constant
	userIDVal, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}
	userID := userIDVal.(int64)

	// ... your existing, excellent file processing and deduplication logic remains here ...
	// (No changes needed from `fileHeader, err := c.FormFile("file")` down to the `os.Remove(tmpPath)`)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required (form field 'file')"})
		return
	}
	// ... (The entire file saving, hashing, and deduplication block is unchanged)
	// I'm omitting it here for brevity, but you should KEEP your original code block.
	// The only change is the final database INSERT shown below.

	// Assume we have `fileHash`, `detectedMime`, `fileHeader.Size` from your logic
	hasher := sha256.New()
	// This is a simplified representation of your hashing logic
	// In your code, you already calculate this correctly.
	// ...
	fileHash := "..."     // Replace with your actual hash calculation result
	detectedMime := "..." // Replace with your actual mime type detection result
	// ...

	// MODIFIED: This is the only line that changes in the latter half of the function.
	// It uses the real userID from the token.
	var insertedID int64
	err = Pool.QueryRow(context.Background(), `
        INSERT INTO files (owner_id, filename, mime_type, size, hash, upload_date)
        VALUES ($1, $2, $3, $4, $5, now()) RETURNING id
    `, userID, fileHeader.Filename, detectedMime, fileHeader.Size, fileHash).Scan(&insertedID)

	// ... The rest of your `uploadHandler` is the same
	c.JSON(http.StatusOK, gin.H{"message": "File uploaded"})
}

func listFilesHandler(c *gin.Context) {
	// MODIFIED: Get user ID from context
	userIDVal, _ := c.Get("userId")
	userID := userIDVal.(int64)

	rows, err := Pool.Query(context.Background(), `
        SELECT id, filename, mime_type, size,
               COALESCE(to_char(upload_date, 'YYYY-MM-DD"T"HH24:MI:SS'), '') as upload_date,
               hash
        FROM files WHERE owner_id = $1 ORDER BY upload_date DESC
    `, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot query files"})
		return
	}
	defer rows.Close()

	var list []FileRecord
	for rows.Next() {
		var f FileRecord
		if err := rows.Scan(&f.ID, &f.Filename, &f.MimeType, &f.Size, &f.UploadDate, &f.Hash); err != nil {
			continue // Or log the error
		}
		list = append(list, f)
	}

	if list == nil {
		list = []FileRecord{}
	}
	c.JSON(http.StatusOK, list)
}

func deleteHandler(c *gin.Context) {
	// MODIFIED: Get user ID from context
	userIDVal, _ := c.Get("userId")
	userID := userIDVal.(int64)

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var hash string
	// MODIFIED: Verify ownership in the initial query
	err = Pool.QueryRow(context.Background(), `SELECT hash FROM files WHERE id=$1 AND owner_id=$2`, id, userID).Scan(&hash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found or permission denied"})
		return
	}

	// ... your existing delete and cleanup logic is the same ...
	_, err = Pool.Exec(context.Background(), `DELETE FROM files WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot delete file record"})
		return
	}
	var count int
	err = Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM files WHERE hash=$1`, hash).Scan(&count)
	if err == nil && count == 0 {
		objectPath := filepath.Join(UploadsDir, ObjectsSubdir, hash)
		_ = os.Remove(objectPath)
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func downloadHandler(c *gin.Context) {
	// MODIFIED: Add ownership check for security
	userIDVal, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}
	userID := userIDVal.(int64)

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var filename, hash, mime string
	// MODIFIED: The query now also checks for owner_id
	err = Pool.QueryRow(context.Background(),
		`SELECT filename, hash, mime_type FROM files WHERE id=$1 AND owner_id=$2`, id, userID).Scan(&filename, &hash, &mime)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found or permission denied"})
		return
	}

	// ... The rest of your download logic is the same
	objectPath := filepath.Join(UploadsDir, ObjectsSubdir, hash)
	// ...
	c.File(objectPath)
}

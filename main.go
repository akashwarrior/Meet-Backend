package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"crypto/sha256"
	"io"

	"github.com/square/go-jose/v3"
	"golang.org/x/crypto/hkdf"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var managers = make(map[string]*UserManager)
var managerMutex = &sync.Mutex{}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	InitDB()

	http.HandleFunc("/", handleWebSocket)

	log.Printf("Server running on :%d", 8080)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	meetingID := r.URL.Query().Get("meetingId")
	name := r.URL.Query().Get("name")

	if meetingID == "" {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	hostID, err := FindMeetingByID(meetingID)
	if err != nil {
		log.Println("Invalid meeting ID:", meetingID)
		http.Error(w, "Invalid Room ID", http.StatusNotFound)
		return
	}

	var userID string
	if cookies, err := r.Cookie("next-auth.session-token"); err == nil {
		if token := cookies.Value; token != "" {
			if decryptedID, err := decryptJWE(&token); err == nil {
				userID = *decryptedID
			}
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}

	socket := &SocketWrapper{
		Conn: conn,
		ID:   userID,
		Name: name,
	}

	manager, exists := managers[meetingID]
	if !exists {
		managerMutex.Lock()
		manager = NewUserManager(hostID)
		managers[meetingID] = manager
		managerMutex.Unlock()
	}

	manager.AddUser(socket)
}

func decryptJWE(token *string) (*string, error) {
	secret := os.Getenv("NEXTAUTH_SECRET")
	if secret == "" {
		log.Fatal("NEXTAUTH_SECRET environment variable is not set")
	}
	derivedKey, err := deriveEncryptionKey(secret, "")
	if err != nil {
		log.Fatalf("Failed to derive encryption key: %v", err)
	}

	// Parse JWE token
	object, err := jose.ParseEncrypted(*token)
	if err != nil {
		log.Fatalf("Failed to parse JWE: %v", err)
	}

	// Decrypt payload
	plaintext, err := object.Decrypt(derivedKey)
	if err != nil {
		log.Fatalf("Failed to decrypt JWE: %v", err)
	}

	var payload map[string]interface{}
	err = json.Unmarshal(plaintext, &payload)
	if err != nil {
		log.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	ID := payload["sub"].(string)
	return &ID, nil
}

func deriveEncryptionKey(secret, salt string) ([]byte, error) {
	info := "NextAuth.js Generated Encryption Key"
	if salt != "" {
		info += fmt.Sprintf(" (%s)", salt)
	}

	hkdf := hkdf.New(sha256.New, []byte(secret), []byte(salt), []byte(info))
	key := make([]byte, 32)

	if _, err := io.ReadFull(hkdf, key); err != nil {
		return nil, err
	}

	return key, nil
}

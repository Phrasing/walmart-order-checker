package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
)

func main() {
	fmt.Println("=== Generate Secure Keys ===\n")

	// Generate SESSION_KEY
	sessionKey := make([]byte, 32)
	if _, err := rand.Read(sessionKey); err != nil {
		log.Fatal(err)
	}
	sessionKeyEncoded := base64.URLEncoding.EncodeToString(sessionKey)
	fmt.Println("SESSION_KEY:")
	fmt.Println(sessionKeyEncoded)
	fmt.Println()

	// Generate ENCRYPTION_KEY
	encryptionKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		log.Fatal(err)
	}
	encryptionKeyEncoded := base64.StdEncoding.EncodeToString(encryptionKey)
	fmt.Println("ENCRYPTION_KEY:")
	fmt.Println(encryptionKeyEncoded)
	fmt.Println()

	fmt.Println("Copy these values to your .env file:")
	fmt.Printf("\nSESSION_KEY=%s\n", sessionKeyEncoded)
	fmt.Printf("ENCRYPTION_KEY=%s\n", encryptionKeyEncoded)
}

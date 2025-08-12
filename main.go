package main

import (
	"fmt"
	"hash/crc32"
	"net/http"
	"net/url"
	"sync"
)

func main() {
	http.HandleFunc("/shorten", shortenHandler)
	http.HandleFunc("/", redirectHandler)

	fmt.Println("URL Shortener server starting on :8080")
	fmt.Println("POST /shorten with 'url' parameter to shorten a URL")
	fmt.Println("GET /<shorturl> to redirect to original URL")
	
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
	}
}

// Takes in a URL and returns a shortened one
var urlStore = make(map[string]string)
var reverseStore = make(map[string]string)
var mu sync.RWMutex

func GenerateURL(ref string) (string, error) {
	// Make sure that the URL isn't empty and that it is a valid URL
	_, err := url.Parse(ref)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Check if URL already exists
	mu.RLock()
	if shortURL, exists := reverseStore[ref]; exists {
		mu.RUnlock()
		return shortURL, nil
	}
	mu.RUnlock()

	// Logic for hash function and base62 encoding
	URLbytes := []byte(ref)
	hashedURL := crc32.ChecksumIEEE(URLbytes)

	shortURL, err := Base62Encoder(hashedURL)
	if err != nil {
		return "", err
	}

	// Store the mapping
	mu.Lock()
	urlStore[shortURL] = ref
	reverseStore[ref] = shortURL
	mu.Unlock()

	return shortURL, nil
}

func Base62Encoder(hashedURL uint32) (string, error) {
	base62Chars := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	
	if hashedURL == 0 {
		return "0", nil
	}
	
	var result []byte
	num := hashedURL
	
	for num > 0 {
		result = append([]byte{base62Chars[num%62]}, result...)
		num = num / 62
	}
	
	return string(result), nil
}

func GetOriginalURL(shortURL string) (string, error) {
	mu.RLock()
	defer mu.RUnlock()
	
	if originalURL, exists := urlStore[shortURL]; exists {
		return originalURL, nil
	}
	
	return "", fmt.Errorf("short URL not found")
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	originalURL := r.FormValue("url")
	if originalURL == "" {
		http.Error(w, "URL parameter is required", http.StatusBadRequest)
		return
	}

	shortURL, err := GenerateURL(originalURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error generating short URL: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"short_url": "%s", "original_url": "%s"}`, shortURL, originalURL)
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	shortURL := r.URL.Path[1:]
	if shortURL == "" {
		http.Error(w, "Short URL is required", http.StatusBadRequest)
		return
	}

	originalURL, err := GetOriginalURL(shortURL)
	if err != nil {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	http.Redirect(w, r, originalURL, http.StatusMovedPermanently)
}

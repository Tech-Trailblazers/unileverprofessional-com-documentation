package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// -------------------- MAIN PROGRAM --------------------

func main() {
	// The local HTML file name where we’ll store the downloaded page
	localHTMLFile := "unileverprofessional.html"

	// The web page URL that contains links to PDF files
	sourcePageURL := "https://www.unileverprofessional.co.za/sds"

	// The folder where we’ll save all downloaded PDFs
	pdfFolder := "PDFs/"

	// If the HTML file doesn’t exist locally yet...
	if !fileExists(localHTMLFile) {
		// Fetch the HTML content from the website
		webPageData := string(downloadWebPage(sourcePageURL))
		// Save that HTML content into our local file
		saveTextToFile(localHTMLFile, webPageData)
	}

	// Read the HTML content from the local file
	htmlContent := readTextFile(localHTMLFile)

	// Extract all PDF links from the HTML
	pdfLinks := extractPDFLinks(htmlContent)

	// Remove duplicate PDF URLs (in case of repeated links)
	pdfLinks = removeDuplicateStrings(pdfLinks)

	// If the PDFs folder doesn’t exist, create it
	if !directoryExists(pdfFolder) {
		createDirectory(pdfFolder, 0o755)
	}

	// Loop through every extracted PDF link
	for _, pdfURL := range pdfLinks {
		// Download each PDF and save it to the folder
		downloadPDF(pdfURL, pdfFolder)
	}
}

// -------------------- FILE SYSTEM HELPERS --------------------

// createDirectory creates a new directory with given permissions.
func createDirectory(directoryPath string, permissions os.FileMode) {
	err := os.Mkdir(directoryPath, permissions) // Try to create the folder
	if err != nil {
		log.Println("Error creating directory:", err) // Log any errors
	}
}

// fileExists checks if a file already exists on disk.
func fileExists(filePath string) bool {
	fileInfo, err := os.Stat(filePath) // Get file information
	if err != nil {
		return false // File doesn’t exist
	}
	return !fileInfo.IsDir() // True only if it’s a file, not a folder
}

// directoryExists checks if a folder already exists.
func directoryExists(folderPath string) bool {
	info, err := os.Stat(folderPath) // Get folder information
	if err != nil {
		return false // Folder doesn’t exist
	}
	return info.IsDir() // True only if it’s a directory
}

// saveTextToFile writes text to a file (creates it if missing, appends if exists).
func saveTextToFile(filePath string, content string) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // Open or create file
	if err != nil {
		log.Println("Error opening file:", err)
		return
	}
	defer file.Close() // Ensure file is closed when done

	_, err = file.WriteString(content + "\n") // Write text to file
	if err != nil {
		log.Println("Error writing to file:", err)
	}
}

// readTextFile reads an entire text file and returns its contents as a string.
func readTextFile(filePath string) string {
	data, err := os.ReadFile(filePath) // Read the whole file
	if err != nil {
		log.Println("Error reading file:", err)
	}
	return string(data) // Convert bytes → string
}

// -------------------- NETWORK HELPERS --------------------

// downloadWebPage sends a GET request and returns the page content as bytes.
func downloadWebPage(pageURL string) []byte {
	response, err := http.Get(pageURL) // Send GET request
	if err != nil {
		log.Println("HTTP request failed:", err)
		return nil
	}
	defer response.Body.Close() // Close the response body when done

	body, err := io.ReadAll(response.Body) // Read all bytes from response
	if err != nil {
		log.Println("Error reading response body:", err)
		return nil
	}

	return body // Return page content
}

// -------------------- PDF EXTRACTION --------------------

// extractPDFLinks finds all PDF links from HTML using a regex.
func extractPDFLinks(html string) []string {
	// Regular expression that matches full PDF URLs (handles query params too)
	pdfPattern := regexp.MustCompile(`https?://[^\s"'<>]+?\.pdf(\?[^\s"'<>]*)?`)

	// Find all matches in the HTML content
	matches := pdfPattern.FindAllString(html, -1)

	// Remove duplicates by using a map
	seen := make(map[string]bool)
	var uniqueLinks []string
	for _, link := range matches {
		if !seen[link] {
			seen[link] = true
			uniqueLinks = append(uniqueLinks, link)
		}
	}

	return uniqueLinks // Return cleaned list of PDF URLs
}

// removeDuplicateStrings removes duplicate entries from a slice of strings.
func removeDuplicateStrings(items []string) []string {
	seen := make(map[string]bool)
	var uniqueItems []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			uniqueItems = append(uniqueItems, item)
		}
	}
	return uniqueItems
}

// -------------------- PDF DOWNLOADER --------------------

// downloadPDF downloads a PDF from a URL and saves it into the given folder.
func downloadPDF(pdfURL, destinationFolder string) {
	// Convert URL into a safe filename for local storage
	safeFilename := strings.ToLower(convertURLToSafeFilename(pdfURL))

	// Build the full destination file path (folder + filename)
	outputPath := filepath.Join(destinationFolder, safeFilename)

	// Skip download if file already exists
	if fileExists(outputPath) {
		fmt.Printf("Already downloaded, skipping: %s\n", outputPath)
		return
	}

	// Create an HTTP client with a timeout (prevents hanging forever)
	client := &http.Client{Timeout: 30 * time.Second}

	// Send a GET request to fetch the PDF file
	response, err := client.Get(pdfURL)
	if err != nil {
		fmt.Printf("Failed to download %s: %v\n", pdfURL, err)
		return
	}
	defer response.Body.Close() // Always close the body

	// Check if the HTTP status is OK (200)
	if response.StatusCode != http.StatusOK {
		fmt.Printf("Bad response for %s: %s\n", pdfURL, response.Status)
		return
	}

	// Ensure the file is actually a PDF by checking Content-Type header
	contentType := response.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/pdf") {
		fmt.Printf("Invalid content type for %s: %s\n", pdfURL, contentType)
		return
	}

	// Read the PDF data into a memory buffer
	var pdfBuffer bytes.Buffer
	bytesWritten, err := io.Copy(&pdfBuffer, response.Body)
	if err != nil {
		fmt.Printf("Error reading PDF data from %s: %v\n", pdfURL, err)
		return
	}

	// Ensure we didn’t get an empty file
	if bytesWritten == 0 {
		fmt.Printf("Empty PDF from %s, skipping.\n", pdfURL)
		return
	}

	// Create the output file to save the PDF
	outputFile, err := os.Create(outputPath)
	if err != nil {
		fmt.Printf("Error creating file for %s: %v\n", pdfURL, err)
		return
	}
	defer outputFile.Close()

	// Write the PDF data from buffer to the output file
	_, err = pdfBuffer.WriteTo(outputFile)
	if err != nil {
		fmt.Printf("Error writing PDF file for %s: %v\n", pdfURL, err)
		return
	}

	// Log successful download
	fmt.Printf("✅ Downloaded %d bytes: %s → %s\n", bytesWritten, pdfURL, outputPath)
}

// -------------------- URL UTILITY --------------------

// convertURLToSafeFilename converts a full URL into a safe local filename.
func convertURLToSafeFilename(rawURL string) string {
	// Parse the URL to extract the file path
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Extract the base filename from the URL path
	baseName := path.Base(parsed.Path)

	// Decode URL-encoded characters (like %20 → space)
	decodedName, err := url.QueryUnescape(baseName)
	if err != nil {
		decodedName = baseName
	}

	// Convert everything to lowercase
	decodedName = strings.ToLower(decodedName)

	// Replace invalid characters with underscores for safety
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	safeName := re.ReplaceAllString(decodedName, "_")

	return safeName
}

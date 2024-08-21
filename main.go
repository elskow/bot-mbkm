package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func loadEnvVariables() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	bearerToken = os.Getenv("BEARER_TOKEN")
	discordWebhook = os.Getenv("DISCORD_WEBHOOK")
	lastResponse = make(map[int]string)
}

var (
	apiURL         = "https://api.kampusmerdeka.kemdikbud.go.id/mbkm/mahasiswa/activities/my"
	bearerToken    string
	discordWebhook string
	lastResponse   map[int]string
	firstRun       = true
)

func fetchData() (string, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func sendDiscordNotification(message, imageURL string) error {
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       "New Activity Update",
				"description": message,
				"image": map[string]string{
					"url": imageURL,
				},
			},
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(discordWebhook, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to send notification, status code: %d", resp.StatusCode)
	}
	return nil
}

func formatNotification(data string) (string, string, error) {
	var responseData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &responseData); err != nil {
		return "", "", err
	}

	activities, ok := responseData["data"].([]interface{})
	if !ok {
		return "", "", fmt.Errorf("unexpected data format: %T", responseData["data"])
	}

	var message, imageURL string
	for _, activity := range activities {
		act, ok := activity.(map[string]interface{})
		if !ok {
			continue
		}

		id := int(act["id"].(float64))
		status := act["status"].(string)

		if status == "PROCESSED" {
			continue
		}

		if lastStatus, exists := lastResponse[id]; !exists || lastStatus != status {
			message += fmt.Sprintf("**Activity:** %s\n", act["nama_kegiatan"])
			message += fmt.Sprintf("**Partner:** %s\n", act["mitra_brand_name"])
			message += fmt.Sprintf("**Status:** %s\n", status)
			imageURL = act["mitra_logo"].(string)
			lastResponse[id] = status
		}
	}

	return message, imageURL, nil
}

func checkForChanges() {
	if firstRun {
		if err := sendDiscordNotification("Your Bot is UP!", "https://example.com/default-image.png"); err != nil {
			log.Println("Error sending initial notification:", err)
		} else {
			log.Println("Initial notification sent!")
		}
		firstRun = false
	}

	data, err := fetchData()
	if err != nil {
		log.Println("Error fetching data:", err)
		return
	}

	message, imageURL, err := formatNotification(data)
	if err != nil {
		log.Println("Error formatting notification:", err)
		return
	}

	if message != "" {
		if err := sendDiscordNotification(message, imageURL); err != nil {
			log.Println("Error sending notification:", err)
		} else {
			log.Println("Notification sent!")
		}
	}
}

func main() {
	loadEnvVariables()
	for {
		checkForChanges()
		time.Sleep(1 * time.Minute)
	}
}

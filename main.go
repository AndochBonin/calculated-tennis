package main

import (
	"encoding/json"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
)

var (
	apiKey     string
	keyAddress string
	clobUrl    string
	gammaUrl   string
)

func makeRequest(method, url string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("RELAYER_API_KEY", apiKey)
	req.Header.Set("RELAYER_API_KEY_ADDRESS", keyAddress)

	client := &http.Client{}
	return client.Do(req)
}

func main() {
	err := godotenv.Load()

	if err != nil {
		log.Println("Error loading .env")
	}

	apiKey = os.Getenv("RELAYER_API_KEY")
	keyAddress = os.Getenv("RELAYER_API_KEY_ADDRESS")
	gammaUrl = os.Getenv("GAMMA_BASE_URL")

	if apiKey == "" || keyAddress == "" || gammaUrl == "" {
		log.Fatal("Missing environment variables")
	}

	resp, err := makeRequest("GET", gammaUrl+"/markets")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		log.Fatal("Did not get a successful response:", resp.StatusCode)
	}
	
	var markets []Market

	if err := json.NewDecoder(resp.Body).Decode(&markets); err != nil {
		log.Fatal(err)
	}

	// 2. Print first active market
	for _, m := range markets {
		if m.Active {
			log.Println("Hello, Polymarket!")
			log.Println("Market:", m.Question)
			log.Println("Market ID:", m.ID)
			break
		}
	}

	// 3. Placeholder for order logic
	log.Println("\nNext step: implement signed order placement")
}

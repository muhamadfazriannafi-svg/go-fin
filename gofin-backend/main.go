package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// --- STRUKTUR DATA ---

type TelegramWebhook struct {
	Message struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text    string `json:"text"`
		Caption string `json:"caption"`
		Photo   []struct {
			FileID string `json:"file_id"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"photo"`
	} `json:"message"`
}

type AIExtractedData struct {
	Amount      float64 `json:"amount"`
	Type        string  `json:"type"`     // 'income' atau 'expense'
	Category    string  `json:"category"`
	Description string  `json:"description"`
}

type Transaction struct {
	UserID      string  `json:"user_id"`
	Type        string  `json:"type"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
}

func main() {
	godotenv.Load()
	r := gin.Default()

	r.POST("/webhook/telegram", handleTelegramWebhook)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	log.Printf("🚀 GoFin Backend berjalan di port %s", port)
	r.Run(":" + port)
}

func handleTelegramWebhook(c *gin.Context) {
	var req TelegramWebhook
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userText := req.Message.Text
	chatID := req.Message.Chat.ID

	// 1. Ambil input: foto struk atau teks
	var extractedData Transaction
	var err error
	if len(req.Message.Photo) > 0 {
		fileID := req.Message.Photo[len(req.Message.Photo)-1].FileID
		img, derr := downloadTelegramPhoto(fileID)
		if derr != nil {
			log.Println("Error download foto:", derr)
			sendTelegramMessage(chatID, "Gagal mengunduh foto. 😢")
			c.Status(http.StatusOK)
			return
		}
		extractedData, err = parseImageWithAI(img, req.Message.Caption)
	} else if userText != "" {
		extractedData, err = parseTextWithAI(userText)
	} else {
		c.Status(http.StatusOK)
		return
	}

	if err != nil {
		log.Println("Error parsing AI:", err)
		sendTelegramMessage(chatID, "Maaf, GoFin tidak mengerti format pesannya. 😢")
		c.Status(http.StatusOK)
		return
	}

	if extractedData.Amount == 0 && extractedData.Type == "" {
		sendTelegramMessage(chatID, "GoFin tidak menemukan transaksi di pesan itu. 🤔")
		c.Status(http.StatusOK)
		return
	}

	// 2. Simpan hasil parsing ke Supabase
	txData := Transaction{
		UserID:      fmt.Sprintf("%d", chatID),
		Type:        extractedData.Type,
		Amount:      extractedData.Amount,
		Category:    extractedData.Category,
		Description: extractedData.Description,
	}

	err = saveToSupabase(txData)
	if err != nil {
		log.Println("Error simpan DB:", err)
		sendTelegramMessage(chatID, "Gagal menyimpan ke database. 🛠️")
		c.Status(http.StatusOK)
		return
	}

	// 3. Balas ke pengguna via Telegram
	replyMsg := fmt.Sprintf("✅ *Tercatat!*\nRp %.0f\nKategori: %s\nKet: %s", 
		extractedData.Amount, extractedData.Category, extractedData.Description)
	sendTelegramMessage(chatID, replyMsg)

	c.Status(http.StatusOK)
}

const systemPrompt = "Kamu adalah asisten sistem. HANYA kembalikan JSON murni tanpa markdown (tanpa backtick ```json)."

// parseTextWithAI memparsing transaksi dari teks bebas
func parseTextWithAI(userText string) (Transaction, error) {
	prompt := `Ekstrak kalimat berikut ke dalam format JSON dengan key: amount (angka integer), category (string), type ("expense" atau "income"), dan description (string). Kalimat: ` + userText
	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": prompt},
	}
	return callGroq("openai/gpt-oss-20b", messages)
}

// parseImageWithAI memparsing transaksi dari foto struk via model vision Groq
func parseImageWithAI(image []byte, caption string) (Transaction, error) {
	prompt := `Ekstrak transaksi dari foto struk/nota ini ke JSON dengan key: amount (angka integer), category (string), type ("expense" atau "income"), dan description (string).`
	if caption != "" {
		prompt += " Konteks tambahan dari pengguna: " + caption
	}
	dataURI := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(image)
	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": []map[string]interface{}{
			{"type": "text", "text": prompt},
			{"type": "image_url", "image_url": map[string]string{"url": dataURI}},
		}},
	}
	return callGroq("qwen/qwen3.8-27b", messages)
}

// callGroq memanggil chat completions Groq dan mengembalikan Transaction terparsing
func callGroq(model string, messages []map[string]interface{}) (Transaction, error) {
	apiKey := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
	if apiKey == "" {
		log.Println("❌ ERROR: GROQ_API_KEY belum diset di file .env")
		return Transaction{}, fmt.Errorf("api key kosong")
	}

	requestBody := map[string]interface{}{
		"model":           model,
		"messages":        messages,
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.1,
	}

	jsonReq, _ := json.Marshal(requestBody)
	httpReq, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonReq))
	if err != nil {
		return Transaction{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Transaction{}, err
	}
	defer resp.Body.Close()

	var groqResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&groqResp)

	if errObj, exists := groqResp["error"]; exists {
		log.Printf("❌ ERROR DARI GROQ: %v", errObj)
		return Transaction{}, fmt.Errorf("akses groq ditolak")
	}

	choicesVal, exists := groqResp["choices"]
	if !exists || choicesVal == nil {
		return Transaction{}, fmt.Errorf("jawaban kosong dari groq")
	}
	choices := choicesVal.([]interface{})
	if len(choices) == 0 {
		return Transaction{}, fmt.Errorf("jawaban groq kosong")
	}

	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	aiText := message["content"].(string)

	var tx Transaction
	if err := json.Unmarshal([]byte(aiText), &tx); err != nil {
		log.Printf("❌ GAGAL PARSING JSON: %v | Teks: %s", err, aiText)
		return Transaction{}, err
	}
	return tx, nil
}

// downloadTelegramPhoto mengunduh byte gambar via Bot API getFile
func downloadTelegramPhoto(fileID string) ([]byte, error) {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN kosong")
	}

	resp, err := http.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", token, fileID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var meta struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, err
	}
	if !meta.OK || meta.Result.FilePath == "" {
		return nil, fmt.Errorf("getFile gagal")
	}

	fileResp, err := http.Get(fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, meta.Result.FilePath))
	if err != nil {
		return nil, err
	}
	defer fileResp.Body.Close()
	return io.ReadAll(fileResp.Body)
}

// Fungsi menyimpan ke Supabase via REST
func saveToSupabase(data Transaction) error {
	url := os.Getenv("SUPABASE_URL") + "/rest/v1/transactions"
	key := os.Getenv("SUPABASE_SERVICE_KEY")

	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("apikey", key)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Supabase error status: %d", resp.StatusCode)
	}
	return nil
}

// Fungsi mengirim pesan balik ke Telegram
func sendTelegramMessage(chatID int64, text string) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	jsonData, _ := json.Marshal(payload)
	http.Post(url, "application/json", bytes.NewBuffer(jsonData))
}
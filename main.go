package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	Username string `json:"username"`
	Limit    int    `json:"limit"`
}

type Recommendation struct {
	Artist string
	Track  string
}

const MAX_QUEUE = 500

var (
	appConfig    Config
	lastRecs     []Recommendation
	queue        = make(chan Recommendation, MAX_QUEUE)
	OutputDir    = "/media/HDD/navidrome/musics"
	tmpl         = template.Must(template.ParseFiles("templates/index.html"))
	clients      = make(map[chan string]bool)
	clientsMutex sync.Mutex
)

func broadcast(msg string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for client := range clients {
		client <- msg
	}
}

func init() {
	loadConfig()

	go func() {
		for rec := range queue {
			log.Printf("⬇️ [Fila] Iniciando download: %s - %s", rec.Artist, rec.Track)

			broadcast(fmt.Sprintf(`<div class="text-blue-400 border-l-2 border-blue-500 pl-2 mb-2 animate-pulse">A transferir: %s - %s...</div>`, rec.Artist, rec.Track))

			err := downloadWithYtdlp(rec.Artist, rec.Track)

			if err != nil {
				log.Printf("❌ [Fila Erro] %s - %s: %v", rec.Artist, rec.Track, err)
				broadcast(fmt.Sprintf(`<div class="text-red-500 border-l-2 border-red-500 pl-2 mb-2">Erro: %s - %s</div>`, rec.Artist, rec.Track))
			} else {
				log.Printf("✅ [Fila Sucesso] Baixado: %s - %s", rec.Artist, rec.Track)
				broadcast(fmt.Sprintf(`<div class="text-emerald-500 border-l-2 border-emerald-500 pl-2 mb-2">Sucesso: %s - %s</div>`, rec.Artist, rec.Track))
			}
		}
	}()

	go startWeeklyCron()
}

func startWeeklyCron() {
	ticker := time.NewTicker(7 * 24 * time.Hour)
	for range ticker.C {
		if appConfig.Username == "" {
			continue
		}
		log.Println("[CRON] Iniciando rotina semanal de descobertas...")
		recs, err := fetchRecommendations()
		if err == nil {
			for _, r := range recs {
				queue <- r
			}
		}
	}
}

// Configs

func loadConfig() {
	file, err := os.ReadFile("config.json")
	if err == nil {
		json.Unmarshal(file, &appConfig)
	}
	if appConfig.Limit <= 0 {
		appConfig.Limit = 10
	}
}

func saveConfig() {
	data, _ := json.MarshalIndent(appConfig, "", "  ")
	os.WriteFile("config.json", data, 0644)
}

// Handles e web-server

func main() {
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/settings", handleSettings)
	http.HandleFunc("/sync", handleSync)
	http.HandleFunc("/download", handleDownload)
	http.HandleFunc("/download-all", handleDownloadAll)

	// Nova rota para as notificações em tempo real
	http.HandleFunc("/events", handleEvents)

	log.Println("Servidor lastfm-miner a correr em http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming não suportado!", http.StatusInternalServerError)
		return
	}

	messageChan := make(chan string)
	clientsMutex.Lock()
	clients[messageChan] = true
	clientsMutex.Unlock()

	defer func() {
		clientsMutex.Lock()
		delete(clients, messageChan)
		close(messageChan)
		clientsMutex.Unlock()
	}()

	for {
		select {
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "index.html", appConfig)
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	appConfig.Username = r.FormValue("username")
	limitStr := r.FormValue("limit")

	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		appConfig.Limit = l
	}

	saveConfig()
	log.Printf("⚙️ Configurações atualizadas: User=%s, Limite=%d", appConfig.Username, appConfig.Limit)
	w.Write([]byte("✓ Configurações e limite salvos com sucesso!"))
}

func handleSync(w http.ResponseWriter, r *http.Request) {
	if appConfig.Username == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<div class='text-red-500 text-sm'>Erro: Configure o Usuário primeiro!</div>"))
		return
	}

	recs, err := fetchRecommendations()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("<div class='text-red-500 text-sm'>Erro: %v</div>", err)))
		return
	}

	lastRecs = recs // Copy to download all
	tmpl.ExecuteTemplate(w, "recs_list", recs)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	artist := r.FormValue("artist")
	track := r.FormValue("track")

	if artist != "" {
		queue <- Recommendation{Artist: artist, Track: track}
		w.Write([]byte(fmt.Sprintf("✓ '%s' adicionada à fila!", track)))
	}
}

func handleDownloadAll(w http.ResponseWriter, r *http.Request) {
	if len(lastRecs) == 0 {
		w.Write([]byte("⚠️ Nenhuma recomendação para baixar. Sincronize primeiro."))
		return
	}

	for _, rec := range lastRecs {
		queue <- rec
	}
	w.Write([]byte(fmt.Sprintf("✓ %d músicas adicionadas à fila de download!", len(lastRecs))))
}

// Miner

func fetchRecommendations() ([]Recommendation, error) {
	stationURL := fmt.Sprintf("https://www.last.fm/player/station/user/%s/recommended", appConfig.Username)
	resp, err := http.Get(stationURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	json.Unmarshal(body, &data)

	var recs []Recommendation
	if playlist, ok := data["playlist"].([]interface{}); ok {
		for _, item := range playlist {
			trackData, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			trackName, _ := trackData["name"].(string)
			var artistName string

			if artists, ok := trackData["artists"].([]interface{}); ok && len(artists) > 0 {
				if firstArtist, ok := artists[0].(map[string]interface{}); ok {
					artistName, _ = firstArtist["name"].(string)
				}
			}

			if trackName != "" && artistName != "" {
				recs = append(recs, Recommendation{Artist: artistName, Track: trackName})
			}

			if len(recs) >= appConfig.Limit {
				break
			}
		}
	}

	return recs, nil
}

// Downloader

func downloadWithYtdlp(artist, track string) error {
	artistFolder := fmt.Sprintf("%s/%s/Single_Downloads", OutputDir, artist)
	_ = os.MkdirAll(artistFolder, 0755)

	searchQuery := fmt.Sprintf("ytsearch1:%s - %s audio", artist, track)
	cmd := exec.Command("yt-dlp",
		"--extractor-args", "youtube:player_client=ios,android,mweb",
		"-x", "--audio-format", "mp3",
		"--audio-quality", "0",
		"--embed-thumbnail",
		"--add-metadata",
		"-o", fmt.Sprintf("%s/%%(title)s.%%(ext)s", artistFolder),
		searchQuery,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v | Log: %s", err, string(out))
	}
	return nil
}

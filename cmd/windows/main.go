package main

import (
	"encoding/json"
	"fmt"
	"fyne.io/systray"
	"github.com/vhalmd/nomi-whatsapp/internal/assets"
	"github.com/vhalmd/nomi-whatsapp/internal/whatsapp"
	"github.com/vhalmd/nomi-whatsapp/ui"
	waLog "go.mau.fi/whatsmeow/util/log"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
)

type API struct {
	Client whatsapp.Client
}

func main() {
	systray.Run(onReady, onExit)
}

func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func onReady() {
	go func() {
		systray.SetIcon(assets.Icon)
		systray.SetTitle("Nomi Whatsapp")
		systray.SetTooltip(fmt.Sprintf("[NOMI WHATSAPP] %s", os.Getenv("NOMI_NAME")))

		mOpenBrowser := systray.AddMenuItem("Scan QR code", "Open a browser tab with the QR code to scan")
		mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

		for {
			select {
			case <-mOpenBrowser.ClickedCh:
				_ = open("http://localhost:5555")
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()

	start()
}

func onExit() {
	// clean up here
}

func start() {
	nomiApiKey := os.Getenv("NOMI_API_KEY")
	nomiID := os.Getenv("NOMI_ID")
	nomiName := os.Getenv("NOMI_NAME")
	openAIToken := os.Getenv("OPENAI_API_KEY")

	cfg := whatsapp.Config{
		NomiAPIKey: nomiApiKey,
		NomiID:     nomiID,
		NomiName:   nomiName,
		OpenAIKey:  openAIToken,
	}

	clientLog := waLog.Stdout("CLIENT", "INFO", true)

	client := whatsapp.NewClient(cfg, clientLog)
	defer client.Whatsapp.Disconnect()
	client.Whatsapp.EnableAutoReconnect = true
	client.Whatsapp.AddEventHandler(client.EventHandler)
	api := API{Client: client}

	go api.Client.ListenQR()

	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.FS(ui.ManagementUI)))
	mux.HandleFunc("POST /connect", api.GetQR)

	err := http.ListenAndServe(":5555", mux)
	if err != nil {
		log.Fatal(err)
	}
}

func (a *API) GetQR(w http.ResponseWriter, r *http.Request) {
	var status string
	if a.Client.Whatsapp.Store.ID == nil {
		status = "disconnected"
	} else {
		status = "connected"
	}

	response := map[string]string{
		"status": status,
		"qr":     a.Client.QRCode,
	}
	data, _ := json.Marshal(response)
	w.WriteHeader(202)
	_, _ = w.Write(data)
}

package main

import (
	"context"
	"fmt"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mdp/qrterminal/v3"
	"github.com/vhalmd/nomi-whatsapp/internal/whatsapp"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"
	"os"
	"os/signal"
	"syscall"
)

func main() {
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
	client.Whatsapp.AddEventHandler(client.EventHandler)

	if client.Whatsapp.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.Whatsapp.GetQRChannel(context.Background())
		err := client.Whatsapp.Connect()
		if err != nil {
			panic(err)
		}

		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err := client.Whatsapp.Connect()
		if err != nil {
			panic(err)
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Whatsapp.Disconnect()
}

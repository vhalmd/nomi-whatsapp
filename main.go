package main

import (
	"context"
	"fmt"
	"github.com/mdp/qrterminal/v3"
	"github.com/sashabaranov/go-openai"
	"github.com/vhalmd/nomi-go-sdk"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
	_ "modernc.org/sqlite"
)

type API struct {
	NomiClient nomi.API
	Whatsapp   *whatsmeow.Client
	OpenAI     *openai.Client

	NomiID      string
	NomiAPIKey  string
	OpenAIToken string
}

func setup() API {
	nomiApiKey := os.Getenv("NOMI_API_KEY")
	nomiID := os.Getenv("NOMI_ID")
	nomiName := os.Getenv("NOMI_NAME")
	openAIToken := os.Getenv("OPENAI_TOKEN")

	nomiClient := nomi.NewClient(nomiApiKey)

	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New("sqlite", "file:store.db?_pragma=foreign_keys(1)", dbLog)
	if err != nil {
		panic(err)
	}

	osName := "[NOMI] " + nomiName
	store.DeviceProps.Os = &osName

	platformType := waCompanionReg.DeviceProps_IE
	store.DeviceProps.PlatformType = &platformType

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "INFO", true)
	if err != nil {
		panic(err)
	}

	whatsapp := whatsmeow.NewClient(deviceStore, clientLog)

	var openaiClient *openai.Client
	if openAIToken != "" {
		openaiClient = openai.NewClient(openAIToken)
	}

	return API{
		NomiClient: nomiClient,
		Whatsapp:   whatsapp,
		OpenAI:     openaiClient,

		NomiAPIKey:  nomiApiKey,
		NomiID:      nomiID,
		OpenAIToken: openAIToken,
	}
}

func (a API) EventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		content := ""
		if received := v.Message.ExtendedTextMessage.GetText(); received != "" {
			content = received
		}
		if received := v.Message.GetConversation(); received != "" {
			content = received
		}
		am := v.Message.GetAudioMessage()
		if content != "" {
			_ = a.Whatsapp.MarkRead([]types.MessageID{v.Info.ID}, time.Now(), v.Info.Chat, v.Info.Sender)

			_ = a.Whatsapp.SendChatPresence(v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
			nomiReply, err := a.NomiClient.SendMessage(a.NomiID, nomi.SendMessageBody{MessageText: content})
			if err != nil {
				slog.Error("Error sending message to nomi", "error", err)
			}

			_, err = a.Whatsapp.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
				Conversation: &nomiReply.ReplyMessage.Text,
			})
			if err != nil {
				slog.Error("Error sending nomi reply to whatsapp", "nomi_reply", nomiReply.ReplyMessage.Text, "jid", v.Info.Chat, "error", err)
			}
		}

		if am != nil {
			a.SendVoice(am, v.Info.Chat)
		}
	}
}

func (a API) SendVoice(msg *waE2E.AudioMessage, targetJID types.JID) {

	if a.OpenAI == nil {
		msg := "Hey! I can't listen to audios right now. Could you send me a text instead? Thanks!"
		_, _ = a.Whatsapp.SendMessage(context.Background(), targetJID, &waE2E.Message{Conversation: &msg})
		return
	}

	f, err := os.OpenFile("audio.mp3", os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		panic(err)
	}

	err = a.Whatsapp.DownloadToFile(msg, f)
	if err != nil {
		panic(err)
	}

	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: "audio.mp3",
	}
	resp, err := a.OpenAI.CreateTranscription(context.Background(), req)
	if err != nil {
		slog.Error("Transcription error", "error", err)
	}

	nomiReply, err := a.NomiClient.SendMessage(a.NomiID, nomi.SendMessageBody{MessageText: resp.Text})
	if err != nil {
		slog.Error("Error sending nomi reply to whatsapp", "nomi_reply", nomiReply.ReplyMessage.Text, "jid", targetJID, "error", err)
	}

	_, _ = a.Whatsapp.SendMessage(context.Background(), targetJID, &waE2E.Message{Conversation: &nomiReply.ReplyMessage.Text})
}

func main() {
	api := setup()
	api.Whatsapp.AddEventHandler(api.EventHandler)

	if api.Whatsapp.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := api.Whatsapp.GetQRChannel(context.Background())
		err := api.Whatsapp.Connect()
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
		err := api.Whatsapp.Connect()
		if err != nil {
			panic(err)
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	api.Whatsapp.Disconnect()
}

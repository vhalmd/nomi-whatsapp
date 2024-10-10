package whatsapp

import (
	"context"
	"errors"
	"fmt"
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
	"time"

	_ "modernc.org/sqlite"
)

type Client struct {
	NomiClient nomi.API
	Whatsapp   *whatsmeow.Client
	OpenAI     *openai.Client
	QRCode     string

	Logger waLog.Logger
	Config Config
}

type Config struct {
	NomiAPIKey string
	NomiID     string
	NomiName   string
	OpenAIKey  string
}

func NewClient(cfg Config, logger waLog.Logger) Client {
	nomiClient := nomi.NewClient(cfg.NomiAPIKey)

	dbLog := waLog.Noop
	container, err := sqlstore.New("sqlite", "file:store.db?_pragma=foreign_keys(1)", dbLog)
	if err != nil {
		panic(err)
	}

	osName := "[NOMI] " + cfg.NomiName
	store.DeviceProps.Os = &osName

	platformType := waCompanionReg.DeviceProps_IE
	store.DeviceProps.PlatformType = &platformType

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}

	whatsapp := whatsmeow.NewClient(deviceStore, logger)

	var openaiClient *openai.Client
	if cfg.OpenAIKey != "" {
		openaiClient = openai.NewClient(cfg.OpenAIKey)
	}

	return Client{
		NomiClient: nomiClient,
		Whatsapp:   whatsapp,
		OpenAI:     openaiClient,
		Logger:     logger,
		Config:     cfg,
	}
}

func (a *Client) EventHandler(evt interface{}) {
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

		senderNumber, _ := types.ParseJID(v.Info.SourceString())
		a.Logger.Infof("[MESSAGE RECEIVED] CONTENT='%s' AUDIO='%t' SENDER='%s'", content, am != nil, senderNumber.String())

		_ = a.Whatsapp.MarkRead([]types.MessageID{v.Info.ID}, time.Now(), v.Info.Chat, v.Info.Sender)
		_ = a.Whatsapp.SendChatPresence(v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
		if content != "" {
			nomiReply, err := a.NomiClient.SendMessage(a.Config.NomiID, nomi.SendMessageBody{MessageText: content})
			if err != nil {
				a.Logger.Errorf("Error sending message to nomi: %s", err)
			}

			_, err = a.Whatsapp.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
				Conversation: &nomiReply.ReplyMessage.Text,
			})
			if err != nil {
				a.Logger.Errorf("Error sending nomi reply to whatsapp. NOMI_REPLY='%s' JID='%s' ERROR=%s", nomiReply.ReplyMessage.Text, v.Info.Chat, err)
			}
		} else if am != nil {
			a.SendVoice(am, v.Info.Chat)
		}
	case *events.LoggedOut:
		fmt.Println("[LOGGED OUT]", a.Whatsapp.Store.ID)
		err := a.Whatsapp.Store.Delete()
		if err != nil {
			panic(err)
		}
		//a.Whatsapp.Store.DeleteAllSessions()
	}
}

func (a *Client) ListenQR() {
	if a.Whatsapp.Store.ID == nil {
		qrChan, _ := a.Whatsapp.GetQRChannel(context.Background())
		err := a.Whatsapp.Connect()
		if err != nil {
			slog.Warn("Websocket already connected. Trying to get new QR Code.")
		}

		for evt := range qrChan {
			if evt.Event == "code" {
				a.QRCode = evt.Code
			} else {
				a.QRCode = ""
			}
		}
	} else {
		err := a.Whatsapp.Connect()
		if err != nil {
			panic(err)
		}
	}
}

func (a *Client) SendErrorMessageAudio(targetJID types.JID) {
	msg := "Hey! I can't listen to audios right now. Could you send me a text instead? Thanks!"
	_, _ = a.Whatsapp.SendMessage(context.Background(), targetJID, &waE2E.Message{Conversation: &msg})
}

func (a *Client) SendVoice(msg *waE2E.AudioMessage, targetJID types.JID) {

	if a.OpenAI == nil {
		a.SendErrorMessageAudio(targetJID)
		return
	}

	f, err := os.OpenFile("audio.mp3", os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		a.SendErrorMessageAudio(targetJID)
		return
	}

	err = a.Whatsapp.DownloadToFile(msg, f)
	if err != nil {
		a.SendErrorMessageAudio(targetJID)
		return
	}

	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: "audio.mp3",
	}
	resp, err := a.OpenAI.CreateTranscription(context.Background(), req)
	if err != nil {
		a.Logger.Errorf("Transcription error: %s", err)
		a.SendErrorMessageAudio(targetJID)
		return
	}

	nomiReply, err := a.NomiClient.SendMessage(a.Config.NomiID, nomi.SendMessageBody{MessageText: resp.Text})
	if err != nil {
		if errors.Is(err, nomi.MessageLengthLimitExceeded) {
			msg := "Hey! The audio is a bit long. Could you send a shorter one? Thanks!"
			_, _ = a.Whatsapp.SendMessage(context.Background(), targetJID, &waE2E.Message{Conversation: &msg})
			return
		}
		a.Logger.Errorf("Error sending nomi reply to Nomi API: %s", err)
		a.SendErrorMessageAudio(targetJID)
		return
	}

	_, err = a.Whatsapp.SendMessage(context.Background(), targetJID, &waE2E.Message{Conversation: &nomiReply.ReplyMessage.Text})
	if err != nil {
		a.Logger.Errorf("Error sending nomi reply to whatsapp. NOMI_REPLY='%s' JID='%s' ERROR=%s", nomiReply.ReplyMessage.Text, targetJID, err)
		a.SendErrorMessageAudio(targetJID)
		return
	}
}

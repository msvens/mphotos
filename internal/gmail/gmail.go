package gmail

import (
	"encoding/base64"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	MIME_TXT  = "MIME-version: 1.0;\nContent-Type: text/plain; charset=\"UTF-8\";\n\n"
	MIME_HTML = "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
)

type GmailService struct {
	service *gmail.Service
}

func ReadOnlyScope() string {
	return gmail.GmailReadonlyScope
}

func SendScope() string {
	return gmail.GmailSendScope
}

func ComposeScope() string {
	return gmail.GmailComposeScope
}

func NewGmailService(token *oauth2.Token, config *oauth2.Config) (*GmailService, error) {
	ctx := context.Background()
	if srv, err := gmail.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx, token))); err != nil {
		return nil, err
	} else {
		return &GmailService{srv}, nil
	}
}

func (gs *GmailService) SendTextMessage(to string, subject string, body string) (bool, error) {

	emailFrom := "From: Mellowtech Photos <msvens@gmail.com> \r\n"
	emailTo := "To: " + to + "\r\n"
	subj := "Subject: " + subject + "\n"
	mime := MIME_TXT
	msg := []byte(emailFrom + emailTo + subj + mime + "\n" + body)
	fmt.Println(subj, emailTo, body)
	var message gmail.Message
	message.Raw = base64.URLEncoding.EncodeToString(msg)
	_, err := gs.service.Users.Messages.Send("me", &message).Do()
	if err != nil {
		fmt.Println("error sending email")
		return false, err
	}
	return true, nil
}

func (gs *GmailService) SendHtmlMessage(to string, subject string, body string) (bool, error) {

	emailFrom := "From: Mellowtech Photos <msvens@gmail.com> \r\n"
	emailTo := "To: " + to + "\r\n"
	subj := "Subject: " + subject + "\n"
	mime := MIME_HTML
	msg := []byte(emailFrom + emailTo + subj + mime + "\n" + body)
	var message gmail.Message
	message.Raw = base64.URLEncoding.EncodeToString(msg)
	_, err := gs.service.Users.Messages.Send("me", &message).Do()
	if err != nil {
		fmt.Println("error sending email")
		return false, err
	}
	return true, nil
}

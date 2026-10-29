package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type resendMailer struct {
	apiKey   string
	from     string
	endpoint string
	client   *http.Client
}

func (m resendMailer) sendMagicLink(ctx context.Context, recipient, magicLink string) error {
	payload := struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}{
		From:    m.from,
		To:      []string{recipient},
		Subject: "Your Tennis Connect sign-in link",
		Text:    "Continue signing in to Tennis Connect:\n\n" + magicLink + "\n\nThis link expires in 15 minutes and can be used once. If you did not request it, ignore this email.\n",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return errors.New("could not create email")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("could not create email request")
	}
	request.Header.Set("Authorization", "Bearer "+m.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := m.client.Do(request)
	if err != nil {
		return errors.New("email provider unavailable")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("email provider rejected request")
	}
	return nil
}

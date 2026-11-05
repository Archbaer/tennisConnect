package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"time"

	"tennisconnect/internal/saferead"
)

const (
	models         = "nudity-2.1,offensive-2.0,face-analysis,ocr"
	responseLimit  = 64 << 10
	referenceLimit = 200
	providerURL    = "https://api.sightengine.com/1.0/check.json"
	requestTimeout = 5 * time.Second
)

var errProvider = errors.New("moderation provider failed")

// Client calls the Sightengine check endpoint.
type Client struct {
	apiUser   string
	apiSecret string
	endpoint  string
	client    *http.Client
}

// New returns a Client for the configured credentials.
func New(apiUser, apiSecret string) *Client {
	return &Client{
		apiUser:   apiUser,
		apiSecret: apiSecret,
		endpoint:  providerURL,
		client:    newHTTPClient(),
	}
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: requestTimeout,
		// A redirect would let an intermediary replay the multipart body.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (c *Client) Moderate(ctx context.Context, image []byte) (Verdict, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("api_user", c.apiUser); err != nil {
		return Verdict{}, errProvider
	}
	if err := writer.WriteField("api_secret", c.apiSecret); err != nil {
		return Verdict{}, errProvider
	}
	if err := writer.WriteField("models", models); err != nil {
		return Verdict{}, errProvider
	}
	part, err := writer.CreateFormFile("media", "photo.jpg")
	if err != nil {
		return Verdict{}, errProvider
	}
	if _, err := part.Write(image); err != nil {
		return Verdict{}, errProvider
	}
	if err := writer.Close(); err != nil {
		return Verdict{}, errProvider
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &body)
	if err != nil {
		return Verdict{}, errProvider
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := c.client.Do(request)
	if err != nil {
		return Verdict{}, errProvider
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Verdict{}, errProvider
	}
	payload, err := saferead.ReadBounded(response.Body, responseLimit)
	if err != nil {
		return Verdict{}, err
	}
	var decoded sightengineResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return Verdict{}, errMalformed
	}
	return verdictFor(decoded)
}

// The documented success schema always contains these objects; a partial
// response must never be read as "clean".
type sightengineResponse struct {
	Status    string                `json:"status"`
	Request   sightengineRequest    `json:"request"`
	Nudity    *sightengineNudity    `json:"nudity"`
	Offensive *sightengineOffensive `json:"offensive"`
	Faces     *[]sightengineFace    `json:"faces"`
	Text      *sightengineText      `json:"text"`
}

type sightengineRequest struct {
	ID string `json:"id"`
}

type sightengineNudity struct {
	SexualActivity *float64 `json:"sexual_activity"`
	SexualDisplay  *float64 `json:"sexual_display"`
	Erotica        *float64 `json:"erotica"`
	VerySuggestive *float64 `json:"very_suggestive"`
}

type sightengineOffensive struct {
	Nazi          *float64 `json:"nazi"`
	AsianSwastika *float64 `json:"asian_swastika"`
	Confederate   *float64 `json:"confederate"`
	Supremacist   *float64 `json:"supremacist"`
	Terrorist     *float64 `json:"terrorist"`
	MiddleFinger  *float64 `json:"middle_finger"`
}

type sightengineFace struct {
	X1         *float64                   `json:"x1"`
	Y1         *float64                   `json:"y1"`
	X2         *float64                   `json:"x2"`
	Y2         *float64                   `json:"y2"`
	Attributes *sightengineFaceAttributes `json:"attributes"`
}

type sightengineFaceAttributes struct {
	Obstruction *sightengineObstruction `json:"obstruction"`
}

type sightengineObstruction struct {
	None *float64 `json:"none"`
}

type sightengineText struct {
	Content *string `json:"content"`
}

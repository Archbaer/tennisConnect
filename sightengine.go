package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"time"
)

const (
	photoModerationModels    = "nudity-2.1,offensive-2.0,face-analysis,ocr"
	moderationResponseLimit  = 64 << 10
	moderationReferenceLimit = 200
	minFaceArea              = 0.01
	minFaceVisibility        = 0.5
	moderationApproveBelow   = 0.4
	moderationRejectAt       = 0.5
	photoModerationTimeout   = 5 * time.Second
	sightengineEndpoint      = "https://api.sightengine.com/1.0/check.json"
)

var (
	errModerationProvider  = errors.New("moderation provider failed")
	errModerationMalformed = errors.New("moderation response malformed")
)

type moderationVerdict struct {
	status    photoModerationStatus
	reference string
}

type moderationFunc func(context.Context, []byte) (moderationVerdict, error)

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

type sightengineClient struct {
	apiUser   string
	apiSecret string
	endpoint  string
	client    *http.Client
}

func newSightengineHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		// A redirect would let an intermediary replay the multipart body.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (c sightengineClient) moderate(ctx context.Context, image []byte) (moderationVerdict, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("api_user", c.apiUser); err != nil {
		return moderationVerdict{}, errModerationProvider
	}
	if err := writer.WriteField("api_secret", c.apiSecret); err != nil {
		return moderationVerdict{}, errModerationProvider
	}
	if err := writer.WriteField("models", photoModerationModels); err != nil {
		return moderationVerdict{}, errModerationProvider
	}
	part, err := writer.CreateFormFile("media", "photo.jpg")
	if err != nil {
		return moderationVerdict{}, errModerationProvider
	}
	if _, err := part.Write(image); err != nil {
		return moderationVerdict{}, errModerationProvider
	}
	if err := writer.Close(); err != nil {
		return moderationVerdict{}, errModerationProvider
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &body)
	if err != nil {
		return moderationVerdict{}, errModerationProvider
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := c.client.Do(request)
	if err != nil {
		return moderationVerdict{}, errModerationProvider
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return moderationVerdict{}, errModerationProvider
	}
	payload, err := readBounded(response.Body, moderationResponseLimit)
	if err != nil {
		return moderationVerdict{}, err
	}
	var decoded sightengineResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return moderationVerdict{}, errModerationMalformed
	}
	return moderationVerdictFor(decoded)
}

// moderationVerdictFor applies the fail-closed product policy. The vendor scores
// `none`, `suggestive` and `mildly_suggestive` high for everyday swimwear and
// vests, so only the safety keys below participate in thresholds.
func moderationVerdictFor(response sightengineResponse) (moderationVerdict, error) {
	if response.Status != "success" {
		return moderationVerdict{}, errModerationMalformed
	}
	if response.Nudity == nil || response.Offensive == nil || response.Faces == nil ||
		response.Text == nil || response.Text.Content == nil {
		return moderationVerdict{}, errModerationMalformed
	}
	safetyScores := []*float64{
		response.Nudity.SexualActivity,
		response.Nudity.SexualDisplay,
		response.Nudity.Erotica,
		response.Nudity.VerySuggestive,
		response.Offensive.Nazi,
		response.Offensive.AsianSwastika,
		response.Offensive.Confederate,
		response.Offensive.Supremacist,
		response.Offensive.Terrorist,
		response.Offensive.MiddleFinger,
	}
	for _, score := range safetyScores {
		if score == nil {
			return moderationVerdict{}, errModerationMalformed
		}
	}

	verdict := moderationVerdict{reference: moderationReference(response.Request.ID)}
	for _, score := range safetyScores {
		if *score >= moderationRejectAt {
			verdict.status = photoModerationRejected
			return verdict, nil
		}
	}
	faceVisible, err := hasVisibleFace(*response.Faces)
	if err != nil {
		return moderationVerdict{}, err
	}
	if !faceVisible || containsContactDetails(*response.Text.Content) {
		verdict.status = photoModerationRejected
		return verdict, nil
	}
	for _, score := range safetyScores {
		if *score >= moderationApproveBelow {
			verdict.status = photoModerationPending
			return verdict, nil
		}
	}
	verdict.status = photoModerationApproved
	return verdict, nil
}

func hasVisibleFace(faces []sightengineFace) (bool, error) {
	for _, face := range faces {
		if face.X1 == nil || face.Y1 == nil || face.X2 == nil || face.Y2 == nil {
			return false, errModerationMalformed
		}
		width := *face.X2 - *face.X1
		height := *face.Y2 - *face.Y1
		if width <= 0 || height <= 0 || width > 1 || height > 1 {
			return false, errModerationMalformed
		}
		if width*height < minFaceArea {
			continue
		}
		if face.Attributes == nil || face.Attributes.Obstruction == nil || face.Attributes.Obstruction.None == nil {
			continue
		}
		if *face.Attributes.Obstruction.None >= minFaceVisibility {
			return true, nil
		}
	}
	return false, nil
}

func moderationReference(reference string) string {
	if len(reference) > moderationReferenceLimit {
		return reference[:moderationReferenceLimit]
	}
	return reference
}

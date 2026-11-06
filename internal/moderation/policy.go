package moderation

import (
	"errors"

	"tennisconnect/internal/textguard"
)

// Status is the moderation outcome stored against a photo.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// Verdict is a moderation decision plus the provider reference to keep for audits.
type Verdict struct {
	Status    Status
	Reference string
}

const (
	minFaceArea       = 0.01
	minFaceVisibility = 0.5
	approveBelow      = 0.4
	rejectAt          = 0.5
)

var errMalformed = errors.New("moderation response malformed")

// verdictFor applies the fail-closed product policy. The vendor scores
// `none`, `suggestive` and `mildly_suggestive` high for everyday swimwear and
// vests, so only the safety keys below participate in thresholds.
func verdictFor(response sightengineResponse) (Verdict, error) {
	if response.Status != "success" {
		return Verdict{}, errMalformed
	}
	if response.Nudity == nil || response.Offensive == nil || response.Faces == nil ||
		response.Text == nil || response.Text.Content == nil {
		return Verdict{}, errMalformed
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
			return Verdict{}, errMalformed
		}
	}

	verdict := Verdict{Reference: moderationReference(response.Request.ID)}
	for _, score := range safetyScores {
		if *score >= rejectAt {
			verdict.Status = StatusRejected
			return verdict, nil
		}
	}
	faceVisible, err := hasVisibleFace(*response.Faces)
	if err != nil {
		return Verdict{}, err
	}
	if !faceVisible || textguard.ContainsContactDetails(*response.Text.Content) {
		verdict.Status = StatusRejected
		return verdict, nil
	}
	for _, score := range safetyScores {
		if *score >= approveBelow {
			verdict.Status = StatusPending
			return verdict, nil
		}
	}
	verdict.Status = StatusApproved
	return verdict, nil
}

func hasVisibleFace(faces []sightengineFace) (bool, error) {
	for _, face := range faces {
		if face.X1 == nil || face.Y1 == nil || face.X2 == nil || face.Y2 == nil {
			return false, errMalformed
		}
		width := *face.X2 - *face.X1
		height := *face.Y2 - *face.Y1
		if width <= 0 || height <= 0 || width > 1 || height > 1 {
			return false, errMalformed
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
	if len(reference) > referenceLimit {
		return reference[:referenceLimit]
	}
	return reference
}

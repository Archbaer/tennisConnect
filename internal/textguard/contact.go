package textguard

import "regexp"

var (
	contactURL    = regexp.MustCompile(`(?i)(?:https?://|www\.)\S+`)
	contactDomain = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,63}(?::\d+)?(?:/\S*)?`)
	contactEmail  = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	contactSocial = regexp.MustCompile(`(?i)(?:instagram\.com|instagr\.am)/\S*|(?:^|[^A-Za-z0-9_])@[A-Za-z0-9_.-]+`)
	contactPhone  = regexp.MustCompile(`\+?\d[\d\s()./-]{6,}\d`)
)

func ContainsPhoneNumber(text string) bool {
	for _, candidate := range contactPhone.FindAllString(text, -1) {
		digits := 0
		for _, r := range candidate {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits >= 8 {
			return true
		}
	}
	return false
}

func ContainsContactDetails(text string) bool {
	return contactURL.MatchString(text) || contactDomain.MatchString(text) || contactEmail.MatchString(text) ||
		contactSocial.MatchString(text) || ContainsPhoneNumber(text)
}

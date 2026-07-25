package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GoogleTranslator struct {
	Client     *http.Client
	targetLang string // ""
}

func NewGoogleTranslator(targetLang string) *GoogleTranslator {
	if targetLang == "" {
		targetLang = "ru"
	}

	return &GoogleTranslator{
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
		targetLang: targetLang,
	}
}

func (g *GoogleTranslator) Translate(ctx context.Context, text string) (string, error) {
	cleanTxt := strings.TrimSpace(text)
	if cleanTxt == "" {
		return "", nil
	}
	endpoint := fmt.Sprintf("https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=%s&dt=t&q=%s",
		g.targetLang,
		url.QueryEscape(cleanTxt))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("Failed to create request:%w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := g.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Failed to do request:%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Translator API returned status:%d", resp.StatusCode)
	}

	var raw []any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", fmt.Errorf("Failed to decode response body:%w", err)
	}
	sliceOfSlices, ok := raw[0].([]any)
	if !ok {
		return "", fmt.Errorf("invalid response structre")
	}
	var sb strings.Builder
	for _, s := range sliceOfSlices {
		textSlice, ok := s.([]any)
		if ok && len(textSlice) > 0 {
			if translated, ok := textSlice[0].(string); ok {
				sb.WriteString(translated)
			}
		}

	}
	return sb.String(), nil

}

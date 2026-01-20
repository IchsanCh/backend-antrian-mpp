package config

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
)

type RecaptchaResponse struct {
	Success bool    `json:"success"`
	Score   float64 `json:"score"`
	Action  string  `json:"action"`
}

func VerifyRecaptcha(token string) (bool, float64, error) {
	data := url.Values{}
	data.Set("secret", os.Getenv("RECAPTCHA_SECRET_KEY"))
	data.Set("response", token)

	resp, err := http.PostForm(
		"https://www.google.com/recaptcha/api/siteverify",
		data,
	)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	var result RecaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, 0, err
	}

	return result.Success, result.Score, nil
}

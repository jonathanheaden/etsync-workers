package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type etsytoken struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty"`
	shopify_domain       string             `bson:"shopify_domain"`
	EtsyOnBoarded        bool               `bson:"etsyOnBoarded"`
	OnBoarded            bool               `bson:"onBoarded"`
	EtsyCodeReference    string             `bson:"etsy_code_reference,omitempty"`
	EtsyAccessToken      string             `bson:"etsy_access_token,omitempty"`
	EtsyCode             string             `bson:"etsy_code,omitempty"`
	EtsyCodeVerifier     string             `bson:"etsy_code_verifier,omitempty"`
	EtsyStateSecret      string             `bson:"etsy_state_secret,omitempty"`
	EtsyCodeError        string             `bson:"etsy_code_error,omitempty"`
	EtsyErrorDescription string             `bson:"etsy_error_description,omitempty"`
	EtsyErrorURI         string             `bson:"etsy_error_uri,omitempty"`
	EtsyTokenType        string             `bson:"etsy_token_type"`
	EtsyExpiresIn        int                `bson:"etsy_expires_in"`
	EtsyTokenExpires     time.Time          `bson:"etsy_token_expires"`
	EtsyRefreshToken     string             `bson:"etsy_refresh_token"`
}

type etsyTokenResponse struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	RefreshToken string    `json:"refresh_token"`
}

// Sample etsy token response
// {
//     "access_token": "532690296.Zj7Ls5EGzuSo_VXXZkTNXHb_h-zVanu1lqRrXmaxZQvFqMqSl4xNxNZwV4zEwwkqhlFgLWOrXFKWa7V5NZSJtDMGIk",
//     "token_type": "Bearer",
//     "expires_in": 3600,
//     "refresh_token": "532690296.R_JL6QMeLG19Et2tpS736GHHPMXn0qp5kCoZWC8hRGFFuT1xTOj1GvhrZFfPegDathMxcsxgmDqrS7taaJlGFuVg7M"
// }

func getEtsyTokenFromAPI(clientid, redirecturi string, etoken etsytoken) (etsytoken, error) {
	var response etsyTokenResponse
	var payloadstr string
	url := "https://api.etsy.com/v3/public/oauth/token"

	method := "POST"
	log.Info("Getting the etsy token: sending request to etsy API")
	if etoken.EtsyOnBoarded {
		payloadstr = fmt.Sprintf("grant_type=refresh_token&client_id=%s&refresh_token=%s", clientid, etoken.EtsyRefreshToken)
	} else {
		payloadstr = fmt.Sprintf("grant_type=authorization_code&client_id=%s&redirect_uri=%s&code=%s&code_verifier=%s", clientid, redirecturi, etoken.EtsyCodeReference, etoken.EtsyCodeVerifier)
	}

	payload := strings.NewReader(payloadstr)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		log.Error(err)
		return etsytoken{}, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return etsytoken{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Errorf("Request for token received a non successful statuscode: %d", res.StatusCode)
		return etsytoken{}, fmt.Errorf("Response unsuccessful: %s", res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return etsytoken{}, err
	}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Errorf("Error with response unmarshall: %v", err)
		return etsytoken{}, err
	}
	expiry_time := time.Now().Local().Add(time.Second * time.Duration(response.ExpiresIn))

	etoken.EtsyTokenExpires = expiry_time
	etoken.EtsyAccessToken = response.AccessToken
	etoken.EtsyRefreshToken = response.RefreshToken
	return etoken, nil
}

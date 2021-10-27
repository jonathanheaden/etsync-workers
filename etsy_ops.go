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

type etsyShop struct {
	ShopID   int    `json:"shop_id"`
	ShopName string `json:"shop_name"`
}

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

type etsyShopListingResult struct {
	ListingID                 int    `json:"listing_id"`
	ShopID                    int    `json:"shop_id"`
	Title                     string `json:"title"`
	Description               string `json:"description"`
	State                     string `json:"state"`
	CreationTimestamp         int    `json:"creation_timestamp"`
	EndingTimestamp           int    `json:"ending_timestamp"`
	OriginalCreationTimestamp int    `json:"original_creation_timestamp"`
	LastModifiedTimestamp     int    `json:"last_modified_timestamp"`
	StateTimestamp            int    `json:"state_timestamp"`
	Quantity                  int    `json:"quantity"`
}

type etsyShopListings struct {
	Count   int                     `json:"count"`
	Results []etsyShopListingResult `json:"results"`
}

type etsyProduct struct {
	ProductID int64  `json:"product_id"`
	Sku       string `json:"sku"`
	Offerings []struct {
		OfferingID int64 `json:"offering_id"`
		Quantity   int   `json:"quantity"`
		IsEnabled  bool  `json:"is_enabled"`
		IsDeleted  bool  `json:"is_deleted"`
		Price      struct {
			Amount       int    `json:"amount"`
			Divisor      int    `json:"divisor"`
			CurrencyCode string `json:"currency_code"`
		} `json:"price"`
	} `json:"offerings"`
	PropertyValues []struct {
		PropertyID   int         `json:"property_id"`
		PropertyName string      `json:"property_name"`
		ScaleID      interface{} `json:"scale_id"`
		ScaleName    interface{} `json:"scale_name"`
		ValueIds     []int       `json:"value_ids"`
		Values       []string    `json:"values"`
	} `json:"property_values"`
}

type etsyListing struct {
	Products           []etsyProduct `json:"products"`
	PriceOnProperty    []interface{} `json:"price_on_property"`
	QuantityOnProperty []int         `json:"quantity_on_property"`
	SkuOnProperty      []int         `json:"sku_on_property"`
	Listing            interface{}   `json:"listing"`
}

type etsyListingUpdate struct {
	Products []struct {
		Sku       string `json:"sku"`
		Offerings []struct {
			Quantity  int     `json:"quantity"`
			IsEnabled bool    `json:"is_enabled"`
			Price     float64 `json:"price"`
		} `json:"offerings"`
		PropertyValues []struct {
			PropertyID   int         `json:"property_id"`
			PropertyName string      `json:"property_name"`
			ScaleID      interface{} `json:"scale_id"`
			ValueIds     []int       `json:"value_ids"`
			Values       []string    `json:"values"`
		} `json:"property_values"`
	} `json:"products"`
	PriceOnProperty    []interface{} `json:"price_on_property"`
	QuantityOnProperty []int         `json:"quantity_on_property"`
	SkuOnProperty      []int         `json:"sku_on_property"`
	Listing            interface{}   `json:"listing"`
}

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

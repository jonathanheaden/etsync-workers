package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type etsyShop struct {
	ShopID   int    `json:"shop_id"`
	ShopName string `json:"shop_name"`
}

type etsytoken struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty"`
	ShopifyDomain        string             `bson:"shopify_domain"`
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
	Quantity                  int    `json:"quantity"` // <- this is the combined quantity for all variant products under this listing
}

type etsyShopListings struct {
	Count   int                     `json:"count"`
	Results []etsyShopListingResult `json:"results"`
}

type etsyOffering struct {
	OfferingID int64 `json:"offering_id"`
	Quantity   int   `json:"quantity"`
	IsEnabled  bool  `json:"is_enabled"`
	IsDeleted  bool  `json:"is_deleted"`
	Price      struct {
		Amount       int    `json:"amount"`
		Divisor      int    `json:"divisor"`
		CurrencyCode string `json:"currency_code"`
	} `json:"price"`
}

type etsyProduct struct {
	ListingID      int            `json:"listing_id"`
	ShopID         int            `json:"shop_id"`
	ShopifyDomain  string         `json:"shopify_domain"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	ProductID      int64          `json:"product_id"`
	Sku            string         `json:"sku"`
	IsDeleted      bool           `json:"is_deleted"`
	Offerings      []etsyOffering `json:"offerings"`
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
}

type EtsyAPIUpdate struct {
	Products           []EtsyProductUpdate `json:"products"`
	PriceOnProperty    []interface{}       `json:"price_on_property"`
	QuantityOnProperty []int               `json:"quantity_on_property"`
	SkuOnProperty      []int               `json:"sku_on_property"`
	Listing            interface{}         `json:"listing"`
}

type EtsyProductUpdate struct {
	Sku            string                            `json:"sku"`
	Offerings      []EtsyProductUpdateOffering       `json:"offerings"`
	PropertyValues []EtsyProductUpdatePropertyValues `json:"property_values"`
}

type EtsyProductUpdateOffering struct {
	Quantity  int     `json:"quantity"`
	IsEnabled bool    `json:"is_enabled"`
	Price     float64 `json:"price"`
}

type EtsyProductUpdatePropertyValues struct {
	PropertyID   int      `json:"property_id"`
	PropertyName string   `json:"property_name"`
	ValueIds     []int    `json:"value_ids"`
	Values       []string `json:"values"`
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

type etsyDelta struct {
	ProductID int64 `json:"product_id"`
	Delta     int   `json:"delta"`
}

func getEtsyTokenFromAPI(clientid, redirecturi string, etoken etsytoken) (etsytoken, error) {
	var response etsyTokenResponse
	var payloadstr string
	url := "https://api.etsy.com/v3/public/oauth/token"

	method := "POST"
	log.WithFields(log.Fields{
		"File":   "etsy_ops",
		"Caller": "GetEtsyTokenFromAPI",
	}).Debug("Getting the etsy token: sending request to etsy API")
	if etoken.EtsyOnBoarded {
		payloadstr = fmt.Sprintf("grant_type=refresh_token&client_id=%s&refresh_token=%s", clientid, etoken.EtsyRefreshToken)
	} else {
		payloadstr = fmt.Sprintf("grant_type=authorization_code&client_id=%s&redirect_uri=%s&code=%s&code_verifier=%s", clientid, redirecturi, etoken.EtsyCodeReference, etoken.EtsyCodeVerifier)
	}

	payload := strings.NewReader(payloadstr)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "GetEtsyTokenFromAPI",
		}).Error(err)
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
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "GetEtsyTokenFromAPI",
		}).Errorf("Request for token received a non successful statuscode: %d", res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "GetEtsyTokenFromAPI",
		}).Debug(string(body))
		return etsytoken{}, fmt.Errorf("Response unsuccessful: %s", res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return etsytoken{}, err
	}
	if err := json.Unmarshal(body, &response); err != nil {
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "GetEtsyTokenFromAPI",
			"Action": "unmarshall",
		}).Errorf("Error with response unmarshall: %v", err)
		return etsytoken{}, err
	}
	expiry_time := time.Now().Local().Add(time.Second * time.Duration(response.ExpiresIn))

	etoken.EtsyTokenExpires = expiry_time
	etoken.EtsyAccessToken = response.AccessToken
	etoken.EtsyRefreshToken = response.RefreshToken
	return etoken, nil
}

func getUsersEtsyShops(storename, clientid, token string, client *mongo.Client) (string, error) {
	var etsy_shop etsyShop
	user := strings.Split(token, ".")[0]
	log.Debugf("Getting shops for user id %s", user)
	url := fmt.Sprintf("https://openapi.etsy.com/v3/application/users/%s/shops", user)
	method := "GET"

	httpclient := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		log.Error(err)
		return "", err
	}
	req.Header.Add("x-api-key", clientid)
	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", token))

	res, err := httpclient.Do(req)
	if err != nil {
		log.Error(err)
		return "", err
	}
	defer res.Body.Close()
	log.WithFields(log.Fields{
		"File":   "etsy_ops",
		"Caller": "GetUsersEtsyShops",
	}).Debugf("Response for request to get User's Etsy Shops: %d", res.StatusCode)
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if err := json.Unmarshal(body, &etsy_shop); err != nil {
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "GetUsersEtsyShops",
			"Action": "unmarshall",
		}).Errorf("Error with response unmarshall: %v", err)
		return "", err
	}

	if err = saveEtsyShop(storename, etsy_shop, client); err != nil {
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "GetUsersEtsyShops",
			"Action": "SaveEtsyShop",
		}).Errorf("Error saving shop to DB: %v", err)
		return "", err
	}
	log.WithFields(log.Fields{
		"File":   "etsy_ops",
		"Caller": "GetUsersEtsyShops",
	}).Debugf("Got shop id %d for shop name %s", etsy_shop.ShopID, etsy_shop.ShopName)
	return fmt.Sprintf("%d", etsy_shop.ShopID), nil
}

func getAndSetEtsyShopListings(storename, etsy_shopid, clientid, token string, eSkusToSet map[int]string, overrideStock map[string]int, client *mongo.Client) error {
	var shoplistings etsyShopListings
	url := fmt.Sprintf("https://openapi.etsy.com/v3/application/shops/%s/listings", etsy_shopid)
	method := "GET"

	httpclient := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		log.Error(err)
		return err
	}
	req.Header.Add("x-api-key", clientid)
	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", token))

	res, err := httpclient.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "GetAndSetEtsyShopListings",
			"Action": "Read Body",
		}).Error(err)
		return err
	}

	if err := json.Unmarshal(body, &shoplistings); err != nil {
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "GetAndSetEtsyShopListings",
			"Action": "unmarshall",
		}).Errorf("Error with response unmarshall: %v", err)
		return err
	}
	log.WithFields(log.Fields{
		"File":   "etsy_ops",
		"Caller": "GetAndSetEtsyShopListings",
	}).Debugf("Got %d shop listings back from Etsy", shoplistings.Count)
	if err = reconcileInventoryListings(storename, etsy_shopid, clientid, token, shoplistings.Results, eSkusToSet, overrideStock, client); err != nil {

	}
	return nil
}

func updateEtsyShopListing(listing_id int, payloadstr, clientid, token string) error {
	url := fmt.Sprintf("https://openapi.etsy.com/v3/application/listings/%d/inventory", listing_id)
	method := "PUT"

	payload := strings.NewReader(payloadstr)

	httpclient := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		log.Error(err)
		return err
	}
	req.Header.Add("x-api-key", clientid)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", token))

	res, err := httpclient.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}

	if res.StatusCode != 200 {
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "UpdateEtsyShopListing",
		}).Errorf("Failed to update inventory for listing %d. Got status code: %d", listing_id, res.StatusCode)
		return fmt.Errorf("Failed to update inventory with status %d", res.StatusCode)
	}
	return nil
}

func reconcileInventoryListings(storename, etsy_shopid, clientid, token string, listings []etsyShopListingResult, eSkusToSet map[int]string, overrideStock map[string]int, client *mongo.Client) error {

	method := "GET"
	httpclient := &http.Client{}
	for _, l := range listings {
		var etsyproducts []etsyProduct
		var etsy_listing etsyListing
		url := fmt.Sprintf("https://openapi.etsy.com/v3/application/listings/%d/inventory", l.ListingID)
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			log.Error(err)
			return err
		}
		req.Header.Add("x-api-key", clientid)
		req.Header.Add("authorization", fmt.Sprintf("Bearer %s", token))
		res, err := httpclient.Do(req)
		if err != nil {
			fmt.Println(err)
			return err
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.WithFields(log.Fields{
				"File":   "etsy_ops",
				"Caller": "ReconcileInventoryListings",
				"Action": "Read Listing Body",
			}).Error(err)
			return err
		}
		if err := json.Unmarshal(body, &etsy_listing); err != nil {
			log.WithFields(log.Fields{
				"File":   "etsy_ops",
				"Caller": "ReconcileInventoryListings",
				"Action": "unmarshall",
			}).Errorf("Error with response unmarshall: %v", err)
			return err
		}
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "ReconcileInventoryListings",
		}).Debugf("Got %d products in listing for %s", len(etsy_listing.Products), l.Title)
		productlist := new(bytes.Buffer)
		for _, product := range etsy_listing.Products {
			fmt.Fprintf(productlist, "[ProductID %d SKU %s], ", product.ProductID, product.Sku)
		}
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "ReconcileInventoryListings",
		}).Debugf("Products in listing: %s", productlist)
		for _, p := range etsy_listing.Products {

			p.ShopifyDomain = storename
			p.ListingID = l.ListingID
			p.ShopID = l.ShopID
			p.Title = l.Title
			p.Description = l.Description
			etsyproducts = append(etsyproducts, p)
		}
		delta, err := saveEtsyProducts(storename, etsyproducts, eSkusToSet, overrideStock, client)
		if err != nil {
			log.Errorf("Error saving products to DB: %v", err)
			return err
		}

		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "ReconcileInventoryListings",
		}).Debugf("Completed write for products in listing %d", l.ListingID)
		// To write inventory back to etsy we need to follow guidance in https://developers.etsy.com/documentation/tutorials/listings/#updating-inventory
		// To get the product array, call getListingInventory for the listing.
		// From the getListingInventory response, remove the following fields: product_id, offering_id, scale_name and is_deleted.
		// Also change the price array in offerings to be a decimal value instead of an array.
		if delta.EstyHasChanges {
			if err = reconcileEtsyStockLevel(storename, clientid, token, l.ListingID, etsy_listing, delta, eSkusToSet, overrideStock, client); err != nil {
				log.Error(err)
			}
		}
		if delta.ShopifyHasChanges {
			stoken := getstoretoken(storename, client)
			if err = reconcileShopifyStockLevel(storename, clientid, stoken, delta, overrideStock, client); err != nil {
				log.Error(err)
			}
		}

	}
	return nil
}

func reconcileEtsyStockLevel(storename, clientid, token string, ListingID int, etsy_listing etsyListing, delta StockReconciliationDelta, eSkusToSet map[int]string, overrideStock map[string]int, client *mongo.Client) error {
	var apiUpdate EtsyAPIUpdate
	apiUpdate.PriceOnProperty = etsy_listing.PriceOnProperty
	apiUpdate.QuantityOnProperty = etsy_listing.QuantityOnProperty
	apiUpdate.SkuOnProperty = etsy_listing.SkuOnProperty
	log.WithFields(log.Fields{
		"File":   "etsy_ops",
		"Caller": "ReconcileEtsyStockLevel",
	}).Infof("Preparing to send update to Etsy for listing %d",ListingID)
	for _, p := range etsy_listing.Products {
		log.WithFields(log.Fields{
			"File":   "etsy_ops",
			"Caller": "ReconcileEtsyStockLevel",
		}).Debugf("Preparing update for %d %s", p.ProductID, p.Title)
		var epu EtsyProductUpdate
		if skutoset, ok := eSkusToSet[int(p.ProductID)]; ok {
			epu.Sku = skutoset
		} else {
			epu.Sku = p.Sku
		}
		var epuo EtsyProductUpdateOffering
		if stockdelta, ok := delta.EtsyDelta[p.ProductID]; ok {
			log.WithFields(log.Fields{
				"File":   "etsy_ops",
				"Caller": "ReconcileEtsyStockLevel",
				"Action": "Read from etsy delta stock map",
			}).Infof("Product has stock level change required %d", stockdelta)
			epuo.Quantity = p.Offerings[0].Quantity + stockdelta
			if epuo.Quantity < 0 {
				epuo.Quantity = 0
			}
		} else if stockset, ok := overrideStock[p.Sku]; ok {
			log.WithFields(log.Fields{
				"File":   "etsy_ops",
				"Caller": "ReconcileEtsyStockLevel",
				"Action": "Read from override stock map",
			}).Infof("Product has stock level change required (set via app) %d", stockset)
			epuo.Quantity = stockset
		} else {
			epuo.Quantity = p.Offerings[0].Quantity
		}
		epuo.IsEnabled = p.Offerings[0].IsEnabled
		epuo.Price = (float64(p.Offerings[0].Price.Amount) / float64(p.Offerings[0].Price.Divisor))
		epu.Offerings = append(epu.Offerings, epuo)
		for _, pv := range p.PropertyValues {
			log.WithFields(log.Fields{
				"File":   "etsy_ops",
				"Caller": "ReconcileEtsyStockLevel",
				"Action": "Prepare update",
			}).Debugf("Adding property value %s", pv.PropertyName)
			var epupv EtsyProductUpdatePropertyValues
			epupv.PropertyID = pv.PropertyID
			epupv.PropertyName = pv.PropertyName
			epupv.ValueIds = pv.ValueIds
			epupv.Values = pv.Values
			epu.PropertyValues = append(epu.PropertyValues, epupv)
		}
		apiUpdate.Products = append(apiUpdate.Products, epu)
	}
	payload, err := json.Marshal(apiUpdate)
	if err != nil {
		panic(err)
	}
	log.WithFields(log.Fields{
		"File":   "etsy_ops",
		"Caller": "ReconcileEtsyStockLevel",
	}).Debugf("Sending update to Etsy: %s", string(payload))
	if err = updateEtsyShopListing(ListingID, string(payload), clientid, token); err != nil {
		log.WithFields(log.Fields{
			"File":    "etsy_ops",
			"Caller":  "ReconcileEtsyStockLevel",
			"Calling": "UpdateEtsyShopListing",
		}).Errorf("Could not update etsy : %v", err)
		return err
	}
	log.WithFields(log.Fields{
		"File":   "etsy_ops",
		"Caller": "ReconcileEtsyStockLevel",
	}).Infof("Successfully updated Etsy listing stock level for %d", ListingID)
	if err = setEtsyStockLevelForProducts(storename, apiUpdate.Products, client); err != nil {
		log.WithFields(log.Fields{
			"File":    "etsy_ops",
			"Caller":  "ReconcileEtsyStockLevel",
			"Calling": "SetEtsyStockLevelForProducts",
		}).Errorf("failed to write Etsy Product stock to DB %v", err)
	}
	return nil
}

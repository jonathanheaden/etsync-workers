package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	shopname *string
	client   *mongo.Client
)

func init() {
	shopname = flag.String("shop", "", "the shop to run inventory check & set for")
	debuglogging := flag.Bool("debug", false, "Use Debug log level")
	flag.Parse()
	log.Infof("Processing inventory updates for %s", *shopname)
	if *debuglogging {
		log.SetLevel(log.DebugLevel)
	}
	log.Debug("Debug Logging Enabled")

}

func main() {

	config, err := LoadConfig(".")
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}
	client, err := mongo.NewClient(options.Client().ApplyURI(config.MONGO_URI))
	if err != nil {
		log.Fatalf("Cannot instantiate new client: %v", err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatalf("Connection timeout setting up DB Connection: %v", err)
	}

	defer client.Disconnect(ctx)

	overridestock, e := getOverrides(*shopname, client)
	if e != nil {
		log.Error(e)
	}
	eSkusToSet, e := getItemsToLink(*shopname, client)
	if e != nil {
		log.Error(e)
	}
	bstock := new(bytes.Buffer)
	for key, value := range overridestock {
		fmt.Fprintf(bstock, "%s=%d ", key, value)
	}
	bsku := new(bytes.Buffer)
	for key, value := range eSkusToSet {
		fmt.Fprintf(bsku, "%d=%s ", key, value)
	}
	if len(overridestock) > 0 {
		log.WithFields(log.Fields{
			"Caller": "Main",
		}).Infof("Items for which we need to set stock levels: %v", bstock)
	}
	if len(eSkusToSet) > 0 {
		log.WithFields(log.Fields{
			"Caller": "Main",
		}).Infof("Etsy Items for which we need to set the sku: %v", bsku)
	}
	token := getstoretoken(*shopname, client)

	inventoryurl, err := getinventorylevels(*shopname, token)
	if err != nil {
		log.WithFields(log.Fields{
			"Caller":  "Main",
			"Calling": "GetInventoryLevels",
		}).Fatalf("Unable to register query for inventory levels: %v", err)
	}
	if err = processinventorylevels(inventoryurl, *shopname, client); err != nil {
		log.WithFields(log.Fields{
			"Caller":  "Main",
			"Calling": "ProcessInventoryLevels",
		}).Fatalf("Unable to process inventory levels: %v", err)
	}
	productsurl, err := getproductvariants(*shopname, token)
	if err != nil {
		log.WithFields(log.Fields{
			"Caller":  "Main",
			"Calling": "ProcessProductLevels",
		}).Fatalf("Unable to get product variants: %v", err)
	}
	log.WithFields(log.Fields{
		"Caller":  "Main",
		"Calling": "GetProductVariants",
	}).Info("Ready to process productvariants")
	if err = processproductlevels(productsurl, *shopname, client); err != nil {
		log.WithFields(log.Fields{
			"Caller":  "Main",
			"Calling": "ProcessProductLevels",
		}).Fatalf("Unable to process products: %v", err)
	}

	// get the etsy stock levels and apply any shopify changes
	e_token, err := getetsytoken(config, client)
	if err != nil {
		log.WithFields(log.Fields{
			"Caller":  "Main",
			"Calling": "GetEtsyToken",
		}).Errorf("Error getting etsy token from db %v", err)
		log.Fatal("Cannot get Etsy token")
	}
	log.WithFields(log.Fields{
		"Caller":  "Main",
		"Calling": "GetEtsyToken",
	}).Infof("Got Token for Etsy (shopify store %s) with expiration time %v", e_token.ShopifyDomain, e_token.EtsyTokenExpires)

	etsyshopid, err := getUsersEtsyShops(*shopname, config.ETSY_CLIENT_ID, e_token.EtsyAccessToken, client)
	if err != nil {
	}

	err = getAndSetEtsyShopListings(*shopname, etsyshopid, config.ETSY_CLIENT_ID, e_token.EtsyAccessToken, eSkusToSet, overridestock, client)
	if err != nil {
		log.WithFields(log.Fields{
			"Caller":  "Main",
			"Calling": "GetAndSetEtsyShopListings",
		}).Fatalf("Could not retrieve Etsy Listings %v", err)
	}

	//apply any etsy stock changes to shopify
}

package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	client *mongo.Client
)

func init() {
	log.SetLevel(log.DebugLevel)
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
		log.Fatalf("Connection timeout: %v", err)
	}

	defer client.Disconnect(ctx)

	overridestock, e := getOverrides(config.SHOP_NAME, client)
	if e != nil {
		log.Error(e)
	}
	eSkusToSet, e := getItemsToLink(config.SHOP_NAME, client)
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
	log.Infof("Items for which we need to set stock levels: %v", bstock)
	log.Infof("Etsy Items for which we need to set the sku: %v", bsku)

	token := getstoretoken(config.SHOP_NAME, client)

	inventoryurl, err := getinventorylevels(config.SHOP_NAME, token)
	if err != nil {
		log.Fatalf("Unable to register query for inventory levels: %v", err)
	}
	if err = processinventorylevels(inventoryurl, config.SHOP_NAME, client); err != nil {
		log.Fatalf("Unable to process inventory levels: %v", err)
	}
	productsurl, err := getproductvariants(config.SHOP_NAME, token)
	if err != nil {
		log.Fatalf("Unable to register query for products: %v", err)
	}
	if err = processproductlevels(productsurl, config.SHOP_NAME, client); err != nil {
		log.Fatalf("Unable to process products: %v", err)
	}

	// get the etsy stock levels and apply any shopify changes
	e_token, err := getetsytoken(config, client)
	if err != nil {
		log.Errorf("Error getting etsy token from db %v", err)
		log.Fatal("Cannot get Etsy token")
	}
	log.Infof("Got Token for Etsy (shopify store %s) with expiration time %v", e_token.ShopifyDomain, e_token.EtsyTokenExpires)

	etsyshopid, err := getUsersEtsyShops(config.SHOP_NAME, config.ETSY_CLIENT_ID, e_token.EtsyAccessToken, client)
	if err != nil {
		log.Fatalf("Could not retrieve users Etsy Shops %v", err)
	}

	err = getAndSetEtsyShopListings(config.SHOP_NAME, etsyshopid, config.ETSY_CLIENT_ID, e_token.EtsyAccessToken, eSkusToSet, overridestock, client)
	if err != nil {
		log.Fatalf("Could not retrieve Etsy Listings %v", err)
	}

	//apply any etsy stock changes to shopify
}

package main

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func getdatabases(client *mongo.Client) ([]string, error) {
	var dblist []string
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	dblist, err := client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		log.Warn(err)
		return dblist, err
	}
	return dblist, nil
}

func getstoretoken(storename string, client *mongo.Client) string {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	var doc bson.M
	collection := client.Database("etync").Collection("shops")
	filter := bson.D{{"shopify_domain", storename}}
	if err := collection.FindOne(ctx, filter).Decode(&doc); err != nil {
		log.Warn(err)
		return ""
	}
	log.Info("Got shop record from database for shopify token")
	return fmt.Sprintf("%v", doc["accessToken"])
}

func getetsytoken(config Config, client *mongo.Client) (etsytoken, error) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	var token etsytoken
	collection := client.Database("etync").Collection("shops")
	filter := bson.D{{"shopify_domain", config.SHOP_NAME}}
	if err := collection.FindOne(ctx, filter).Decode(&token); err != nil {
		log.Warn(err)
		return etsytoken{}, err
	}
	log.Info("Got shop record from database for etsy token")

	if token.EtsyOnBoarded && (time.Now().Add(10 * time.Minute).Before(token.EtsyTokenExpires)) {
		// etsy has been onboarded & the etsy accesscode has not expired
		return token, nil
		log.Info("Etsy token has greater than 10 minutes ttl, reusing current token")
	} else {
		log.Info("New Etsy token required, sending request to etsy API")
		token, err := getEtsyTokenFromAPI(config.ETSY_CLIENT_ID, config.ETSY_REDIRECT_URI, token)
		if err != nil {
			return etsytoken{}, err
		}
		token.EtsyOnBoarded = true
		if err := writeEtsyToken(config.SHOP_NAME, token, client); err != nil {
			log.Errorf("Unable to store the etsy token in database! %v", err)
			return etsytoken{}, err
		}
	}
	return token, nil

}

func writeEtsyToken(storename string, token etsytoken, client *mongo.Client) error {
	log.Info("Writing the etsy token to DB")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	shop_collection := client.Database("etync").Collection("shops")
	filter := bson.D{{"shopify_domain", storename}}
	update := bson.M{
		"$set": bson.M{
			"etsyOnBoarded":      token.EtsyOnBoarded,
			"etsy_access_token":  token.EtsyAccessToken,
			"etsy_refresh_token": token.EtsyRefreshToken,
			"etsy_token_expires": token.EtsyTokenExpires,
		},
	}

	opts := options.FindOneAndUpdate().SetUpsert(true)
	result := shop_collection.FindOneAndUpdate(ctx, filter, update, opts)
	if result.Err() != nil {
		log.Infof("No prior record found when inserting doc %s", result.Err())
		return result.Err()
	}
	log.Info("Success writing etsy token to Database")
	return nil
}

func saveEtsyShop(storename string, etsy_shop etsyShop, client *mongo.Client) error {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	shop_collection := client.Database("etync").Collection("shops")
	filter := bson.D{{"shopify_domain", storename}}
	update := bson.M{
		"$set": bson.M{
			"etsy_shop_id":   etsy_shop.ShopID,
			"etsy_shop_name": etsy_shop.ShopName,
		},
	}

	opts := options.FindOneAndUpdate().SetUpsert(true)
	result := shop_collection.FindOneAndUpdate(ctx, filter, update, opts)
	if result.Err() != nil {
		log.Infof("Error writing Etsy shop details %s", result.Err())
		return result.Err()
	}
	log.Info("Success writing etsy shop details to Database")
	return nil
}

func setshopstock(storename string, items []ShopifyItem, client *mongo.Client) error {

	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	stockCollection := client.Database("etync").Collection("stock")
	for _, item := range items {
		var existingRecord ShopifyItem
		var update bson.M
		itemtype := item.ItemType
		item.ItemType = "shopify-stock-level"
		filter := bson.M{"s_inventory_id": item.InventoryID}
		if itemtype == "inventory" {
			if err := stockCollection.FindOne(ctx, filter).Decode(&existingRecord); err != nil {
				log.WithFields(log.Fields{
					"ID":       item.InventoryID,
					"Response": err,
				}).Infof("Record not found, initialising with current stock level %d", item.Available)
				item.PriorAvailable = item.Available
			} else {
				item.PriorAvailable = existingRecord.Available
				log.Infof("Loading existing record for %s: stock levels (prev->new) %d -> %d", item.InventoryID, item.PriorAvailable, item.Available)
			}
			update = bson.M{
				"$set": item,
			}
		} else {
			// this is a product variant so we explicitly set the fields we want to write so as to avoid overwriting the stock levels
			update = bson.M{
				"$set": bson.M{
					"s_parent_product":    item.Parent,
					"s_parent_product_id": item.ParentID,
					"sku":                 item.SKU,
					"s_variant_id":        item.VariantID,
					"s_variant_name":      item.VariantName,
				},
			}
		}
		log.WithFields(log.Fields{
			"ID": item.InventoryID,
		}).Info("Updating Database")

		opts := options.FindOneAndUpdate().SetUpsert(true)

		result := stockCollection.FindOneAndUpdate(ctx, filter, update, opts)
		if result.Err() != nil {
			log.Infof("No prior record found when inserting doc %s", result.Err())
			continue
		}
		doc := bson.M{}
		if err := result.Decode(&doc); err != nil {
			log.Errorf("Problem decoding record for %s", item.InventoryID)
		}
		log.WithFields(log.Fields{
			"Kind": itemtype,
		}).Info(fmt.Sprintf("Upserted Doc %s", item.InventoryID))
	}

	return nil
}

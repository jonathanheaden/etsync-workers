package main

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	// "go.mongodb.org/mongo-driver/mongo/readpref"
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
	log.Info("Got shop record from database")
	return fmt.Sprintf("%v", doc["accessToken"])
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
			"ID":                   item.InventoryID,
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

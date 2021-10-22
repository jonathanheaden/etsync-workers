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

func upsertproduct(storename string, items []ShopifyItem, client *mongo.Client) error {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

	productsCollection := client.Database("etync").Collection("products")
	for _, item := range items {
		log.WithFields(log.Fields{
			"Store": storename,
			"title": item.VariantName,
			"SKU":   item.SKU,
		}).Info("Adding record")
		filter := bson.M{"SKU": item.SKU}
		update := bson.M{
			"$set": bson.M{
				"storename":      storename,
				"s_title":        item.VariantName,
				"s_id":           item.VariantID,
				"s_inventory_id": item.InventoryID,
				"s_product_id":   item.ParentID,
				"s_product":      item.Parent,
			},
		}
		upsert := true
		after := options.After
		opt := options.FindOneAndUpdateOptions{
			ReturnDocument: &after,
			Upsert:         &upsert,
		}
		result := productsCollection.FindOneAndUpdate(ctx, filter, update, &opt)
		if result.Err() != nil {
			log.Errorf("Error inserting record %v Err: %v", item, result.Err())
			return result.Err()
		}
		log.Info("Added record %v", result.Decode)
	}

	log.Info("Completed insert for %d products", len(items))
	return nil
}

func upsertstock(storename string, items []ShopifyItem, client *mongo.Client) error {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

	inventoryCollection := client.Database("etync").Collection("inventory")
	for _, item := range items {
		log.WithFields(log.Fields{
			"Store":          storename,
			"s_inventory_id": item.InventoryID,
		}).Info("Adding record")
		filter := bson.M{"SKU": item.SKU}
		update := bson.M{
			"$set": bson.M{
				"storename":      storename,
				"s_location_id":  item.LocationID,
				"s_curr_avail":   item.Available,
				"s_inventory_id": item.InventoryID,
			},
		}
		upsert := true
		after := options.After
		opt := options.FindOneAndUpdateOptions{
			ReturnDocument: &after,
			Upsert:         &upsert,
		}
		result := inventoryCollection.FindOneAndUpdate(ctx, filter, update, &opt)
		if result.Err() != nil {
			log.Errorf("Error inserting record %v Err: %v", item, result.Err())
			return result.Err()
		}
		log.Info("Added record %v", result.Decode)
	}

	log.Info("Completed insert for %d products", len(items))
	return nil
}

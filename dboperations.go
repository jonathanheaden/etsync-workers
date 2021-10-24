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
	for _,item := range items {
		itemtype := item.ItemType
		item.ItemType = "shopify-stock-level"
		filter := bson.M{"s_inventoryid": item.InventoryID}
		update := bson.M{
			"$set": item,
		}

		upsert := true
		after := options.After
		opt := options.FindOneAndUpdateOptions{
			ReturnDocument: &after,
			Upsert:         &upsert,
		}

	result := stockCollection.FindOneAndUpdate(ctx, filter, update, &opt)
	if result.Err() != nil {
		return nil
	}
	doc := bson.M{}
	if err := result.Decode(&doc); err != nil {
		log.Errorf("Problem decoding record for ", item.InventoryID)
	}
	log.WithFields(log.Fields{
		"Kind": itemtype, 
	}).Info(fmt.Sprintf("Upserted Doc %s",item.InventoryID))
	}

	return nil
}

package main

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	// "go.mongodb.org/mongo-driver/mongo/options"
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
	return fmt.Sprintf("%v", doc["accessToken"])
}

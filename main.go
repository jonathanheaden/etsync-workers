package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	// "go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	storename := "etsync.myshopify.com"
	config, err := LoadConfig(".")
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}
	client, err := mongo.NewClient(options.Client().ApplyURI(config.MONGO_URI))
	if err != nil {
		log.Fatalf("Cannot instantiate new client: %v",err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatalf("Connection timeout: %v",err)
	}

	defer client.Disconnect(ctx)

	token := getstoretoken(storename, client)
	
	// inventoryurl, err := getinventorylevels(storename, token)
	// if err != nil {
	// 	log.Fatalf("Unable to register query for inventory levels: %v",err)
	// }
	// if err = processinventorylevels(inventoryurl, storename); err != nil {
	// 	log.Fatalf("Unable to process inventory levels: %v",err)
	// }
	productsurl, err := getproductvariants(storename, token)
	if err != nil {
		log.Fatalf("Unable to register query for products: %v",err)
	}
	if err = processproductlevels(productsurl, storename); err != nil {
		log.Fatalf("Unable to process products: %v",err)
	}
}

package main

import (
	"context"
	"fmt"
	"time"
  
  log "github.com/sirupsen/logrus"
  // "go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

)

func main() {
	config, err := LoadConfig(".")
	if err != nil {
		log.Fatal("cannot load config:", err)
	}
	client, err := mongo.NewClient(options.Client().ApplyURI(config.MONGO_URI))
	if err != nil {
		log.Fatal(err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer client.Disconnect(ctx)
  databaselist, err := getdatabases(client)
 
	fmt.Println(databaselist)
  fmt.Printf("token %s", getstoretoken("etsync.myshopify.com", client))
}

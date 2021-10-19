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
  storename := "etsync.myshopify.com"
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

  token := getstoretoken(storename, client)
  statusurl,err := registerbulkquery(storename, token)
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println(statusurl)
}

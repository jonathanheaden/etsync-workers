package main

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type StockItem struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"`
	ShopifyDomain       string             `bson:shopify_domain,omitempty`
	ItemType            string             `bson:"itemtype,omitempty"`
	Available           int                `bson:"s_curr_stock"`
	PriorAvailable      int                `bson:"s_prev_stock"`
	InventoryID         string             `bson:"s_inventory_id,omitempty"`
	LocationID          string             `bson:"s_location_id,omitempty"`
	Parent              string             `bson:"s_parent_product,omitempty"`
	ParentID            string             `bson:"s_parent_product_id,omitempty"`
	SKU                 string             `bson:"sku,omitempty"`
	VariantID           string             `bson:"s_variant_id,omitempty"`
	VariantName         string             `bson:"s_variant_name,omitempty`
	EtsyProductID       int                `bson:"e_product_id,omitempty"`
	EtsyDescription     string             `bson:"e_description,omitempty"`
	EtsyProductTitle    string             `bson:"e_product_title,omitempty"`
	EtsyShopID          int                `bson:"e_shop_id,omitempty"`
	EtsyQuantity        int                `bson:"e_curr_stock"`
	EtsyPriorQuantity   int                `bson:"e_prev_stock"`
	EtsyItemInitialised bool               `bson:"e_item_initialised"`
}

type StockReconciliationDelta struct {
	EtsyDelta         map[int64]int  `json:"etsy_delta"`
	ShopifyDelta      map[string]int `json:"shopify_delta"`
	EstyHasChanges    bool           `json:"etsyhaschanges"`
	ShopifyHasChanges bool           `json:"shopifyhaschanges"`
}

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
		log.Info("Etsy token has greater than 10 minutes ttl, reusing current token")
		return token, nil
	} else {
		log.Info("New Etsy token required, sending request to etsy API")
		rtoken, err := getEtsyTokenFromAPI(config.ETSY_CLIENT_ID, config.ETSY_REDIRECT_URI, token)
		if err != nil {
			return etsytoken{}, err
		}
		rtoken.EtsyOnBoarded = true
		rtoken.ShopifyDomain = config.SHOP_NAME // if this is a new token from etsy API then it won't have the shop
		log.Infof("Token retrieved from etsy api for %s with expiration %v", rtoken.ShopifyDomain, rtoken.EtsyTokenExpires)

		if err := writeEtsyToken(config.SHOP_NAME, rtoken, client); err != nil {
			log.Errorf("Unable to store the etsy token in database! %v", err)
			return etsytoken{}, err
		}
		token = rtoken
	}
	return token, nil

}

func getShopifyStockItem(storename, VariantId string, client *mongo.Client) (StockItem, error){
	var item StockItem
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	stockCollection := client.Database("etync").Collection("stock")
	filter := bson.D{{"shopify_domain", storename}, {"s_variant_id", VariantId}}

	if err := stockCollection.FindOne(ctx, filter).Decode(&item); err != nil {
		log.Infof("Error writing Etsy shop details %v", err)
		return StockItem{},err
	}
	return item,nil
	
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

// When we write the etsy inventory level to the DB we need to decide if this is the first time the
// Etsy record is being written, if so then the current inventory level is the previous level
// If this is not the first time then there  should be an inventory level so use that as the prior level
// This function returns a delta list for the products in the listing
// This should be returned as a struct with two independent sets of actions:
// 1. a map of productid -> delta which gets applied to the Etsy API
// 2. a map of shopify variant Ids -> delta which gets applied to Shopify API
func saveEtsyProducts(storename string, products []etsyProduct, client *mongo.Client) (StockReconciliationDelta, error) {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	var stockdelta StockReconciliationDelta
	etsyDelta := make(map[int64]int)
	shopifyDelta := make(map[string]int)
	var existingRecord StockItem
	stockCollection := client.Database("etync").Collection("stock")
	for _, p := range products {
		updateRecord := bson.M{
			"shop_id":        p.ShopID,
			"product_title":  p.Title,
			"description":    p.Description,
			"sku":            p.Sku,
			"shopify_domain": p.ShopifyDomain,
			"e_curr_stock":   p.Offerings[0].Quantity,
		}
		log.WithFields(log.Fields{
			"Product_ID": p.ProductID,
			"Title":      p.Title,
			"Sku":        p.Sku,
		}).Info("Updating DB with Etsy product")
		filter := bson.M{"sku": p.Sku, "shopify_domain": p.ShopifyDomain}
		if err := stockCollection.FindOne(ctx, filter).Decode(&existingRecord); err != nil {
			log.WithFields(log.Fields{
				"etsy-ProductID": p.ProductID,
				"Response":       err,
			}).Infof("Record not found for shopify item with this sku, initialising with current stock level %d", p.Offerings[0].Quantity)
			updateRecord["e_prev_stock"] = p.Offerings[0].Quantity
			updateRecord["e_item_exists"] = true
		} else {
			updateRecord["e_prev_stock"] = existingRecord.EtsyQuantity
			if !existingRecord.EtsyItemInitialised {
				updateRecord["e_prev_stock"] = p.Offerings[0].Quantity
				updateRecord["e_item_initialised"] = true
			}
			log.Infof("Loading existing record for %d: stock levels (prev->new) %d -> %d", p.ProductID, updateRecord["e_prev_stock"], p.Offerings[0].Quantity)
			if existingRecord.Available != existingRecord.PriorAvailable {
				stockdelta.EstyHasChanges = true
				etsyDelta[p.ProductID] = (existingRecord.Available - existingRecord.PriorAvailable)
			}
			
			if updateRecord["e_curr_stock"] != updateRecord["e_prev_stock"] {
				stockdelta.ShopifyHasChanges = true
				shopifyDelta[existingRecord.VariantID] = (p.Offerings[0].Quantity - existingRecord.EtsyQuantity)
			}
		}
		update := bson.M{
			"$set": updateRecord,
		}

		opts := options.FindOneAndUpdate().SetUpsert(true)
		result := stockCollection.FindOneAndUpdate(ctx, filter, update, opts)
		if result.Err() != nil {
			log.Infof("No prior listing recorded, adding new %s", result.Err())
			continue
		}
	}
	stockdelta.EtsyDelta = etsyDelta
	stockdelta.ShopifyDelta = shopifyDelta

	log.Infof("Success writing %d etsy listing details to Database", len(products))
	return stockdelta, nil
}

func setEtsyStockLevelForProducts(storename string, products []EtsyProductUpdate, client *mongo.Client) error {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	stockCollection := client.Database("etync").Collection("stock")
	for _, item := range products {
		filter := bson.M{"sku": item.Sku, "shopify_domain": storename}
		update := bson.M{
			"$set": bson.M{
				"e_curr_stock": item.Offerings[0].Quantity,
				"e_prev_stock": item.Offerings[0].Quantity,
			},
		}
		opts := options.FindOneAndUpdate().SetUpsert(false)

		result := stockCollection.FindOneAndUpdate(ctx, filter, update, opts)
		if result.Err() != nil {
			log.Infof("No prior record found when inserting doc %s", result.Err())
			return result.Err()
		}
	}
	return nil
}

func setShopifyStockLevelForVariant(storename, VariantId string, stocklevel int, client *mongo.Client) error {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	stockCollection := client.Database("etync").Collection("stock")
	
	filter := bson.D{{"shopify_domain", storename}, {"s_variant_id", VariantId}}
	update := bson.M{
		"$set": bson.M{
			"s_curr_stock": stocklevel,
			"s_prev_stock": stocklevel,
		},
	}
	opts := options.FindOneAndUpdate().SetUpsert(false)

	result := stockCollection.FindOneAndUpdate(ctx, filter, update, opts)
	if result.Err() != nil {
		log.Infof("No prior record found when inserting doc %s", result.Err())
		return result.Err()
	}
	return nil
}

func setshopstock(storename string, items []StockItem, client *mongo.Client) error {

	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	stockCollection := client.Database("etync").Collection("stock")
	for _, item := range items {
		var existingRecord StockItem
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
				item.EtsyItemInitialised = false
			} else {
				item.PriorAvailable = existingRecord.Available
				log.Infof("Loading existing record for %s: stock levels (prev->new) %d -> %d", item.InventoryID, item.PriorAvailable, item.Available)
			}
			update = bson.M{
				"$set": bson.M{
					"shopify_domain": storename,
					"s_curr_stock":   item.Available,
					"s_prev_stock":   item.PriorAvailable,
					"s_inventory_id": item.InventoryID,
					"s_location_id":  item.LocationID,
				},
			}
		} else {
			// this is a product variant so we explicitly set the fields we want to write so as to avoid overwriting the stock levels
			update = bson.M{
				"$set": bson.M{
					"shopify_domain":      storename,
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

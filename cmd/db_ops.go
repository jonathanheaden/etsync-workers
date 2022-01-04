package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type StockItem struct {
	ID                     primitive.ObjectID `bson:"_id,omitempty"`
	ShopifyDomain          string             `bson:shopify_domain,omitempty`
	ItemType               string             `bson:"itemtype,omitempty"`
	Available              int                `bson:"s_curr_stock"`
	PriorAvailable         int                `bson:"s_prev_stock"`
	InventoryID            string             `bson:"s_inventory_id,omitempty"`
	LocationID             string             `bson:"s_location_id,omitempty"`
	Parent                 string             `bson:"s_parent_product,omitempty"`
	ParentID               string             `bson:"s_parent_product_id,omitempty"`
	SKU                    string             `bson:"sku,omitempty"`
	VariantID              string             `bson:"s_variant_id,omitempty"`
	VariantName            string             `bson:"s_variant_name,omitempty`
	EtsyProductID          int                `bson:"e_product_id,omitempty"`
	EtsyDescription        string             `bson:"e_description,omitempty"`
	EtsyProductTitle       string             `bson:"e_product_title,omitempty"`
	EtsyShopID             int                `bson:"e_shop_id,omitempty"`
	EtsyQuantity           int                `bson:"e_curr_stock"`
	EtsyPriorQuantity      int                `bson:"e_prev_stock"`
	EtsyItemInitialised    bool               `bson:"e_item_initialised"`
	OverrideStockRequested bool               `bson:"override_stock_requested"`
	OverrideStockLevel     int                `bson:"override_stock_level"`
}

type StockReconciliationDelta struct {
	EtsyDelta         map[int64]int  `json:"etsy_delta"`
	ShopifyDelta      map[string]int `json:"shopify_delta"`
	EstyHasChanges    bool           `json:"etsyhaschanges"`
	ShopifyHasChanges bool           `json:"shopifyhaschanges"`
}

func createKeyValuePairs(m primitive.M) string {
	b := new(bytes.Buffer)
	for key, value := range m {
		fmt.Fprintf(b, "%s=\"%v\"\n", key, value)
	}
	return b.String()
}

func getdatabases(client *mongo.Client) ([]string, error) {
	var dblist []string
	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	dblist, err := client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetDatabases",
		}).Warn(err)
		return dblist, err
	}
	return dblist, nil
}

func getstoretoken(storename string, client *mongo.Client) string {
	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	var doc bson.M
	collection := client.Database("etsync").Collection("shops")
	filter := bson.D{{"shopify_domain", storename}}
	if err := collection.FindOne(ctx, filter).Decode(&doc); err != nil {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetStoreToken",
		}).Warn(err)
		return ""
	}
	log.WithFields(log.Fields{
		"File":   "db_ops",
		"Caller": "GetStoreToken",
	}).Info("Got shop record from database for shopify token")
	return fmt.Sprintf("%v", doc["accessToken"])
}

func getetsytoken(config Config, client *mongo.Client) (etsytoken, error) {
	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	var token etsytoken
	collection := client.Database("etsync").Collection("shops")
	filter := bson.D{{"shopify_domain", *shopname}}
	if err := collection.FindOne(ctx, filter).Decode(&token); err != nil {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetEtsyToken",
		}).Warn(err)
		return etsytoken{}, err
	}
	log.WithFields(log.Fields{
		"File":   "db_ops",
		"Caller": "GetEtsyToken",
	}).Info("Got shop record from database for etsy token")

	if token.EtsyOnBoarded && (time.Now().Add(10 * time.Minute).Before(token.EtsyTokenExpires)) {
		// etsy has been onboarded & the etsy accesscode has not expired
		log.Info("Etsy token has greater than 10 minutes ttl, reusing current token")
		return token, nil
	} else {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetEtsyToken",
		}).Info("New Etsy token required, sending request to etsy API")
		rtoken, err := getEtsyTokenFromAPI(config.ETSY_CLIENT_ID, config.ETSY_REDIRECT_URI, token)
		if err != nil {
			return etsytoken{}, err
		}
		rtoken.EtsyOnBoarded = true
		rtoken.ShopifyDomain = *shopname // if this is a new token from etsy API then it won't have the shop
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetEtsyToken",
		}).Infof("Token retrieved from etsy api for %s with expiration %v", rtoken.ShopifyDomain, rtoken.EtsyTokenExpires)

		if err := writeEtsyToken(*shopname, rtoken, client); err != nil {
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "GetEtsyToken",
			}).Errorf("Unable to store the etsy token in database! %v", err)
			return etsytoken{}, err
		}
		token = rtoken
	}
	return token, nil

}
func getOverrides(storename string, client *mongo.Client) (map[string]int, error) {

	overrides := make(map[string]int)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	stockCollection := client.Database("etsync").Collection("stock")
	filter := bson.D{{"shopify_domain", storename}, {"override_stock_requested", true}}

	cursor, err := stockCollection.Find(ctx, filter)
	if err != nil {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetOverrides",
		}).Errorf("Error getting items with stock-overrider-requested %v", err)
		return overrides, err
	}
	for cursor.Next(ctx) {
		var elem StockItem
		err := cursor.Decode(&elem)
		if err != nil {
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "GetOverrides",
			}).Fatal(err)
		}
		overrides[elem.SKU] = elem.OverrideStockLevel
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetOverrides",
		}).Fatal(err)
	}
	cursor.Close(ctx)
	return overrides, nil
}

func getItemsToLink(storename string, client *mongo.Client) (map[int]string, error) {
	linkitems := make(map[int]string)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	stockCollection := client.Database("etsync").Collection("stock")
	filter := bson.D{{"shopify_domain", storename}, {"e_sku_sync_requested", true}}

	cursor, err := stockCollection.Find(ctx, filter)
	if err != nil {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetItemsToLink",
		}).Errorf("Error getting items with sku-set-requested %v ", err)
		return linkitems, err
	}
	for cursor.Next(ctx) {
		var elem StockItem
		err := cursor.Decode(&elem)
		if err != nil {
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "GetItemsToLink",
			}).Fatal(err)
		}
		linkitems[elem.EtsyProductID] = elem.SKU
	}

	if err := cursor.Err(); err != nil {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetItemsToLink",
		}).Fatal(err)
	}
	cursor.Close(ctx)
	return linkitems, nil
}

func getShopifyStockItem(storename, VariantId string, client *mongo.Client) (StockItem, error) {
	var item StockItem
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	stockCollection := client.Database("etsync").Collection("stock")
	filter := bson.D{{"shopify_domain", storename}, {"s_variant_id", VariantId}}

	if err := stockCollection.FindOne(ctx, filter).Decode(&item); err != nil {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetShopifyStockItem",
		}).Infof("Error writing Etsy shop details %v", err)
		return StockItem{}, err
	}
	return item, nil

}

func getShopifyStockItemBySku(storename, Sku string, client *mongo.Client) (StockItem, error) {
	var item StockItem
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	stockCollection := client.Database("etsync").Collection("stock")
	filter := bson.D{{"shopify_domain", storename}, {"sku", Sku}}

	if err := stockCollection.FindOne(ctx, filter).Decode(&item); err != nil {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "GetShopifyStockItemBySku",
		}).Infof("Error writing Etsy shop details %v", err)
		return StockItem{}, err
	}
	return item, nil

}

func writeEtsyToken(storename string, token etsytoken, client *mongo.Client) error {
	log.WithFields(log.Fields{
		"File":   "db_ops",
		"Caller": "WriteEtsyToken",
	}).Debug("Writing the etsy token to DB")
	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	shop_collection := client.Database("etsync").Collection("shops")
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
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "WriteEtsyToken",
		}).Debugf("No prior record found when inserting doc %s", result.Err())
		return result.Err()
	}
	log.WithFields(log.Fields{
		"File":   "db_ops",
		"Caller": "WriteEtsyToken",
	}).Debug("Success writing etsy token to Database")
	return nil
}

func saveEtsyShop(storename string, etsy_shop etsyShop, client *mongo.Client) error {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	shop_collection := client.Database("etsync").Collection("shops")
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
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "SaveEtsyShop",
		}).Debugf("Error writing Etsy shop details %s", result.Err())
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
func saveEtsyProducts(storename string, products []etsyProduct, eSkusToSet map[int]string, overrideStock map[string]int, client *mongo.Client) (StockReconciliationDelta, error) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	var stockdelta StockReconciliationDelta
	if len(overrideStock) > 0 {
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "SaveEtsyProducts",
		}).Debug("Stock override set so setting both Etsy & Shopify hasDelta flags")
		stockdelta.EstyHasChanges = true
		stockdelta.ShopifyHasChanges = true
	}
	etsyDelta := make(map[int64]int)
	shopifyDelta := make(map[string]int)
	var existingRecord StockItem
	stockCollection := client.Database("etsync").Collection("stock")
	for _, p := range products {
		var vdesc []string
		for _, pv := range p.PropertyValues {
			vstring := fmt.Sprintf("%s: %s", pv.PropertyName, strings.Join(pv.Values, "-"))
			vdesc = append(vdesc, vstring)
		}
		var skutoset string
		var override bool
		var setsku bool
		stockset := p.Offerings[0].Quantity
		if stockset, ok := overrideStock[p.Sku]; ok {
			// If we are setting the stock then there must be an existing db record
			// we need to set the current & previous value to the override value (in stockset)
			override = true
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "SaveEtsyProducts",
			}).Debugf("Override: Set the stock level for %d to %d", p.ProductID, stockset)
		}

		if s, ok := eSkusToSet[int(p.ProductID)]; ok {
			// we need to override setting the sku in the DB for this product
			skutoset = s
			setsku = true
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "SaveEtsyProducts",
			}).Debugf("Overriding the sku for %d to: %s", p.ProductID, skutoset)
		} else {
			skutoset = p.Sku
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "SaveEtsyProducts",
			}).Debugf("Sku for %d should be %s", p.ProductID, skutoset)
		}
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "SaveEtsyProducts",
		}).Debugf("Preparing update for %d with sku %s & quantity %d", p.ProductID, skutoset, stockset)
		updateRecord := bson.M{
			"shop_id":                 p.ShopID,
			"e_product_title":         p.Title,
			"e_description":           p.Description,
			"sku":                     skutoset,
			"shopify_domain":          p.ShopifyDomain,
			"e_curr_stock":            stockset,
			"e_product_id":            p.ProductID,
			"e_variation_description": strings.Join(vdesc, ", "),
		}
		log.WithFields(log.Fields{
			"File":       "db_ops",
			"Caller":     "SaveEtsyProducts",
			"Product_ID": p.ProductID,
			"Title":      p.Title,
			"Sku":        skutoset,
		}).Debug("Updating DB with Etsy product")
		var filter bson.M
		if skutoset != "" {
			filter = bson.M{"sku": skutoset, "shopify_domain": p.ShopifyDomain}
		} else {
			filter = bson.M{"e_product_id": p.ProductID, "shopify_domain": p.ShopifyDomain}
		}
		if err := stockCollection.FindOne(ctx, filter).Decode(&existingRecord); err != nil {
			log.WithFields(log.Fields{
				"File":           "db_ops",
				"Caller":         "SaveEtsyProducts",
				"etsy-ProductID": p.ProductID,
				"Response":       err,
			}).Debugf("Record not found for shopify item with this sku, initialising with current stock level %d", stockset)
			updateRecord["e_prev_stock"] = stockset
			updateRecord["e_item_exists"] = true
		} else {
			if override {
				// we need to override previous stock in the DB for this product
				updateRecord["e_prev_stock"] = stockset
				updateRecord["override_stock_requested"] = false
			} else {
				updateRecord["e_prev_stock"] = existingRecord.EtsyQuantity
			}
			if setsku {
				updateRecord["e_sku_sync_requested"] = false
			}
			if !existingRecord.EtsyItemInitialised {
				log.WithFields(log.Fields{
					"File":           "db_ops",
					"Caller":         "SaveEtsyProducts",
					"etsy-ProductID": p.ProductID,
					"Action":       "initialise etsy item",
				}).Debugf("Record found for new etsy item [sku %s], initialising with current stock level %d", updateRecord["sku"],  stockset)
				updateRecord["e_prev_stock"] = stockset
				updateRecord["e_item_initialised"] = true
			}
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "SaveEtsyProducts",
			}).Debugf("Loading existing record for %d: stock levels (prev->new) %d -> %d", p.ProductID, updateRecord["e_prev_stock"], p.Offerings[0].Quantity)
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "SaveEtsyProducts",
			}).Debug(createKeyValuePairs(updateRecord))
			if (existingRecord.Available != existingRecord.PriorAvailable) && !override {
				// we don't need to make changes to shopify if the stock is being overridden (those changes will be
				// handled seperately)
				log.WithFields(log.Fields{
					"File":           "db_ops",
					"Caller":         "SaveEtsyProducts",
					"Action":       "propogate shopify changes to etsy",
				}).Debugf("Record has changes in etsy current %d previous %d", existingRecord.Available, existingRecord.PriorAvailable)
				stockdelta.EstyHasChanges = true
				etsyDelta[p.ProductID] = (existingRecord.Available - existingRecord.PriorAvailable)
			}

			if (updateRecord["e_curr_stock"] != updateRecord["e_prev_stock"]) && !override {
				// we don't need to make changes to etsy if the stock is being overridden (those changes will be
				// handled seperately)
				log.WithFields(log.Fields{
					"File":           "db_ops",
					"Caller":         "SaveEtsyProducts",
					"Action":       "propogate shopify changes to etsy",
				}).Debugf("Record has changes in etsy current %d previous %d", p.Offerings[0].Quantity, existingRecord.EtsyQuantity)
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

	log.WithFields(log.Fields{
		"File":   "db_ops",
		"Caller": "SaveEtsyProducts",
	}).Infof("Success writing %d etsy listing details to Database", len(products))
	return stockdelta, nil
}

func setEtsyStockLevelForProducts(storename string, products []EtsyProductUpdate, client *mongo.Client) error {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	stockCollection := client.Database("etsync").Collection("stock")
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
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "SetEtsyStockLevelForProducts",
			}).Debugf("No prior record found when inserting doc %s", result.Err())
			return result.Err()
		}
	}
	return nil
}

func setShopifyStockLevelForVariant(storename, VariantId string, stocklevel int, client *mongo.Client) error {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	stockCollection := client.Database("etsync").Collection("stock")

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
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "SetEtsyStockLevelForProducts",
		}).Debugf("No prior record found when inserting doc %s", result.Err())
		return result.Err()
	}
	return nil
}

func setshopstock(storename string, items []StockItem, client *mongo.Client) error {

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	stockCollection := client.Database("etsync").Collection("stock")
	for _, item := range items {
		var existingRecord StockItem
		var update bson.M
		itemtype := item.ItemType
		item.ItemType = "shopify-stock-level"
		filter := bson.M{"s_inventory_id": item.InventoryID}
		if itemtype == "inventory" {
			if err := stockCollection.FindOne(ctx, filter).Decode(&existingRecord); err != nil {
				log.WithFields(log.Fields{
					"File":     "db_ops",
					"Caller":   "SetShopStock",
					"ID":       item.InventoryID,
					"Response": err,
				}).Debugf("Record not found, initialising with current stock level %d", item.Available)
				item.PriorAvailable = item.Available
				item.EtsyItemInitialised = false
			} else {
				item.PriorAvailable = existingRecord.Available
				log.WithFields(log.Fields{
					"File":   "db_ops",
					"Caller": "SetShopStock",
				}).Debugf("Loading existing record for %s: stock levels (prev->new) %d -> %d", item.InventoryID, item.PriorAvailable, item.Available)
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
			"File":   "db_ops",
			"Caller": "SetShopStock",
			"ID":     item.InventoryID,
		}).Debug("Updating Database")

		opts := options.FindOneAndUpdate().SetUpsert(true)

		result := stockCollection.FindOneAndUpdate(ctx, filter, update, opts)
		if result.Err() != nil {
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "SetShopStock",
			}).Debugf("No prior record found when inserting doc %s", result.Err())
			continue
		}
		doc := bson.M{}
		if err := result.Decode(&doc); err != nil {
			log.WithFields(log.Fields{
				"File":   "db_ops",
				"Caller": "SetShopStock",
			}).Errorf("Problem decoding record for %s", item.InventoryID)
		}
		log.WithFields(log.Fields{
			"File":   "db_ops",
			"Caller": "SetShopStock",
			"Kind":   itemtype,
		}).Debugf("Upserted Doc %s", item.InventoryID)
	}

	return nil
}

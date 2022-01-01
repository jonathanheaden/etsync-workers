package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

type BulkRequest struct {
	Data struct {
		BulkOperationRunQuery struct {
			BulkOperation struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"bulkOperation"`
			UserErrors []string `json:"userErrors"`
		} `json:"bulkOperationRunQuery"`
	} `json:"data"`
}

type BulkRequestStatus struct {
	Data struct {
		CurrentBulkOperation struct {
			ID             string      `json:"id"`
			Status         string      `json:"status"`
			ErrorCode      interface{} `json:"errorCode"`
			CreatedAt      time.Time   `json:"createdAt"`
			CompletedAt    time.Time   `json:"completedAt"`
			ObjectCount    string      `json:"objectCount"`
			FileSize       string      `json:"fileSize"`
			URL            string      `json:"url"`
			PartialDataURL interface{} `json:"partialDataUrl"`
		} `json:"currentBulkOperation"`
	} `json:"data"`
	Extensions struct {
		Cost struct {
			RequestedQueryCost int `json:"requestedQueryCost"`
			ActualQueryCost    int `json:"actualQueryCost"`
			ThrottleStatus     struct {
				MaximumAvailable   float64 `json:"maximumAvailable"`
				CurrentlyAvailable int     `json:"currentlyAvailable"`
				RestoreRate        float64 `json:"restoreRate"`
			} `json:"throttleStatus"`
		} `json:"cost"`
	} `json:"extensions"`
}

type ProductVariant struct {
	DisplayName         string `json:"displayName"`
	ID                  string `json:"id"`
	InventoryManagement string `json:"inventoryManagement"`
	InventoryItem       struct {
		ID string `json:"id"`
	} `json:"inventoryItem"`
	Product  Product `json:"product"`
	ParentID string  `json:"__parentId"`
	Sku      string  `json:"sku`
}

type InventoryLevel struct {
	Location struct {
		Address struct {
			Address1 string      `json:"address1"`
			Address2 interface{} `json:"address2"`
			City     string      `json:"city"`
			Country  string      `json:"country"`
		} `json:"address"`
		ID string `json:"id"`
	} `json:"location"`
	Available   int       `json:"available"`
	ID          string    `json:"id"`
	UpdatedAt   time.Time `json:"updatedAt"`
	InventoryID string    `json:"__parentId"`
}

type Product struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func registerbulkquery(storeurl, token, query string) (string, error) {
	var response BulkRequest
	url := fmt.Sprintf("https://%s/admin/api/2021-01/graphql.json", storeurl)
	log.WithFields(log.Fields{
		"File":   "shopify_ops",
		"Caller": "RegisterBulkQuery",
	}).Debugf("sending request to %s", url)
	method := "POST"
	payload := strings.NewReader(query)
	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "RegisterBulkQuery",
		}).Errorf("Error with http client: %v", err)
		return "", err
	}
	req.Header.Add("X-Shopify-Access-Token", token)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "RegisterBulkQuery",
			"Action": "make http request",
		}).Errorf("Error with http client request action: %v", err)
		return "", err
	}
	log.WithFields(log.Fields{
		"File":   "shopify_ops",
		"Caller": "RegisterBulkQuery",
	}).Infof("Request to graphql - response code %d", res.StatusCode)

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "RegisterBulkQuery",
		}).Errorf("Error reading response body: %v", err)
		return "", err
	}

	if err := json.Unmarshal(body, &response); err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "RegisterBulkQuery",
			"Action": "unmarshall",
		}).Errorf("Error with response unmarshall: %v", err)
		return "", err
	}

	log.WithFields(log.Fields{
		"File":   "shopify_ops",
		"Caller": "RegisterBulkQuery",
		"Status": response.Data.BulkOperationRunQuery.BulkOperation.Status,
		"ID":     response.Data.BulkOperationRunQuery.BulkOperation.ID,
	}).Debug("Response from Shopify Register Bulk Query operation")

	if response.Data.BulkOperationRunQuery.BulkOperation.Status == "CREATED" {
		return response.Data.BulkOperationRunQuery.BulkOperation.ID, nil
	} else {
		errstring := strings.Join(response.Data.BulkOperationRunQuery.UserErrors, "\n")
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "RegisterBulkQuery",
		}).Warnf("Errors contained in response: %v", err)
		return "", fmt.Errorf("%s", errstring)
	}
}

func getBulkRequestStatus(storeurl, token string) (string, string, error) {
	var response BulkRequestStatus
	url := fmt.Sprintf("https://%s/admin/api/2021-01/graphql.json", storeurl)
	method := "POST"

	payload := strings.NewReader("{\"query\":\"query {\\n  currentBulkOperation {\\n    id\\n    status\\n    errorCode\\n    createdAt\\n    completedAt\\n    objectCount\\n    fileSize\\n    url\\n    partialDataUrl\\n  }\\n}\\n\",\"variables\":{}}")

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "GetBulkRequestStatus",
		}).Errorf("Error with http client: %v", err)
		return "", "", err
	}
	req.Header.Add("X-Shopify-Access-Token", token)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return "", "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "GetBulkRequestStatus",
			"Action": "Read response",
		}).Errorf("Error reading response body: %v", err)
		return "", "", err
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", "", err
	}
	return response.Data.CurrentBulkOperation.Status, response.Data.CurrentBulkOperation.URL, nil
}

func getproductvariants(storeurl, token string) (string, error) {
	query := "{\"query\":\"mutation {\\n  bulkOperationRunQuery(\\n   query: \\\"\\\"\\\"\\n    {\\n      products {\\n        edges {\\n          node {\\n            id,\\n            variants {\\n              edges {\\n                node {\\n                  displayName,\\n                  id,\\n                  inventoryManagement,\\n                  inventoryItem  {\\n                     id\\n                  },\\n                  sku,\\n                  product {\\n                    id,\\n                    title\\n                  }\\n                }\\n              }\\n            }\\n          }\\n        }\\n      }\\n    }\\n    \\\"\\\"\\\"\\n  ) {\\n    bulkOperation {\\n      id\\n      status\\n    }\\n    userErrors {\\n      field\\n      message\\n    }\\n  }\\n}\",\"variables\":{}}"
	_, err := registerbulkquery(storeurl, token, query)
	if err != nil {
		return "", err
	}
	var url string
	attempt := 1
	statusurl := "PENDING"
	for statusurl != "COMPLETED" {
		statusurl, url, err = getBulkRequestStatus(storeurl, token)
		if err != nil {
			return "", err
		}
		log.WithFields(log.Fields{
			"File":    "shopify_ops",
			"Caller":  "GetProductVariants",
			"Status":  statusurl,
			"retries": attempt,
		}).Debug("Awaiting graphql response")
		// if the statusurl is returned then we should break out before the timed sleep
		attempt = attempt + 1
		if attempt > 12 {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "GetProductVariants",
			}).Error("Exiting function as 4 minutes have expired")
			return "", fmt.Errorf("Query exceeded 4 minute timeout")
		}
		if statusurl != "COMPLETED" {
			time.Sleep(20 * time.Second)
		}
	}
	return url, nil
}

func getinventorylevels(storeurl, token string) (string, error) {
	query := "{\"query\":\"mutation {\\n  bulkOperationRunQuery(\\n   query: \\\"\\\"\\\"\\n   {\\n  inventoryItems {\\n    edges {\\n      node {\\n        id\\n        inventoryLevels {\\n          edges {\\n            node {\\n              location {\\n                address {\\n                  address1\\n                  address2\\n                  city\\n                  country\\n                }\\n                id\\n              }\\n              available\\n              id\\n              updatedAt\\n            }\\n\\n          }\\n        }\\n      }\\n    }\\n  }\\n  }\\n    \\\"\\\"\\\"\\n  ) {\\n    bulkOperation {\\n      id\\n      status\\n    }\\n    userErrors {\\n      field\\n      message\\n    }\\n  }\\n}\",\"variables\":{}}"
	_, err := registerbulkquery(storeurl, token, query)
	if err != nil {
		return "", err
	}
	var url string
	attempt := 1
	statusurl := "PENDING"
	for statusurl != "COMPLETED" {
		statusurl, url, err = getBulkRequestStatus(storeurl, token)
		if err != nil {
			return "", err
		}
		log.WithFields(log.Fields{
			"File":    "shopify_ops",
			"Caller":  "GetInventoryLevels",
			"Status":  statusurl,
			"retries": attempt,
		}).Debug("Awaiting graphql response")
		// if the statusurl is returned then we should break out before the timed sleep
		attempt = attempt + 1
		if attempt > 12 {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "GetInventoryLevels",
			}).Error("Exiting function as 4 minutes have expired")
			return "", fmt.Errorf("Query exceeded 4 minute timeout")
		}
		if statusurl != "COMPLETED" {
			time.Sleep(20 * time.Second)
		}
	}
	return url, nil
}

func processinventorylevels(url, storename string, client *mongo.Client) error {

	log.Debugf("Started processing inventory list for %s", storename)
	var Items []StockItem

	response, err := http.Get(url)

	if err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "ProcessInventoryLevels",
		}).Errorf("Error reading products: %v", err)
		return err
	}

	defer response.Body.Close()

	scanner := bufio.NewScanner(response.Body)

	for scanner.Scan() {
		var inventorylevel InventoryLevel
		var avail int

		if err := json.Unmarshal(scanner.Bytes(), &inventorylevel); err != nil {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "ProcessInventoryLevels",
				"Err":    err,
			}).Warn("Problem scanning line")
			continue
		}
		if inventorylevel.Location.ID != "" {
			avail = inventorylevel.Available
			log.WithFields(log.Fields{
				"File":        "shopify_ops",
				"Caller":      "ProcessInventoryLevels",
				"ID":          inventorylevel.InventoryID,
				"Location":    inventorylevel.Location.ID,
				"Stock level": avail,
			}).Debug("Processing inventory item")
			item := StockItem{
				ItemType:    "inventory",
				InventoryID: inventorylevel.InventoryID,
				LocationID:  inventorylevel.Location.ID,
				Available:   avail,
			}
			Items = append(Items, item)
		}
	}
	if err := scanner.Err(); err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "ProcessInventoryLevels",
		}).Errorf("Error reading input:", err)
	}
	if err := setshopstock(storename, Items, client); err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "ProcessInventoryLevels",
		}).Errorf("Error with DB upsert %v", err)
		return err
	}
	log.WithFields(log.Fields{
		"File":   "shopify_ops",
		"Caller": "ProcessInventoryLevels",
	}).Debugf("Writing %d inventory levels to DB", len(Items))
	return nil
}

func processproductlevels(url, storename string, client *mongo.Client) error {
	log.WithFields(log.Fields{
		"File":   "shopify_ops",
		"Caller": "ProcessProductLevels",
	}).Infof("Started processing inventory list for %s", storename)
	var Items []StockItem

	response, err := http.Get(url)

	if err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "ProcessInventoryLevels",
		}).Errorf("Error reading products: %v", err)
		return err
	}

	defer response.Body.Close()

	scanner := bufio.NewScanner(response.Body)
	count := 0
	for scanner.Scan() {
		var productvariant ProductVariant
		count++
		if err := json.Unmarshal(scanner.Bytes(), &productvariant); err != nil {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "ProcessInventoryLevels",
			}).Warn("Problem scanning line")
			continue
		}
		if productvariant.InventoryManagement == "SHOPIFY" {
			item := StockItem{
				InventoryID: productvariant.InventoryItem.ID,
				ItemType:    "productvariant",
				VariantName: productvariant.DisplayName,
				VariantID:   productvariant.ID,
				Parent:      productvariant.Product.Title,
				ParentID:    productvariant.Product.ID,
				SKU:         productvariant.Sku,
			}
			Items = append(Items, item)
		}
	}
	if err := scanner.Err(); err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "ProcessInventoryLevels",
		}).Errorf("Error reading input:", err)
	}
	if err := setshopstock(storename, Items, client); err != nil {
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "ProcessInventoryLevels",
		}).Errorf("Error with DB upsert %v", err)
		return err
	}
	log.WithFields(log.Fields{
		"File":   "shopify_ops",
		"Caller": "ProcessInventoryLevels",
	}).Debugf("Writing %d products to DB", len(Items))
	return nil
}

func reconcileShopifyStockLevel(storename, clientid, token string, delta StockReconciliationDelta, overrideStock map[string]int, client *mongo.Client) error {
	log.Debugf("Setting Shopify stock:delta [%v] overrides [%v]", delta.ShopifyDelta, overrideStock)
	url := fmt.Sprintf("https://%s/admin/api/2020-10/inventory_levels/set.json", storename)
	method := "POST"
	overridesprocessed := make(map[string]bool)
	for k, v := range delta.ShopifyDelta {

		var newstock int
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "ReconcileShopifyStockLevel",
		}).Debugf("Update stock for %s by %d", k, v)
		item, err := getShopifyStockItem(storename, k, client)
		if err != nil {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "ReconcileShopifyStockLevel",
			}).Warnf("Error getting record for %s from DB %v", k, err)
		}
		loc := item.LocationID[strings.LastIndex(item.LocationID, "/")+1:]
		i := item.InventoryID[strings.LastIndex(item.InventoryID, "/")+1:]
		if stockset, ok := overrideStock[item.SKU]; ok {
			newstock = stockset
			overridesprocessed[item.SKU] = true
		} else {
			newstock = item.Available + v
			if newstock < 0 {
				newstock = 0
			}
		}
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "ReconcileShopifyStockLevel",
		}).Debugf("Updating shopify for item sku %s new stock %d", item.SKU, newstock)
		payload := strings.NewReader(fmt.Sprintf("location_id=%s&inventory_item_id=%s&available=%d", loc, i, newstock))

		httpclient := &http.Client{}
		req, err := http.NewRequest(method, url, payload)

		if err != nil {
			log.Error(err)
			continue
		}
		req.Header.Add("X-Shopify-Access-Token", token)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		res, err := httpclient.Do(req)
		if err != nil {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "ReconcileShopifyStockLevel",
				"Action": "http request",
			}).Error(err)
			continue
		}
		if res.StatusCode != 200 {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "ReconcileShopifyStockLevel",
				"Action": "http response",
			}).Errorf("Unable to set Shopify stock level in API for %s, Got response %d", k, res.StatusCode)
		}
		if err = setShopifyStockLevelForVariant(storename, k, newstock, client); err != nil {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "ReconcileShopifyStockLevel",
				"Action": "setShopifyStockLevelForVariant",
			}).Error(err)
		}

	}
	// need to handle cases where the override is set but that sku is not in the regular stock delta
	for k, v := range overrideStock {
		if overridesprocessed[k] {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "ReconcileShopifyStockLevel",
			}).Debugf("Skipping set shopify stock for %s as already processed", k)
			continue
		}
		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "ReconcileShopifyStockLevel",
		}).Debugf("Force set shopify stock for %s as requested via app", k)
		item, err := getShopifyStockItemBySku(storename, k, client)
		if err != nil {
			log.Warnf("Error getting record for %s from DB %v", k, err)
		}
		loc := item.LocationID[strings.LastIndex(item.LocationID, "/")+1:]
		i := item.InventoryID[strings.LastIndex(item.InventoryID, "/")+1:]

		log.WithFields(log.Fields{
			"File":   "shopify_ops",
			"Caller": "ReconcileShopifyStockLevel",
		}).Debugf("Updating shopify for item sku %s new stock %d", k, v)
		payload := strings.NewReader(fmt.Sprintf("location_id=%s&inventory_item_id=%s&available=%d", loc, i, v))

		httpclient := &http.Client{}
		req, err := http.NewRequest(method, url, payload)

		if err != nil {
			log.Error(err)
			continue
		}
		req.Header.Add("X-Shopify-Access-Token", token)
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		res, err := httpclient.Do(req)
		if err != nil {
			log.Error(err)
			continue
		}
		if res.StatusCode != 200 {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "ReconcileShopifyStockLevel",
			}).Errorf("Unable to set Shopify stock level in API for %s, Got response %d", k, res.StatusCode)
		}
		if err = setShopifyStockLevelForVariant(storename, item.VariantID, v, client); err != nil {
			log.WithFields(log.Fields{
				"File":   "shopify_ops",
				"Caller": "ReconcileShopifyStockLevel",
			}).Error(err)

		}

	}
	return nil
}

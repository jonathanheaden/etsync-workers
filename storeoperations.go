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
	"go.mongodb.org/mongo-driver/bson/primitive"
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

type ShopifyItem struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	ItemType    string             `bson:"itemtype"`
	Available   int                `bson:"s_curr_stock,omitempty"`
	InventoryID string             `bson:"s_inventory_id,omitempty"`
	LocationID  string             `bson:"s_location_id,omitempty"`
	Parent      string             `bson:"s_parent_product,omitempty"`
	ParentID    string             `bson:"s_parent_product_id,omitempty"`
	SKU         string             `bson:"sku,omitempty"`
	VariantID   string             `bson:"s_variant_id,omitempty"`
	VariantName string             `bson:"s_variant_name,omitempty`
}

type Product struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func registerbulkquery(storeurl, token, query string) (string, error) {
	var response BulkRequest
	url := fmt.Sprintf("https://%s/admin/api/2021-01/graphql.json", storeurl)
	log.Info(fmt.Sprintf("sending request to %s", url))
	method := "POST"
	payload := strings.NewReader(query)
	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		log.Errorf("Error with http client: %v", err)
		return "", err
	}
	req.Header.Add("X-Shopify-Access-Token", token)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		log.Errorf("Error with http client request action: %v", err)
		return "", err
	}
	log.Info(fmt.Sprintf("response code %d", res.StatusCode))

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("Error reading response body: %v", err)
		return "", err
	}

	if err := json.Unmarshal(body, &response); err != nil {
		log.Errorf("Error with response unmarshall: %v", err)
		return "", err
	}

	log.WithFields(log.Fields{
		"Status": response.Data.BulkOperationRunQuery.BulkOperation.Status,
		"ID":     response.Data.BulkOperationRunQuery.BulkOperation.ID,
	}).Info("Response from Shopify Register Bulk Query operation")

	if response.Data.BulkOperationRunQuery.BulkOperation.Status == "CREATED" {
		return response.Data.BulkOperationRunQuery.BulkOperation.ID, nil
	} else {
		errstring := strings.Join(response.Data.BulkOperationRunQuery.UserErrors, "\n")
		log.Warnf("Errors contained in response: %v", err)
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
		log.Errorf("Error with http client: %v", err)
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
		log.Errorf("Error reading response body: %v", err)
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
			"Status":  statusurl,
			"retries": attempt,
		}).Info()
		// if the statusurl is returned then we should break out before the timed sleep
		attempt = attempt + 1
		if attempt > 12 {
			log.Error("Exiting function as 4 minutes have expired")
			return "", fmt.Errorf("Query exceeded 4 minute timeout")
		}
		time.Sleep(20 * time.Second)
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
			"Status":  statusurl,
			"retries": attempt,
		}).Info()
		// if the statusurl is returned then we should break out before the timed sleep
		attempt = attempt + 1
		if attempt > 12 {
			log.Error("Exiting function as 4 minutes have expired")
			return "", fmt.Errorf("Query exceeded 4 minute timeout")
		}
		if statusurl != "COMPLETED" {
			time.Sleep(20 * time.Second)
		}
	}
	return url, nil
}

func processinventorylevels(url, storename string, client *mongo.Client) error {

	log.Info(fmt.Sprintf("Started processing inventory list for %s", storename))
	var Items []ShopifyItem
	var inventorylevel InventoryLevel

	response, err := http.Get(url) //use package "net/http"

	if err != nil {
		log.Errorf("Error reading products: %v", err)
		return err
	}

	defer response.Body.Close()

	scanner := bufio.NewScanner(response.Body)
	count := 0
	for scanner.Scan() {
		count++
		log.Debug(fmt.Sprintf("Processing inventory file line %d", count))
		if err := json.Unmarshal(scanner.Bytes(), &inventorylevel); err != nil {
			log.Warn("Problem scanning line")
			continue
		}
		if len(inventorylevel.InventoryID) > 0 {
			item := ShopifyItem{
				ItemType:    "inventory",
				InventoryID: inventorylevel.InventoryID,
				LocationID:  inventorylevel.Location.ID,
				Available:   inventorylevel.Available,
			}
			Items = append(Items, item)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Errorf("Error reading input:", err)
	}
	if err := setshopstock(storename, Items, client); err != nil {
		log.Errorf("Error with DB upsert %v", err)
		return err
	}
	log.Info(fmt.Sprintf("Writing %d inventory levels to DB", len(Items)))
	return nil
}

func processproductlevels(url, storename string, client *mongo.Client) error {
	log.Info(fmt.Sprintf("Started processing inventory list for %s", storename))
	var Items []ShopifyItem
	var productvariant ProductVariant

	response, err := http.Get(url) //use package "net/http"

	if err != nil {
		log.Errorf("Error reading products: %v", err)
		return err
	}

	defer response.Body.Close()

	scanner := bufio.NewScanner(response.Body)
	count := 0
	for scanner.Scan() {
		count++
		log.Debug(fmt.Sprintf("Processing products file line %d", count))
		if err := json.Unmarshal(scanner.Bytes(), &productvariant); err != nil {
			log.Warn("Problem scanning line")
			continue
		}
		if productvariant.InventoryManagement == "SHOPIFY" {
			item := ShopifyItem{
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
		log.Errorf("Error reading input:", err)
	}
	if err := setshopstock(storename, Items, client); err != nil {
		log.Errorf("Error with DB upsert %v", err)
		return err
	}
	log.Info(fmt.Sprintf("Writing %d products to DB", len(Items)))
	return nil
}

// NOTE - should create a new object for each new shopify item to make sure there is no stale data

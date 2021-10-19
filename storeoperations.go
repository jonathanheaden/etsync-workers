package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
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


func registerbulkquery(storeurl, token string) (string, error) {
	var response BulkRequest 
	url := fmt.Sprintf("https://%s/admin/api/2021-01/graphql.json", storeurl)
	method := "POST"

	payload := strings.NewReader("{\"query\":\"mutation {\\n  bulkOperationRunQuery(\\n   query: \\\"\\\"\\\"\\n   {\\n  inventoryItems {\\n    edges {\\n      node {\\n        id\\n        inventoryLevels {\\n          edges {\\n            node {\\n              location {\\n                address {\\n                  address1\\n                  address2\\n                  city\\n                  country\\n                }\\n                id\\n              }\\n              available\\n              id\\n              updatedAt\\n            }\\n\\n          }\\n        }\\n      }\\n    }\\n  }\\n  }\\n    \\\"\\\"\\\"\\n  ) {\\n    bulkOperation {\\n      id\\n      status\\n    }\\n    userErrors {\\n      field\\n      message\\n    }\\n  }\\n}\",\"variables\":{}}")

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return "",err
	}
	req.Header.Add("X-Shopify-Access-Token", token)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return "",err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return "",err
	}

	if err := json.Unmarshal(body, &response); err != nil {
        return "",err
    }

	if response.Data.BulkOperationRunQuery.BulkOperation.Status == "CREATED" {
		return response.Data.BulkOperationRunQuery.BulkOperation.ID, nil
	} else {
		errstring := strings.Join(response.Data.BulkOperationRunQuery.UserErrors,"\n")
		return "", fmt.Errorf("%s",errstring)
	}
}

func getBulkRequestStatus(storeurl, token string){
	var response BulkRequestStatus
	url := fmt.Sprintf("https://%s/admin/api/2021-01/graphql.json", storeurl)
	method := "POST"
  
	payload := strings.NewReader("{\"query\":\"query {\\n  currentBulkOperation {\\n    id\\n    status\\n    errorCode\\n    createdAt\\n    completedAt\\n    objectCount\\n    fileSize\\n    url\\n    partialDataUrl\\n  }\\n}\\n\",\"variables\":{}}")
  
	client := &http.Client {
	}
	req, err := http.NewRequest(method, url, payload)
  
	if err != nil {
	  fmt.Println(err)
	  return
	}
	req.Header.Add("X-Shopify-Access-Token", token)
	req.Header.Add("Content-Type", "application/json")
  
	res, err := client.Do(req)
	if err != nil {
	  fmt.Println(err)
	  return
	}
	defer res.Body.Close()
  
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
	  fmt.Println(err)
	  return
	}
	if err := json.Unmarshal(body, &response); err != nil {
        return 
    }
	fmt.Println(response.Data.CurrentBulkOperation.Status)
}
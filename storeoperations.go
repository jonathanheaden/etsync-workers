package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func registerbulkquery(storeurl, token string) {
	url := fmt.Sprintf("https://%s/admin/api/2021-01/graphql.json", storeurl)
	method := "POST"

	payload := strings.NewReader("{\"query\":\"mutation {\\n  bulkOperationRunQuery(\\n   query: \\\"\\\"\\\"\\n   {\\n  inventoryItems {\\n    edges {\\n      node {\\n        id\\n        inventoryLevels {\\n          edges {\\n            node {\\n              location {\\n                address {\\n                  address1\\n                  address2\\n                  city\\n                  country\\n                }\\n                id\\n              }\\n              available\\n              id\\n              updatedAt\\n            }\\n\\n          }\\n        }\\n      }\\n    }\\n  }\\n  }\\n    \\\"\\\"\\\"\\n  ) {\\n    bulkOperation {\\n      id\\n      status\\n    }\\n    userErrors {\\n      field\\n      message\\n    }\\n  }\\n}\",\"variables\":{}}")

	client := &http.Client{}
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
	fmt.Println(string(body))
}

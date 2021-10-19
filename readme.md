# Sync Worker
Run offline tasks to poll shopify and etsy stores for shop data, current stock levels & changes to stock level.

## Overview:
Run graphql operations to
- get a list of all products and their corresponding variants
- get a list of all inventory levels for each sku
- record the results in the database
Use the etsy api to
- get a new auth token and store record for next auth token in database
- get the product listings for etsy
- get the inventory levels for etsy
- record the results to the database

- compare both sides to the previous level and apply any changes to the other store

## main.go
Loads config and parses options

## dboperations.go
Connect to and manipulate the crud functions for database

## util.go
Loads the config

## storeoperations.go
single library to handle operations for both shopify and etsy stores
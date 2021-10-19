# Sync Worker
Run offline tasks to poll shopify and etsy stores for shop data, current stock levels & changes to stock level.

## main.go
Loads config and parses options

## dboperations.go
Connect to and manipulate the crud functions for database

## storeoperations.go
single library to handle operations for both shopify and etsy stores
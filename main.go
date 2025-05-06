package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/dgraph-io/badger/v4"
	"github.com/gin-gonic/gin"
)

type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Quantity    int     `json:"quantity"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
}

var db *badger.DB

func initDB() {
	var err error
	opts := badger.DefaultOptions("./data")
	opts.Logger = nil
	db, err = badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	initDB()
	defer db.Close()

	r := gin.Default()

	// Enable CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})

	// Routes
	r.POST("/products", createProduct)
	r.GET("/products", getProducts)

	r.Run(":4433")
}

func createProduct(c *gin.Context) {
	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get next ID
	var nextID int
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("next_id"))
		if err == badger.ErrKeyNotFound {
			nextID = 1
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			nextID, err = strconv.Atoi(string(val))
			return err
		})
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	product.ID = nextID

	// Save product
	err = db.Update(func(txn *badger.Txn) error {
		// Save product
		productData, err := json.Marshal(product)
		if err != nil {
			return err
		}
		err = txn.Set([]byte(fmt.Sprintf("product:%d", product.ID)), productData)
		if err != nil {
			return err
		}

		// Update next ID
		return txn.Set([]byte("next_id"), []byte(strconv.Itoa(nextID+1)))
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, product)
}

func getProducts(c *gin.Context) {
	var products []Product

	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("product:")
		iterator := txn.NewIterator(opts)
		defer iterator.Close()

		for iterator.Rewind(); iterator.Valid(); iterator.Next() {
			item := iterator.Item()
			err := item.Value(func(val []byte) error {
				var product Product
				if err := json.Unmarshal(val, &product); err != nil {
					return err
				}
				products = append(products, product)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, products)
} 
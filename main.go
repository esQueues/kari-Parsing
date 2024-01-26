package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	client           *mongo.Client
	shoesCollection  *mongo.Collection
	shoesCollectionM sync.Mutex
	wg               sync.WaitGroup
)

func init() {
	// Initialize MongoDB
	initMongoDB()
}

func initMongoDB() {
	// Create a MongoDB client
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}

	// Connect to MongoDB
	err = client.Connect(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Create a MongoDB collection
	shoesCollection = client.Database("shoe_shop").Collection("shoes")
}

func main() {
	c := colly.NewCollector()

	c.OnHTML(".css-f4s3gt", func(e *colly.HTMLElement) {
		wg.Add(1) // Increment the wait group

		go func() {
			defer wg.Done() // Decrement the wait group when the goroutine finishes

			brand := e.ChildText(".css-wfg91f span")
			name := e.ChildText(".aqa-item-name")
			originalPriceStr := strings.TrimRight(e.ChildText(".css-1hu3vxw"), " ₽")
			discountStr := e.ChildText(".css-1yjzpb2")
			discountedPriceStr := strings.TrimRight(e.ChildText(".css-1xczz6l"), " ₽")
			reviewsCount := e.ChildText(".reviews-count")

			originalPrice, err := parsePrice(originalPriceStr)
			if err != nil {
				log.Printf("Error parsing original price: %v", err)
				return
			}

			discountInt, err := parseDiscountToInt(discountStr)
			if err != nil {
				log.Printf("Error parsing discount: %v", err)
				return
			}
			discountedPrice, err := parsePrice(discountedPriceStr)
			if err != nil {
				log.Printf("Error parsing discounted price: %v", err)
				return
			}

			reviewsCountInt, err := parseReviewsCountToInt(reviewsCount)
			if err != nil {
				log.Printf("Error parsing reviews count: %v", err)
				return
			}
			var promotions []string
			e.ForEach(".e1i0l88z7", func(i int, el *colly.HTMLElement) {
				promotion := el.Text
				promotions = append(promotions, promotion)
			})

			fmt.Printf("Brand: %s\n", brand)
			fmt.Printf("Name: %s\n", name)
			fmt.Printf("Original Price: %d\n", originalPrice)
			fmt.Printf("Discount: %d\n", discountInt)
			fmt.Printf("Discounted Price: %d\n", discountedPrice)
			fmt.Printf("Reviews Count: %d\n", reviewsCountInt)
			fmt.Printf("Promotions: %v\n", promotions)

			// Insert the data into the MongoDB collection
			shoesCollectionM.Lock()
			_, err = shoesCollection.InsertOne(context.TODO(), map[string]interface{}{
				"brand": brand,
				"name":  name,
				"price": map[string]interface{}{
					"original_price":   originalPrice,
					"discount":         discountStr,
					"discounted_price": discountedPrice,
				},
				"reviews_count": reviewsCount,
				"promotions":    promotions,
			})
			shoesCollectionM.Unlock()

			if err != nil {
				log.Println("Error inserting data into MongoDB:", err)
			}
		}()
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	totalPages := 172

	// Start scraping
	for page := 1; page <= totalPages; page++ {
		url := fmt.Sprintf("https://kari.com/catalog/muzhchinam/muzhskaya-obuv/?page=%d", page)
		err := c.Visit(url)
		if err != nil {
			log.Println("Error visiting URL:", err)
		}
		// Add a delay between requests to avoid rate-limiting
		time.Sleep(2 * time.Second)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Disconnect from MongoDB
	client.Disconnect(context.Background())
}
func parseDiscountToInt(discountStr string) (int, error) {
	// Remove '%' and trim spaces
	discountStr = strings.TrimSpace(strings.TrimRight(discountStr, "%"))

	// Parse the discount string to an integer
	discountInt, err := strconv.Atoi(discountStr)
	if err != nil {
		return 0, err
	}

	return discountInt, nil
}
func parsePrice(priceStr string) (int, error) {
	// Remove commas and non-breaking spaces from the price string
	cleanedPriceStr := strings.ReplaceAll(priceStr, ",", "")
	cleanedPriceStr = strings.ReplaceAll(cleanedPriceStr, "\u00a0", "")

	// Trim leading and trailing spaces
	cleanedPriceStr = strings.TrimSpace(cleanedPriceStr)

	// Parse the cleaned string to an integer
	price, err := strconv.Atoi(cleanedPriceStr)
	if err != nil {
		return 0, err
	}

	return price, nil
}
func parseReviewsCountToInt(reviewsCountStr string) (int, error) {
	// Remove any non-numeric characters and trim spaces
	reviewsCountStr = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, reviewsCountStr))

	// Parse the reviews count string to an integer
	reviewsCountInt, err := strconv.Atoi(reviewsCountStr)
	if err != nil {
		return 0, err
	}

	return reviewsCountInt, nil
}

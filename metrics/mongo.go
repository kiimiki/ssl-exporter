package metrics

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// getMongoURI builds connection string using secrets and MONGO_HOST env
func getMongoURI() string {
	user, err1 := os.ReadFile("/run/secrets/mongo_user")
	pass, err2 := os.ReadFile("/run/secrets/mongo_password")
	host := os.Getenv("MONGO_HOST")

	if err1 != nil || err2 != nil || host == "" {
		log.Println("[WARN] Could not read Mongo secrets or host")
		return ""
	}

	return fmt.Sprintf("mongodb://%s:%s@%s",
		strings.TrimSpace(string(user)),
		strings.TrimSpace(string(pass)),
		host,
	)
}

// loadDomainsFromMongo connects to MongoDB and retrieves domains from the collection
func loadDomainsFromMongo(uri, db, coll string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(ctx)

	cursor, err := client.Database(db).Collection(coll).Find(ctx, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []string
	for cursor.Next(ctx) {
		var doc map[string]interface{}
		if err := cursor.Decode(&doc); err == nil {
			if val, ok := doc["domain"].(string); ok && strings.TrimSpace(val) != "" {
				results = append(results, val)
			}
		}
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		log.Println("[WARN] No domains found in MongoDB")
	}

	return results, nil
}

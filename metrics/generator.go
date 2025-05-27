package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"ssl-exporter/ssl"
)

type DomainList struct {
	Domains []string `json:"domains"`
}

const metricsFile = "metrics"

// getMongoURI builds MongoDB connection string using Docker Secrets and MONGO_HOST
func getMongoURI() string {
	user, err1 := os.ReadFile("/run/secrets/mongo_user")
	pass, err2 := os.ReadFile("/run/secrets/mongo_password")
	host := os.Getenv("MONGO_HOST")

	if err1 != nil || err2 != nil || host == "" {
		log.Println("[WARN] Mongo credentials or host missing")
		return ""
	}

	return fmt.Sprintf("mongodb://%s:%s@%s",
		url.QueryEscape(strings.TrimSpace(string(user))),
		url.QueryEscape(strings.TrimSpace(string(pass))),
		host,
	)
}

// getDomains decides whether to load domains from MongoDB or JSON file
func getDomains() []string {
	mongoURI := getMongoURI()
	mongoDB := os.Getenv("MONGO_DB")
	mongoCollection := os.Getenv("MONGO_COLLECTION")

	if mongoURI != "" && mongoDB != "" && mongoCollection != "" {
		domains, err := loadDomainsFromMongo(mongoURI, mongoDB, mongoCollection)
		if err == nil {
			return domains
		}
		log.Printf("[WARN] Fallback to JSON: %v", err)
	}

	domains, err := loadDomainsFromJSON("configs/domains.json")
	if err != nil {
		log.Fatalf("[FATAL] Failed to load domains from JSON: %v", err)
	}
	return domains
}

// loadDomainsFromJSON reads domains from local domains.json file
func loadDomainsFromJSON(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var list DomainList
	err = json.Unmarshal(data, &list)
	return list.Domains, err
}

// loadDomainsFromMongo connects to MongoDB and retrieves all documents with "domain" field
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

// Generate loads domains and writes metrics to file
func Generate() {
	domains := getDomains()

	log.Println("🔄 Generating metrics...")

	content := "# HELP ssl_cert_days_left Days left to expiration\n" +
		"# TYPE ssl_cert_days_left gauge\n" +
		"# HELP ssl_cert_start_timestamp Certificate start date (Unix timestamp)\n" +
		"# TYPE ssl_cert_start_timestamp gauge\n" +
		"# HELP ssl_cert_end_timestamp Certificate expiry date (Unix timestamp)\n" +
		"# TYPE ssl_cert_end_timestamp gauge\n" +
		"# HELP ssl_cert_domain_colored Used for domain coloring in Grafana\n" +
		"# TYPE ssl_cert_domain_colored gauge\n"

	for _, domain := range domains {
		cleanDomain := domain
		ftpLabel := "false"
		if strings.HasPrefix(domain, "ftp://") {
			cleanDomain = strings.TrimPrefix(domain, "ftp://")
			ftpLabel = "true"
		}

		var start, end time.Time
		var err error

		if ftpLabel == "true" {
			log.Printf("📡 Checking FTP TLS for: %s", cleanDomain)
			start, end, err = ssl.GetFTPCertificateTimestamps(cleanDomain)
		} else {
			log.Printf("📡 Checking HTTPS TLS for: %s", cleanDomain)
			start, end, err = ssl.GetCertificateTimestamps(cleanDomain)
		}

		if err != nil {
			log.Printf("❌ %s: %v", cleanDomain, err)
			content += fmt.Sprintf("ssl_cert_days_left{domain=\"%s\",is_ftp=\"%s\"} 0\n", cleanDomain, ftpLabel)
			content += fmt.Sprintf("ssl_cert_domain_colored{domain=\"%s\",is_ftp=\"%s\"} 0\n", cleanDomain, ftpLabel)
			content += fmt.Sprintf("ssl_cert_start_timestamp{domain=\"%s\",is_ftp=\"%s\"} 0\n", cleanDomain, ftpLabel)
			content += fmt.Sprintf("ssl_cert_end_timestamp{domain=\"%s\",is_ftp=\"%s\"} 0\n", cleanDomain, ftpLabel)
			continue
		}

		daysLeft := int(end.Sub(time.Now()).Hours() / 24)
		log.Printf("✅ %s: Days left: %d", cleanDomain, daysLeft)

		content += fmt.Sprintf("ssl_cert_days_left{domain=\"%s\",is_ftp=\"%s\"} %d\n", cleanDomain, ftpLabel, daysLeft)
		content += fmt.Sprintf("ssl_cert_domain_colored{domain=\"%s\",is_ftp=\"%s\"} %d\n", cleanDomain, ftpLabel, daysLeft)
		content += fmt.Sprintf("ssl_cert_start_timestamp{domain=\"%s\",is_ftp=\"%s\"} %d\n", cleanDomain, ftpLabel, start.Unix())
		content += fmt.Sprintf("ssl_cert_end_timestamp{domain=\"%s\",is_ftp=\"%s\"} %d\n", cleanDomain, ftpLabel, end.Unix())
	}

	_ = os.WriteFile(metricsFile, []byte(content), 0644)
}

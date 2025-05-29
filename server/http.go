package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func Start() {
	http.Handle("/metrics", http.HandlerFunc(metricsHandler))
	http.HandleFunc("/admin/add", addDomainHandler)

	log.Println("📡 Listening on :9115")
	log.Fatal(http.ListenAndServe(":9115", nil))
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "metrics")
}

func addDomainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var input struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.Domain) == "" {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	uri := getMongoURI()
	db := os.Getenv("MONGO_DB")
	coll := os.Getenv("MONGO_COLLECTION")

	if uri == "" || db == "" || coll == "" {
		http.Error(w, "Mongo config missing", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		http.Error(w, "Mongo connect error", http.StatusInternalServerError)
		return
	}
	defer client.Disconnect(ctx)

	collection := client.Database(db).Collection(coll)
	_, _ = collection.InsertOne(ctx, map[string]interface{}{"domain": strings.TrimSpace(input.Domain)})
	log.Println("✅ Added domain:", input.Domain)

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("OK"))
}

func getMongoURI() string {
	user, _ := os.ReadFile("/run/secrets/mongo_user")
	pass, _ := os.ReadFile("/run/secrets/mongo_password")
	host := os.Getenv("MONGO_HOST")

	if len(user) == 0 || len(pass) == 0 || host == "" {
		return ""
	}

	return "mongodb://" + strings.TrimSpace(string(user)) + ":" + strings.TrimSpace(string(pass)) + "@" + host
}

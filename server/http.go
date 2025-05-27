package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type payload struct {
	Domain string `json:"domain"`
}

func Start() {
	http.Handle("/metrics", http.StripPrefix("/metrics", http.FileServer(http.Dir("."))))
	http.HandleFunc("/admin/add", auth(addDomainHandler))
	log.Println("📡 Listening on :9115")
	log.Fatal(http.ListenAndServe(":9115", nil))
}

// Basic auth middleware
func auth(next http.HandlerFunc) http.HandlerFunc {
	user := os.Getenv("ADMIN_USER")
	pass := os.Getenv("ADMIN_PASSWORD")
	return func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// Handles POST /admin/add with JSON payload {"domain": "example.com"}
func addDomainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var p payload
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &p); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	p.Domain = strings.TrimSpace(p.Domain)
	if p.Domain == "" {
		http.Error(w, "Domain required", http.StatusBadRequest)
		return
	}

	uri := getMongoURI()
	db := os.Getenv("MONGO_DB")
	coll := os.Getenv("MONGO_COLLECTION")

	if uri == "" || db == "" || coll == "" {
		http.Error(w, "MongoDB env missing", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		http.Error(w, "Mongo connection error", http.StatusInternalServerError)
		return
	}
	defer client.Disconnect(ctx)

	_, err = client.Database(db).Collection(coll).InsertOne(ctx, map[string]interface{}{"domain": p.Domain})
	if err != nil {
		http.Error(w, "Mongo insert error", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Added domain: %s", p.Domain)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("OK"))
}

// getMongoURI builds MongoDB connection string using Docker Secrets and MONGO_HOST
func getMongoURI() string {
	user, err1 := os.ReadFile("/run/secrets/mongo_user")
	pass, err2 := os.ReadFile("/run/secrets/mongo_password")
	host := os.Getenv("MONGO_HOST")

	if err1 != nil || err2 != nil || host == "" {
		log.Println("[WARN] Mongo credentials or host missing")
		return ""
	}

	return "mongodb://" + strings.TrimSpace(string(user)) + ":" + strings.TrimSpace(string(pass)) + "@" + host
}

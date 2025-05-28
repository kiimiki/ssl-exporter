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
	http.HandleFunc("/metrics", serveCustomMetrics)
	http.HandleFunc("/admin/add", auth(addDomainHandler))
	log.Println("📡 Listening on :9115")
	log.Fatal(http.ListenAndServe(":9115", nil))
}

func serveCustomMetrics(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("metrics")
	if err != nil {
		log.Printf("❌ Failed to read metrics file: %v", err)
		http.Error(w, "metrics unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

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

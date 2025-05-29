package server

import (
	"context"
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
	http.HandleFunc("/admin/add", auth(addDomainHandler))

	log.Println("📡 Listening on :9115")
	log.Fatal(http.ListenAndServe(":9115", nil))
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "metrics")
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

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Parse error", http.StatusBadRequest)
		return
	}

	lines := strings.Split(r.FormValue("domains"), "\n")
	var clean []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			clean = append(clean, line)
		}
	}

	if len(clean) == 0 {
		http.Error(w, "Empty input", http.StatusBadRequest)
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
	for _, d := range clean {
		_, _ = collection.InsertOne(ctx, map[string]interface{}{"domain": d})
		log.Println("✅ Added domain:", d)
	}

	w.WriteHeader(http.StatusOK)
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

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

type Item struct {
	Name     string `json:"name"`
	Bought   bool   `json:"bought"`
	Category string `json:"category"`
	Priority int    `json:"priority"` // 1 - низкий, 2 - средний, 3 - высокий
}

var (
	mutex    sync.Mutex
	username string
	password string
)

func init() {
	// Читаем логин и пароль из переменных окружения, переданных через docker-compose
	username = os.Getenv("USERNAME")
	password = os.Getenv("PASSWORD")
	if username == "" || password == "" {
		log.Fatal("USERNAME and PASSWORD must be set in .env")
	}
}

func main() {
	r := mux.NewRouter()

	r.Use(authMiddleware)
	r.HandleFunc("/", indexHandler).Methods("GET")
	r.HandleFunc("/auth", authHandler).Methods("GET") // Новый эндпоинт для передачи credentials
	r.HandleFunc("/list", listHandler).Methods("GET")
	r.HandleFunc("/add", addHandler).Methods("POST")
	r.HandleFunc("/buy/{name}", buyHandler).Methods("PUT")
	r.HandleFunc("/delete/{name}", deleteHandler).Methods("DELETE")
	r.HandleFunc("/edit/{name}", editHandler).Methods("PUT")
	r.HandleFunc("/reorder", reorderHandler).Methods("POST")
	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Basic ") {
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		encodedCredentials := strings.TrimPrefix(authHeader, "Basic ")
		decodedCredentials, err := base64.StdEncoding.DecodeString(encodedCredentials)
		if err != nil {
			http.Error(w, "Invalid base64 encoding", http.StatusUnauthorized)
			return
		}

		credentials := strings.SplitN(string(decodedCredentials), ":", 2)
		if len(credentials) != 2 || credentials[0] != username || credentials[1] != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"username": username,
		"password": password,
	})
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	htmlFile, err := os.Open("index.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error opening HTML file:", err)
		return
	}
	defer htmlFile.Close()

	w.Header().Set("Content-Type", "text/html")
	_, err = io.Copy(w, htmlFile)
	if err != nil {
		log.Println("Error sending HTML content:", err)
	}
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	category := r.URL.Query().Get("category")
	if category == "" {
		http.Error(w, "Category is required", http.StatusBadRequest)
		return
	}

	key := "shoppingList:" + category
	val, err := client.Get(ctx, key).Result()
	if err == redis.Nil {
		json.NewEncoder(w).Encode([]Item{})
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	err = json.Unmarshal([]byte(val), &items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(items)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	var newItem Item
	err := json.NewDecoder(r.Body).Decode(&newItem)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if newItem.Category == "" {
		http.Error(w, "Category is required", http.StatusBadRequest)
		return
	}

	if newItem.Priority == 0 {
		newItem.Priority = 2
	}

	if newItem.Priority < 1 || newItem.Priority > 3 {
		newItem.Priority = 2
	}

	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	mutex.Lock()
	defer mutex.Unlock()

	key := "shoppingList:" + newItem.Category
	val, err := client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	if err == nil {
		err = json.Unmarshal([]byte(val), &items)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	items = append(items, newItem)
	logActivity("Added", newItem.Name)

	data, err := json.Marshal(items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = client.Set(ctx, key, data, 0).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]string{"message": "Item added successfully"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	oldName := vars["name"]

	var editedItem Item
	err := json.NewDecoder(r.Body).Decode(&editedItem)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	oldCategory := r.URL.Query().Get("oldCategory")
	if oldCategory == "" {
		http.Error(w, "Old category is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	mutex.Lock()
	defer mutex.Unlock()

	oldKey := "shoppingList:" + oldCategory
	val, err := client.Get(ctx, oldKey).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var oldItems []Item
	if err == nil {
		err = json.Unmarshal([]byte(val), &oldItems)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var newOldItems []Item
	for _, item := range oldItems {
		if item.Name != oldName {
			newOldItems = append(newOldItems, item)
		}
	}

	data, err := json.Marshal(newOldItems)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = client.Set(ctx, oldKey, data, 0).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	newKey := "shoppingList:" + editedItem.Category
	val, err = client.Get(ctx, newKey).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var newItems []Item
	if err == nil {
		err = json.Unmarshal([]byte(val), &newItems)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if editedItem.Priority < 1 || editedItem.Priority > 3 {
		editedItem.Priority = 2
	}

	newItems = append(newItems, editedItem)
	logActivity("Edited", editedItem.Name)

	data, err = json.Marshal(newItems)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = client.Set(ctx, newKey, data, 0).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func buyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemName := vars["name"]

	var item struct {
		Bought   bool   `json:"bought"`
		Category string `json:"category"`
	}
	err := json.NewDecoder(r.Body).Decode(&item)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	mutex.Lock()
	defer mutex.Unlock()

	key := "shoppingList:" + item.Category
	val, err := client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	if err == nil {
		err = json.Unmarshal([]byte(val), &items)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	for i := range items {
		if items[i].Name == itemName {
			items[i].Bought = item.Bought
			break
		}
	}

	data, err := json.Marshal(items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = client.Set(ctx, key, data, 0).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemName := vars["name"]

	category := r.URL.Query().Get("category")
	if category == "" {
		http.Error(w, "Category is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	mutex.Lock()
	defer mutex.Unlock()

	key := "shoppingList:" + category
	val, err := client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	if err == nil {
		err = json.Unmarshal([]byte(val), &items)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var newItems []Item
	for _, item := range items {
		if item.Name != itemName {
			newItems = append(newItems, item)
		}
	}
	logActivity("Deleted", itemName)

	data, err := json.Marshal(newItems)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = client.Set(ctx, key, data, 0).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func reorderHandler(w http.ResponseWriter, r *http.Request) {
	var items []Item
	err := json.NewDecoder(r.Body).Decode(&items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(items) == 0 {
		http.Error(w, "No items provided", http.StatusBadRequest)
		return
	}

	category := items[0].Category
	key := "shoppingList:" + category

	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	mutex.Lock()
	defer mutex.Unlock()

	data, err := json.Marshal(items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = client.Set(ctx, key, data, 0).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func getRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
}

func logActivity(action, itemName string) {
	log.Printf("%s item: %s", action, itemName)
}

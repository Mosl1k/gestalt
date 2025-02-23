package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	mutex sync.Mutex
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", indexHandler).Methods("GET")
	r.HandleFunc("/list", listHandler).Methods("GET")
	r.HandleFunc("/add", addHandler).Methods("POST")
	r.HandleFunc("/buy/{name}", buyHandler).Methods("PUT")
	r.HandleFunc("/delete/{name}", deleteHandler).Methods("DELETE")
	r.HandleFunc("/edit/{name}", editHandler).Methods("PUT")
	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", r))
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

	val, err := client.Get(ctx, "shoppingList").Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	err = json.Unmarshal([]byte(val), &items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	category := r.URL.Query().Get("category")
	if category != "" {
		var filteredItems []Item
		for _, item := range items {
			if item.Category == category {
				filteredItems = append(filteredItems, item)
			}
		}
		items = filteredItems
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
		newItem.Category = "купить"
	}

	// Устанавливаем приоритет 2 (средний) по умолчанию, если не указан
	if newItem.Priority == 0 {
		newItem.Priority = 2
	}

	// Ограничиваем приоритет значениями 1-3
	if newItem.Priority < 1 || newItem.Priority > 3 {
		newItem.Priority = 2
	}

	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	mutex.Lock()
	defer mutex.Unlock()

	val, err := client.Get(ctx, "shoppingList").Result()
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

	err = client.Set(ctx, "shoppingList", data, 0).Err()
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

	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	mutex.Lock()
	defer mutex.Unlock()

	val, err := client.Get(ctx, "shoppingList").Result()
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
		if items[i].Name == oldName {
			items[i].Name = editedItem.Name
			items[i].Category = editedItem.Category
			// Ограничиваем приоритет значениями 1-3
			if editedItem.Priority >= 1 && editedItem.Priority <= 3 {
				items[i].Priority = editedItem.Priority
			}
			break
		}
	}

	data, err := json.Marshal(items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = client.Set(ctx, "shoppingList", data, 0).Err()
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
		Bought bool `json:"bought"`
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

	val, err := client.Get(ctx, "shoppingList").Result()
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

	err = client.Set(ctx, "shoppingList", data, 0).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemName := vars["name"]

	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	mutex.Lock()
	defer mutex.Unlock()

	val, err := client.Get(ctx, "shoppingList").Result()
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

	err = client.Set(ctx, "shoppingList", data, 0).Err()
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

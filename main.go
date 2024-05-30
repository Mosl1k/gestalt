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
	Quantity int    `json:"quantity"`
	Bought   bool   `json:"bought"`
	Category string `json:"category"` // Поле для категории
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

	fmt.Println("Server is running on port 80...")
	// log.Println(os.Getenv("REDIS_ADDR"), os.Getenv("REDIS_PASSWORD"))
	log.Fatal(http.ListenAndServe(":80", r))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	htmlFile, err := os.Open("index.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error opening HTML file:", err)
		return
	}
	defer htmlFile.Close()

	// Отправляем содержимое файла как ответ
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

	// Устанавливаем категорию "купить" по умолчанию
	if newItem.Category == "" {
		newItem.Category = "купить"
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

	w.WriteHeader(http.StatusCreated)
}

func buyHandler(w http.ResponseWriter, r *http.Request) {
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

	for i := range items {
		if items[i].Name == itemName {
			items[i].Bought = !items[i].Bought
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

	// Создаем новый срез для хранения элементов, кроме удаляемого
	var newItems []Item
	// Перебираем элементы и копируем их в новый срез, исключая удаляемый
	for _, item := range items {
		if item.Name != itemName {
			newItems = append(newItems, item)
		}
	}
	// Выполняем логирование удаления элемента
	logActivity("Deleted", itemName)

	data, err := json.Marshal(newItems) // Маршализуем обновленный список
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
		DB:       0, // Используемая база данных
	})
}

func logActivity(activity string, itemName string) {
	// Логгирование действий в консоль
	log.Printf("%s: %s", activity, itemName)
}

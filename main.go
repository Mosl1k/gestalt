package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

type Item struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Bought   bool   `json:"bought"`
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

	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	html := `
	<!DOCTYPE html>
	<html lang="en">
	<head>
	    <meta charset="UTF-8">
	    <meta name="viewport" content="width=device-width, initial-scale=1.0">
	    <title>Shopping List</title>
	</head>
	<body>
	    <h1>Shopping List</h1>
	    <div>
	        <input type="text" id="itemName" placeholder="Item name">
	        <button onclick="addItem()">Add</button>
	    </div>
	    <ul id="shoppingList"></ul>

	    <script>
	        function addItem() {
	            const itemNameInput = document.getElementById('itemName');
	            const itemName = itemNameInput.value.trim();
	            if (!itemName) return;

	            fetch('/add', {
	                method: 'POST',
	                headers: {
	                    'Content-Type': 'application/json',
	                },
	                body: JSON.stringify({
	                    name: itemName,
	                    quantity: 1,
	                }),
	            })
	            .then(response => {
	                if (!response.ok) {
	                    throw new Error('Network response was not ok');
	                }
	                itemNameInput.value = '';
	                fetchShoppingList();
	            })
	            .catch(error => {
	                console.error('There was an error!', error);
	            });
	        }

	        function toggleBought(name, checked) {
	            fetch('/buy/' + encodeURIComponent(name), {
	                method: 'PUT',
	                headers: {
	                    'Content-Type': 'application/json',
	                },
	                body: JSON.stringify({
	                    bought: checked,
	                }),
	            })
	            .then(response => {
	                if (!response.ok) {
	                    throw new Error('Network response was not ok');
	                }
	                fetchShoppingList();
	            })
	            .catch(error => {
	                console.error('There was an error!', error);
	            });
	        }

	        function deleteItem(name, listItem) {
	            fetch('/delete/' + encodeURIComponent(name), {
	                method: 'DELETE',
	            })
	            .then(response => {
	                if (!response.ok) {
	                    throw new Error('Network response was not ok');
	                }
	                listItem.remove();
	            })
	            .catch(error => {
	                console.error('There was an error!', error);
	            });
	        }

	        function fetchShoppingList() {
	            fetch('/list')
	            .then(response => {
	                if (!response.ok) {
	                    throw new Error('Network response was not ok');
	                }
	                return response.json();
	            })
	            .then(data => {
	                const shoppingList = document.getElementById('shoppingList');
	                shoppingList.innerHTML = '';
	                data.forEach(item => {
	                    const li = document.createElement('li');
	                    const checkbox = document.createElement('input');
	                    checkbox.type = 'checkbox';
	                    checkbox.checked = item.bought;
	                    checkbox.onclick = () => toggleBought(item.name, checkbox.checked);
	                    li.appendChild(checkbox);
	                    li.appendChild(document.createTextNode(item.name));
	                    if (item.bought) {
	                        const deleteButton = document.createElement('button');
	                        deleteButton.textContent = 'Delete';
	                        deleteButton.onclick = () => deleteItem(item.name, li);
	                        li.appendChild(deleteButton);
	                    }
	                    shoppingList.appendChild(li);
	                });
	            })
	            .catch(error => {
	                console.error('There was an error!', error);
	            });
	        }

	        // Load shopping list on page load
	        fetchShoppingList();
	    </script>
	</body>
	</html>
	`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
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

	json.NewEncoder(w).Encode(items)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	var newItem Item
	err := json.NewDecoder(r.Body).Decode(&newItem)
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

	items = append(items, newItem)

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

	for i, item := range items {
		if item.Name == itemName {
			items = append(items[:i], items[i+1:]...)
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

func getRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     "redis:6379", // Название контейнера Redis в сети Docker Compose
		Password: "",           // Пароль, если он установлен
		DB:       0,            // Используемая база данных
	})
}

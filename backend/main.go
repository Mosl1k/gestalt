package main

import (
	"context"
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
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/yandex"
)

type Item struct {
	Name     string `json:"name"`
	Bought   bool   `json:"bought"`
	Category string `json:"category"`
	Priority int    `json:"priority"` // 1 - низкий, 2 - средний, 3 - высокий
}

var (
	mutex sync.Mutex
	store *sessions.CookieStore
)

func init() {
	// Загружаем переменные окружения из .env файла (только для локальной разработки)
	// В production переменные должны быть установлены через Kubernetes Secrets/ConfigMaps
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		godotenv.Load()
	}

	// Получаем секретный ключ из переменных окружения
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		log.Fatal("Необходимо установить переменную окружения SESSION_SECRET")
	}

	// Создаём хранилище сессий с секретным ключом
	store = sessions.NewCookieStore([]byte(sessionSecret))

	// Настройка хранилища сессий
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30, // 30 дней
		HttpOnly: true,
		Secure:   true, // true для HTTPS
		SameSite: http.SameSiteLaxMode, // Lax для работы через proxy
		// Domain не указываем, чтобы cookies работали через nginx proxy
	}

	// Инициализация OAuth2 провайдера Yandex
	clientID := os.Getenv("YANDEX_CLIENT_ID")
	clientSecret := os.Getenv("YANDEX_CLIENT_SECRET")
	callbackURL := os.Getenv("YANDEX_CALLBACK_URL")

	if clientID != "" && clientSecret != "" {
		if callbackURL == "" {
			log.Println("Предупреждение: YANDEX_CALLBACK_URL не установлен. OAuth может не работать.")
		}
		goth.UseProviders(
			yandex.New(clientID, clientSecret, callbackURL),
		)
		gothic.Store = store
		log.Println("Yandex OAuth провайдер инициализирован")
	} else {
		log.Println("Предупреждение: YANDEX_CLIENT_ID и YANDEX_CLIENT_SECRET не установлены. OAuth будет недоступен.")
	}
}

// Middleware для проверки, что запрос идет из внутренней сети (Docker)
func internalNetworkMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем IP адрес клиента
		clientIP := r.RemoteAddr
		// Убираем порт из IP адреса
		if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
			clientIP = clientIP[:idx]
		}
		// Убираем квадратные скобки для IPv6
		clientIP = strings.Trim(clientIP, "[]")
		
		// Проверяем X-Forwarded-For заголовок (если есть прокси)
		// НО для внутренних запросов из Docker сети не должно быть X-Forwarded-For
		// Если есть X-Forwarded-For, это может быть внешний запрос
		forwarded := r.Header.Get("X-Forwarded-For")
		if forwarded != "" {
			// Если есть X-Forwarded-For, берем первый IP
			clientIP = strings.TrimSpace(strings.Split(forwarded, ",")[0])
		}
		
		// Разрешаем доступ только из Docker сети
		// Docker использует подсети: 172.16.0.0/12, 10.0.0.0/8, 192.168.0.0/16
		allowed := false
		
		// Проверка по IP адресу (Docker сети)
		// 172.16.0.0/12 = 172.16.0.0 - 172.31.255.255
		if strings.HasPrefix(clientIP, "172.16.") ||
			strings.HasPrefix(clientIP, "172.17.") ||
			strings.HasPrefix(clientIP, "172.18.") ||
			strings.HasPrefix(clientIP, "172.19.") ||
			strings.HasPrefix(clientIP, "172.20.") ||
			strings.HasPrefix(clientIP, "172.21.") ||
			strings.HasPrefix(clientIP, "172.22.") ||
			strings.HasPrefix(clientIP, "172.23.") ||
			strings.HasPrefix(clientIP, "172.24.") ||
			strings.HasPrefix(clientIP, "172.25.") ||
			strings.HasPrefix(clientIP, "172.26.") ||
			strings.HasPrefix(clientIP, "172.27.") ||
			strings.HasPrefix(clientIP, "172.28.") ||
			strings.HasPrefix(clientIP, "172.29.") ||
			strings.HasPrefix(clientIP, "172.30.") ||
			strings.HasPrefix(clientIP, "172.31.") {
			allowed = true
		}
		
		// Проверка других Docker подсетей
		if strings.HasPrefix(clientIP, "10.") ||
			strings.HasPrefix(clientIP, "192.168.") ||
			clientIP == "127.0.0.1" ||
			clientIP == "::1" ||
			clientIP == "localhost" {
			allowed = true
		}
		
		// Если запрос идет без X-Forwarded-For - это запрос напрямую из Docker контейнера
		// В Docker сети запросы идут напрямую между контейнерами, без внешнего прокси
		if !allowed && forwarded == "" {
			// Разрешаем все запросы без X-Forwarded-For (внутренние запросы из Docker)
			allowed = true
			log.Printf("Разрешен внутренний запрос без X-Forwarded-For от IP: %s", clientIP)
		}
		
		// Если не разрешено, проверяем по заголовку (для дополнительной безопасности)
		if !allowed && r.Header.Get("X-Internal-Request") == "true" {
			allowed = true
		}
		
		if !allowed {
			log.Printf("Запрещен доступ к внутреннему API с IP: %s, X-Forwarded-For: %s, RemoteAddr: %s", 
				clientIP, forwarded, r.RemoteAddr)
			http.Error(w, "Forbidden: Internal API access only", http.StatusForbidden)
			return
		}
		
		// Логируем успешный доступ для отладки
		log.Printf("Разрешен доступ к внутреннему API от IP: %s, Path: %s", clientIP, r.URL.Path)
		
		next.ServeHTTP(w, r)
	})
}

func main() {
	r := mux.NewRouter()

	// Публичные маршруты (без авторизации)
	r.HandleFunc("/", indexHandler).Methods("GET") // Главная страница доступна всем
	r.HandleFunc("/auth/yandex", authHandler).Methods("GET")
	r.HandleFunc("/auth/yandex/callback", callbackHandler).Methods("GET")
	r.HandleFunc("/logout", logoutHandler).Methods("GET")

	// Внутренние API endpoints для сервисов (без авторизации, только из Docker сети)
	internal := r.PathPrefix("/internal/api").Subrouter()
	internal.Use(internalNetworkMiddleware)
	internal.HandleFunc("/list", internalListHandler).Methods("GET")
	internal.HandleFunc("/add", internalAddHandler).Methods("POST")
	internal.HandleFunc("/buy/{name}", internalBuyHandler).Methods("PUT")
	internal.HandleFunc("/delete/{name}", internalDeleteHandler).Methods("DELETE")
	internal.HandleFunc("/edit/{name}", internalEditHandler).Methods("PUT")

	// Защищённые маршруты (требуют авторизации через OAuth)
	// Регистрируем напрямую с применением middleware
	r.HandleFunc("/list", authMiddleware(http.HandlerFunc(listHandler)).ServeHTTP).Methods("GET")
	r.HandleFunc("/add", authMiddleware(http.HandlerFunc(addHandler)).ServeHTTP).Methods("POST")
	r.HandleFunc("/buy/{name}", authMiddleware(http.HandlerFunc(buyHandler)).ServeHTTP).Methods("PUT")
	r.HandleFunc("/delete/{name}", authMiddleware(http.HandlerFunc(deleteHandler)).ServeHTTP).Methods("DELETE")
	r.HandleFunc("/edit/{name}", authMiddleware(http.HandlerFunc(editHandler)).ServeHTTP).Methods("PUT")
	r.HandleFunc("/reorder", authMiddleware(http.HandlerFunc(reorderHandler)).ServeHTTP).Methods("POST")
	
	// API для друзей
	r.HandleFunc("/api/user", authMiddleware(http.HandlerFunc(getCurrentUserHandler)).ServeHTTP).Methods("GET")
	r.HandleFunc("/api/users/search", authMiddleware(http.HandlerFunc(searchUsersHandler)).ServeHTTP).Methods("GET")
	r.HandleFunc("/api/users/all", authMiddleware(http.HandlerFunc(getAllUsersHandler)).ServeHTTP).Methods("GET")
	r.HandleFunc("/api/friends", authMiddleware(http.HandlerFunc(getFriendsHandler)).ServeHTTP).Methods("GET")
	r.HandleFunc("/api/friends/add", authMiddleware(http.HandlerFunc(addFriendHandler)).ServeHTTP).Methods("POST")
	r.HandleFunc("/api/friends/remove", authMiddleware(http.HandlerFunc(removeFriendHandler)).ServeHTTP).Methods("DELETE")
	r.HandleFunc("/api/shared-lists", authMiddleware(http.HandlerFunc(getSharedListsHandler)).ServeHTTP).Methods("GET")
	r.HandleFunc("/api/share-list", authMiddleware(http.HandlerFunc(shareListHandler)).ServeHTTP).Methods("POST")

	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", r))
}

// Middleware для проверки авторизации через Yandex OAuth
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "session")

		// Проверяем, авторизован ли пользователь
		userID, ok := session.Values["user_id"].(string)
		if !ok || userID == "" {
			log.Printf("Пользователь не авторизован для %s. Cookies: %v", r.URL.Path, r.Header.Get("Cookie"))
			// Пользователь не авторизован - перенаправляем на страницу авторизации
			http.Redirect(w, r, "/auth/yandex", http.StatusFound)
			return
		}

		// Пользователь авторизован - продолжаем
		next.ServeHTTP(w, r)
	})
}

// Обработчик начала авторизации через Yandex
func authHandler(w http.ResponseWriter, r *http.Request) {
	// Устанавливаем провайдер в контекст для gothic
	// Gothic сам управляет состоянием OAuth, нам не нужно делать это вручную
	ctx := context.WithValue(r.Context(), "provider", "yandex")
	r = r.WithContext(ctx)

	// Перенаправляем на страницу авторизации Yandex
	// Gothic сам сгенерирует и сохранит состояние
	gothic.BeginAuthHandler(w, r)
}

// Обработчик callback от Yandex
func callbackHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем, есть ли ошибка от Yandex
	if errorParam := r.URL.Query().Get("error"); errorParam != "" {
		errorDesc := r.URL.Query().Get("error_description")
		if errorDesc == "" {
			errorDesc = errorParam
		}

		errorHTML := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<title>Ошибка авторизации</title>
	<meta charset="UTF-8">
	<style>
		body {
			font-family: Arial, sans-serif;
			display: flex;
			justify-content: center;
			align-items: center;
			height: 100vh;
			margin: 0;
			background: #f5f5f5;
		}
		.container {
			background: white;
			padding: 40px;
			border-radius: 10px;
			box-shadow: 0 2px 10px rgba(0,0,0,0.1);
			text-align: center;
			max-width: 500px;
		}
		h1 { color: #dc3545; }
		.error { 
			background: #f8d7da;
			color: #721c24;
			padding: 15px;
			border-radius: 5px;
			margin: 20px 0;
		}
		.btn {
			display: inline-block;
			padding: 12px 30px;
			background: #667eea;
			color: white;
			text-decoration: none;
			border-radius: 5px;
			font-weight: bold;
			margin-top: 20px;
		}
	</style>
</head>
<body>
	<div class="container">
		<h1>❌ Ошибка авторизации</h1>
		<div class="error">
			<p><strong>Ошибка:</strong> %s</p>
			<p>%s</p>
		</div>
		<a href="/" class="btn">Вернуться на главную</a>
	</div>
</body>
</html>
`, errorParam, errorDesc)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errorHTML)
		return
	}

	// Устанавливаем провайдер в контекст для gothic
	ctx := context.WithValue(r.Context(), "provider", "yandex")
	r = r.WithContext(ctx)

	// Получаем пользователя от провайдера
	// Gothic сам проверяет состояние OAuth внутри CompleteUserAuth
	user, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		log.Printf("Ошибка CompleteUserAuth: %v", err)
		
		// Если код истек, перенаправляем на повторную авторизацию
		if strings.Contains(err.Error(), "invalid_grant") || strings.Contains(err.Error(), "Code has expired") {
			log.Println("Код авторизации истек, перенаправляем на повторную авторизацию")
			http.Redirect(w, r, "/auth/yandex", http.StatusFound)
			return
		}
		
		// Для других ошибок показываем страницу с ошибкой
		errorHTML := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<title>Ошибка авторизации</title>
	<meta charset="UTF-8">
	<style>
		body {
			font-family: Arial, sans-serif;
			display: flex;
			justify-content: center;
			align-items: center;
			height: 100vh;
			margin: 0;
			background: #f5f5f5;
		}
		.container {
			background: white;
			padding: 40px;
			border-radius: 10px;
			box-shadow: 0 2px 10px rgba(0,0,0,0.1);
			text-align: center;
			max-width: 500px;
		}
		h1 { color: #dc3545; }
		.error { 
			background: #f8d7da;
			color: #721c24;
			padding: 15px;
			border-radius: 5px;
			margin: 20px 0;
		}
		.btn {
			display: inline-block;
			padding: 12px 30px;
			background: #667eea;
			color: white;
			text-decoration: none;
			border-radius: 5px;
			font-weight: bold;
			margin-top: 20px;
		}
	</style>
</head>
<body>
	<div class="container">
		<h1>❌ Ошибка авторизации</h1>
		<div class="error">
			<p><strong>Ошибка:</strong> %s</p>
			<p>Попробуйте авторизоваться снова</p>
		</div>
		<a href="/auth/yandex" class="btn">Повторить авторизацию</a>
		<a href="/" class="btn">Вернуться на главную</a>
	</div>
</body>
</html>
`, err.Error())
		
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, errorHTML)
		return
	}

	// Сохраняем информацию о пользователе в сессии
	session, _ := store.Get(r, "session")
	session.Values["user"] = user.Name
	session.Values["email"] = user.Email
	session.Values["provider"] = user.Provider
	session.Values["user_id"] = user.UserID
	
	// Сохраняем сессию и проверяем на ошибки
	if err := session.Save(r, w); err != nil {
		log.Printf("Ошибка сохранения сессии: %v", err)
		http.Error(w, fmt.Sprintf("Ошибка сохранения сессии: %v", err), http.StatusInternalServerError)
		return
	}
	
	log.Printf("Сессия сохранена для пользователя: %s (%s), user_id: %s", user.Name, user.Email, user.UserID)
	
	// Сохраняем информацию о пользователе в Redis для поиска друзей
	saveUserToRedis(user.UserID, user.Name, user.Email)
	
	log.Printf("Пользователь авторизован: %s (%s)", user.Name, user.Email)

	// Перенаправляем на главную страницу
	http.Redirect(w, r, "/", http.StatusFound)
}

// Обработчик выхода
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")

	// Очищаем сессию
	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1
	session.Save(r, w)

	// Перенаправляем на главную
	http.Redirect(w, r, "/", http.StatusFound)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	
	// Проверяем, авторизован ли пользователь
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		log.Printf("Пользователь не авторизован, показываем приветственную страницу. Cookies: %v", r.Header.Get("Cookie"))
		// Пользователь не авторизован - показываем приветственную страницу
		welcomeHTML := `
<!DOCTYPE html>
<html>
<head>
	<title>Gestalt - Список покупок</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		body {
			font-family: Arial, sans-serif;
			display: flex;
			justify-content: center;
			align-items: center;
			height: 100vh;
			margin: 0;
			background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
		}
		.container {
			background: white;
			padding: 40px;
			border-radius: 10px;
			box-shadow: 0 10px 40px rgba(0,0,0,0.2);
			text-align: center;
			max-width: 500px;
		}
		h1 { color: #333; margin-bottom: 20px; }
		p { color: #666; line-height: 1.6; }
		.btn {
			display: inline-block;
			padding: 12px 30px;
			background: #FFCC00;
			color: #000;
			text-decoration: none;
			border-radius: 5px;
			font-weight: bold;
			margin-top: 20px;
		}
		.btn:hover {
			background: #FFD700;
		}
	</style>
</head>
<body>
	<div class="container">
		<h1>🛒 Gestalt</h1>
		<p>Добро пожаловать в систему управления списками покупок!</p>
		<p>Для доступа к приложению необходимо авторизоваться через Yandex.</p>
		<a href="/auth/yandex" class="btn">Войти через Yandex</a>
	</div>
</body>
</html>
`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, welcomeHTML)
		return
	}

	// Пользователь авторизован - показываем основное приложение
	htmlFile, err := os.Open("/app/index.html")
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
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	category := r.URL.Query().Get("category")
	if category == "" {
		http.Error(w, "Category is required", http.StatusBadRequest)
		return
	}

	// Для категории "купить" проверяем общие списки
	if category == "купить" {
		// Загружаем личный список
		personalKey := "shoppingList:" + userID + ":" + category
		personalVal, err := client.Get(ctx, personalKey).Result()
		var personalItems []Item
		if err == nil && personalVal != "" {
			if err := json.Unmarshal([]byte(personalVal), &personalItems); err != nil {
				log.Printf("Ошибка парсинга JSON для %s: %v", personalKey, err)
				personalItems = []Item{} // Используем пустой список при ошибке
			}
		}
		
		// Проверяем, есть ли общие списки
		sharedLists, _ := client.SMembers(ctx, "shared_lists:"+userID).Result()
		for _, listKey := range sharedLists {
			parts := splitListKey(listKey)
			if len(parts) == 2 && parts[1] == "купить" {
				ownerID := parts[0]
				// Загружаем общий список друга (из его личного списка)
				sharedKey := "shoppingList:" + ownerID + ":купить"
				val, err := client.Get(ctx, sharedKey).Result()
				if err == nil && val != "" {
					var sharedItems []Item
					if err := json.Unmarshal([]byte(val), &sharedItems); err != nil {
						log.Printf("Ошибка парсинга JSON для %s: %v", sharedKey, err)
						continue // Пропускаем невалидные данные
					}
					// Получаем имя владельца
					ownerData, _ := client.Get(ctx, "user:"+ownerID).Result()
					var ownerInfo map[string]string
					ownerName := ownerID
					if ownerData != "" {
						json.Unmarshal([]byte(ownerData), &ownerInfo)
						ownerName = ownerInfo["name"]
					}
					// Добавляем информацию о владельце
					for i := range sharedItems {
						sharedItems[i].Name = "[" + ownerName + "] " + sharedItems[i].Name
					}
					personalItems = append(personalItems, sharedItems...)
				}
			}
		}
		
		json.NewEncoder(w).Encode(personalItems)
		return
	}

	// Личный список (для всех категорий, кроме общих "купить")
	key := "shoppingList:" + userID + ":" + category
	val, err := client.Get(ctx, key).Result()
	if err == redis.Nil || val == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Item{})
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	err = json.Unmarshal([]byte(val), &items)
	if err != nil {
		log.Printf("Ошибка парсинга JSON для %s: %v, значение: %s", key, err, val)
		// Возвращаем пустой список вместо ошибки
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Item{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
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

	// Всегда сохраняем в личный список пользователя
	key := "shoppingList:" + userID + ":" + newItem.Category
	val, err := client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	if err == nil && val != "" {
		err = json.Unmarshal([]byte(val), &items)
		if err != nil {
			log.Printf("Ошибка парсинга JSON: %v, значение: %s", err, val)
			// Используем пустой список при ошибке парсинга
			items = []Item{}
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
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
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

	oldKey := "shoppingList:" + userID + ":" + oldCategory
	val, err := client.Get(ctx, oldKey).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var oldItems []Item
	if err == nil && val != "" {
		err = json.Unmarshal([]byte(val), &oldItems)
		if err != nil {
			log.Printf("Ошибка парсинга JSON для %s: %v, значение: %s", oldKey, err, val)
			oldItems = []Item{} // Используем пустой список при ошибке
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

	newKey := "shoppingList:" + userID + ":" + editedItem.Category
	val, err = client.Get(ctx, newKey).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var newItems []Item
	if err == nil && val != "" {
		err = json.Unmarshal([]byte(val), &newItems)
		if err != nil {
			log.Printf("Ошибка парсинга JSON для %s: %v, значение: %s", newKey, err, val)
			newItems = []Item{} // Используем пустой список при ошибке
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
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
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

	key := "shoppingList:" + userID + ":" + item.Category
	val, err := client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	if err == nil && val != "" {
		err = json.Unmarshal([]byte(val), &items)
		if err != nil {
			log.Printf("Ошибка парсинга JSON: %v, значение: %s", err, val)
			// Используем пустой список при ошибке парсинга
			items = []Item{}
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
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
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

	key := "shoppingList:" + userID + ":" + category
	val, err := client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	if err == nil && val != "" {
		err = json.Unmarshal([]byte(val), &items)
		if err != nil {
			log.Printf("Ошибка парсинга JSON: %v, значение: %s", err, val)
			// Используем пустой список при ошибке парсинга
			items = []Item{}
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
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
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
	key := "shoppingList:" + userID + ":" + category

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

// Сохранение пользователя в Redis для поиска друзей
func saveUserToRedis(userID, name, email string) {
	ctx := context.Background()
	client := getRedisClient()
	defer client.Close()

	userData := map[string]interface{}{
		"name":  name,
		"email": email,
	}
	userJSON, _ := json.Marshal(userData)
	
	// Сохраняем пользователя с ключом user:{userID}
	client.Set(ctx, "user:"+userID, userJSON, 0)
	
	// Добавляем в список всех пользователей
	client.SAdd(ctx, "users:all", userID)
}

// Получение данных текущего пользователя
func getCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	
	user := map[string]string{
		"id":    session.Values["user_id"].(string),
		"name":  session.Values["user"].(string),
		"email": session.Values["email"].(string),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// Получение всех пользователей (кроме текущего и друзей)
func getAllUsersHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	currentUserID := session.Values["user_id"].(string)

	ctx := context.Background()
	client := getRedisClient()
	defer client.Close()

	// Получаем список друзей
	friendIDs, _ := client.SMembers(ctx, "friends:"+currentUserID).Result()
	friendMap := make(map[string]bool)
	for _, id := range friendIDs {
		friendMap[id] = true
	}

	// Получаем всех пользователей
	userIDs, _ := client.SMembers(ctx, "users:all").Result()
	
	var users []map[string]string
	for _, userID := range userIDs {
		if userID == currentUserID || friendMap[userID] {
			continue // Пропускаем текущего пользователя и друзей
		}
		
		userData, err := client.Get(ctx, "user:"+userID).Result()
		if err != nil {
			continue
		}
		
		var userInfo map[string]string
		json.Unmarshal([]byte(userData), &userInfo)
		userInfo["id"] = userID
		users = append(users, userInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// Поиск пользователей
func searchUsersHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	currentUserID := session.Values["user_id"].(string)
	
	query := r.URL.Query().Get("q")
	if query == "" {
		query = ""
	}

	ctx := context.Background()
	client := getRedisClient()
	defer client.Close()

	// Получаем список друзей
	friendIDs, _ := client.SMembers(ctx, "friends:"+currentUserID).Result()
	friendMap := make(map[string]bool)
	for _, id := range friendIDs {
		friendMap[id] = true
	}

	// Получаем всех пользователей
	userIDs, _ := client.SMembers(ctx, "users:all").Result()
	
	var users []map[string]string
	for _, userID := range userIDs {
		if userID == currentUserID || friendMap[userID] {
			continue // Пропускаем текущего пользователя и друзей
		}
		
		userData, err := client.Get(ctx, "user:"+userID).Result()
		if err != nil {
			continue
		}
		
		var userInfo map[string]string
		json.Unmarshal([]byte(userData), &userInfo)
		
		// Фильтрация по запросу (если есть)
		if query == "" || 
		   contains(userInfo["name"], query) || 
		   contains(userInfo["email"], query) {
			userInfo["id"] = userID
			users = append(users, userInfo)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// Получение списка друзей
func getFriendsHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := context.Background()
	client := getRedisClient()
	defer client.Close()

	// Получаем список друзей
	friendIDs, _ := client.SMembers(ctx, "friends:"+userID).Result()
	
	var friends []map[string]string
	for _, friendID := range friendIDs {
		userData, err := client.Get(ctx, "user:"+friendID).Result()
		if err != nil {
			continue
		}
		
		var userInfo map[string]string
		json.Unmarshal([]byte(userData), &userInfo)
		userInfo["id"] = friendID
		friends = append(friends, userInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(friends)
}

// Добавление друга
func addFriendHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		FriendID string `json:"friend_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	ctx := context.Background()
	client := getRedisClient()
	defer client.Close()

	// Добавляем друга (двусторонняя связь)
	client.SAdd(ctx, "friends:"+userID, req.FriendID)
	client.SAdd(ctx, "friends:"+req.FriendID, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Friend added"})
}

// Удаление друга
func removeFriendHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	friendID := r.URL.Query().Get("friend_id")
	if friendID == "" {
		http.Error(w, "friend_id is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	client := getRedisClient()
	defer client.Close()

	// Удаляем друга (двусторонняя связь)
	client.SRem(ctx, "friends:"+userID, friendID)
	client.SRem(ctx, "friends:"+friendID, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Friend removed"})
}

// Получение общих списков (списки, которыми поделились с пользователем)
func getSharedListsHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := context.Background()
	client := getRedisClient()
	defer client.Close()

	// Получаем списки, которыми поделились с пользователем
	sharedLists, _ := client.SMembers(ctx, "shared_lists:"+userID).Result()
	
	var lists []map[string]string
	for _, listKey := range sharedLists {
		// listKey имеет формат "owner_id:купить"
		parts := splitListKey(listKey)
		if len(parts) == 2 {
			ownerID := parts[0]
			category := parts[1]
			
			ownerData, _ := client.Get(ctx, "user:"+ownerID).Result()
			var ownerInfo map[string]string
			json.Unmarshal([]byte(ownerData), &ownerInfo)
			
			lists = append(lists, map[string]string{
				"owner_id": ownerID,
				"owner_name": ownerInfo["name"],
				"category": category,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lists)
}

// Поделиться списком с другом
func shareListHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		FriendID string `json:"friend_id"`
		Category string `json:"category"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Можно делиться только списком "купить"
	if req.Category != "купить" {
		http.Error(w, "Можно делиться только списком 'купить'", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	client := getRedisClient()
	defer client.Close()

	// Копируем текущий список в общий список
	personalKey := "shoppingList:" + userID + ":" + req.Category
	personalVal, err := client.Get(ctx, personalKey).Result()
	if err == nil && personalVal != "" {
		// Проверяем, что это валидный JSON
		var items []Item
		if err := json.Unmarshal([]byte(personalVal), &items); err == nil {
			// Сохраняем копию списка для друга
			sharedKey := "shoppingList:" + userID + ":" + req.Category
			client.Set(ctx, sharedKey, personalVal, 0)
		} else {
			log.Printf("Ошибка парсинга JSON для %s: %v", personalKey, err)
		}
	}

	// Добавляем в список общих списков друга
	listKey := userID + ":" + req.Category
	client.SAdd(ctx, "shared_lists:"+req.FriendID, listKey)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "List shared"})
}

// Вспомогательные функции
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func splitListKey(key string) []string {
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{key}
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

// Внутренние обработчики для сервисов (без авторизации)
// Используют дефолтного пользователя или user_id из заголовка X-User-ID
func getServiceUserID(r *http.Request) string {
	// Пытаемся получить user_id из заголовка
	userID := r.Header.Get("X-User-ID")
	if userID != "" {
		return userID
	}
	// Дефолтный user_id для сервисов
	return os.Getenv("SERVICE_USER_ID")
}

func internalListHandler(w http.ResponseWriter, r *http.Request) {
	userID := getServiceUserID(r)
	if userID == "" {
		// Если SERVICE_USER_ID не настроен, используем дефолтное значение "service"
		// Это позволит сервисам работать без настройки
		userID = "service"
		log.Printf("SERVICE_USER_ID не настроен, используется дефолтный: service")
	}
	
	ctx := r.Context()
	client := getRedisClient()
	defer client.Close()

	category := r.URL.Query().Get("category")
	if category == "" {
		http.Error(w, "Category is required", http.StatusBadRequest)
		return
	}

	key := "shoppingList:" + userID + ":" + category
	val, err := client.Get(ctx, key).Result()
	if err == redis.Nil || val == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Item{})
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	err = json.Unmarshal([]byte(val), &items)
	if err != nil {
		log.Printf("Ошибка парсинга JSON для %s: %v, значение: %s", key, err, val)
		// Возвращаем пустой список вместо ошибки
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Item{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func internalAddHandler(w http.ResponseWriter, r *http.Request) {
	userID := getServiceUserID(r)
	if userID == "" {
		userID = "service"
		log.Printf("SERVICE_USER_ID не настроен, используется дефолтный: service")
	}
	
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

	key := "shoppingList:" + userID + ":" + newItem.Category
	val, err := client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	if err == nil && val != "" {
		err = json.Unmarshal([]byte(val), &items)
		if err != nil {
			log.Printf("Ошибка парсинга JSON: %v, значение: %s", err, val)
			// Используем пустой список при ошибке парсинга
			items = []Item{}
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

func internalBuyHandler(w http.ResponseWriter, r *http.Request) {
	userID := getServiceUserID(r)
	if userID == "" {
		userID = "service"
		log.Printf("SERVICE_USER_ID не настроен, используется дефолтный: service")
	}
	
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

	key := "shoppingList:" + userID + ":" + item.Category
	val, err := client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	if err == nil && val != "" {
		err = json.Unmarshal([]byte(val), &items)
		if err != nil {
			log.Printf("Ошибка парсинга JSON: %v, значение: %s", err, val)
			// Используем пустой список при ошибке парсинга
			items = []Item{}
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

func internalDeleteHandler(w http.ResponseWriter, r *http.Request) {
	userID := getServiceUserID(r)
	if userID == "" {
		userID = "service"
		log.Printf("SERVICE_USER_ID не настроен, используется дефолтный: service")
	}
	
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

	key := "shoppingList:" + userID + ":" + category
	val, err := client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []Item
	if err == nil && val != "" {
		err = json.Unmarshal([]byte(val), &items)
		if err != nil {
			log.Printf("Ошибка парсинга JSON: %v, значение: %s", err, val)
			// Используем пустой список при ошибке парсинга
			items = []Item{}
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

func internalEditHandler(w http.ResponseWriter, r *http.Request) {
	userID := getServiceUserID(r)
	if userID == "" {
		userID = "service"
		log.Printf("SERVICE_USER_ID не настроен, используется дефолтный: service")
	}
	
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

	oldKey := "shoppingList:" + userID + ":" + oldCategory
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

	newKey := "shoppingList:" + userID + ":" + editedItem.Category
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

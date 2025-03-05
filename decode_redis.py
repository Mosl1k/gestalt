import redis
import json
import os
from dotenv import load_dotenv

# Загрузка переменных окружения из .env файла
load_dotenv()

# Получение пароля из переменной окружения
redis_password = os.getenv('REDIS_PASSWORD')

# Подключение к Redis с использованием пароля
client = redis.StrictRedis(host='localhost', port=6379, password=redis_password, decode_responses=True)

# Получение всех ключей с префиксом 'shoppingList:*'
keys = client.keys('shoppingList:*')

# Проверка, есть ли ключи
if keys:
    # Проходим по каждому ключу
    for key in keys:
        raw_data = client.get(key)  # Получаем значение ключа
        
        print(f"Ключ: {key}")
        
        if raw_data:
            try:
                # Пытаемся декодировать как JSON
                shopping_list = json.loads(raw_data)
                print(json.dumps(shopping_list, ensure_ascii=False, indent=4))
            except json.JSONDecodeError:
                # Если не JSON, выводим как строку
                print(f"Значение (строка): {raw_data}")
        else:
            print("Значение отсутствует.")
        print("-" * 40)  # Разделитель между ключами
else:
    print("Нет ключей с префиксом 'shoppingList:*'.")
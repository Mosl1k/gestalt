import redis
import json
import os
from dotenv import load_dotenv

# Загрузка переменных окружения из .env файла
load_dotenv()

# Получение пароля из переменной окружения
redis_password = os.getenv('REDIS_PASSWORD')

# Подключение к Redis с использованием пароля
client = redis.StrictRedis(host='geshtalt.ddns.net', port=6379, password=redis_password, decode_responses=True)

# Получение данных
raw_data = client.get('shoppingList')

# Проверка, что данные не пустые
if raw_data:
    try:
        # Конвертация строки в JSON-объект
        shopping_list = json.loads(raw_data)
        
        # Вывод данных
        print(json.dumps(shopping_list, ensure_ascii=False, indent=4))
    except json.JSONDecodeError as e:
        print(f"Ошибка при декодировании JSON: {e}")
else:
    print("Нет данных в ключе 'shoppingList'.")
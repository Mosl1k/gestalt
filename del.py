import os
import redis
import json
from dotenv import load_dotenv


# Получение переменных
REDIS_HOST = 'geshtalt.ddns.net'
REDIS_PORT = 6379
REDIS_PASSWORD=""

# Подключение к Redis
client = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, password=REDIS_PASSWORD, db=0)

# Пример удаления пустой записи из списка покупок
current_list = client.get('shoppingList')

if current_list:
    items = json.loads(current_list)
    for item in items:
        if item['name'] == "":
            items.remove(item)

    client.set('shoppingList', json.dumps(items))

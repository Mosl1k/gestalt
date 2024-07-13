import redis
import json

# Подключение к Redis с использованием пароля
redis_client = redis.StrictRedis(
    host='localhost',
    port=6379,
    password='s!mpleRed1sP@$',  # Замените 'your_redis_password' на ваш реальный пароль
    db=0
)

# Получение данных по ключу
data_from_redis = redis_client.get('shoppingList')

if data_from_redis:
    decoded_data = json.loads(data_from_redis.decode('utf-8'))
    print("Данные в Redis:")
    print(json.dumps(decoded_data, indent=4, ensure_ascii=False))
else:
    print("Ключ 'shoppingList' не найден или пуст")

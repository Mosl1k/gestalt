from flask import Flask, request, jsonify
import requests
import g4f
import os
import base64

app = Flask(__name__)

# Хост и порт для обращения к geshtalt внутри Docker-сети
GESHTALT_HOST = 'geshtalt'
GESHTALT_PORT = '8080'
BASE_URL = f'http://{GESHTALT_HOST}:{GESHTALT_PORT}'

# Получаем логин и пароль из .env
USERNAME = os.getenv("USERNAME")
PASSWORD = os.getenv("PASSWORD")

# Формируем заголовок Basic Auth
AUTH_HEADER = {
    "Authorization": f"Basic {base64.b64encode(f'{USERNAME}:{PASSWORD}'.encode()).decode()}",
    "Content-Type": "application/json"
}

def add_to_shopping_list(item_name, category):
    url = f'{BASE_URL}/add'
    payload = {
        "name": item_name,
        "category": category
    }
    response = requests.post(url, json=payload, headers=AUTH_HEADER)
    return response.json()

def get_list_by_category(category):
    url = f'{BASE_URL}/list?category={category}'
    response = requests.get(url, headers=AUTH_HEADER)
    if response.status_code == 200:
        data = response.json()
        print(f"Полученные данные для категории '{category}': {data}")
        # Фильтруем элементы по категории (на случай, если сервер не делает это сам)
        filtered_items = [item['name'] for item in data if item.get('category', '').lower() == category.lower()]
        print(f"Элементы в категории '{category}': {filtered_items}")
        return filtered_items
    else:
        print(f"Ошибка получения данных: {response.status_code}, текст ошибки: {response.text}")
        return []

@app.route('/', methods=['POST'])
def webhook():
    data = request.json
    print("Request received: ", data)
    
    command = data['request']['command'].strip().lower()
    
    if command == '':
        response_text = "Привет, что нужно сделать?"
    elif command.startswith('запиши') or command.startswith('записать'):
        item_to_record = ' '.join(data['request']['nlu']['tokens'][1:])
        print(f"Записано в список 'не забыть': '{item_to_record}'")
        
        api_response = add_to_shopping_list(item_to_record, 'не-забыть')
        print("API response:", api_response)
        
        response_text = f"Записано в список 'не забыть': '{item_to_record}'"
    elif command.startswith('купить'):
        item_to_buy = ' '.join(data['request']['nlu']['tokens'][1:])
        print(f"Записано в список 'купить': '{item_to_buy}'")
        
        api_response = add_to_shopping_list(item_to_buy, 'купить')
        print("API response:", api_response)
        
        response_text = f"Записано в список 'купить': '{item_to_buy}'"
    elif command == 'что купить':
        items_to_buy = get_list_by_category('купить')
        if items_to_buy:
            response_text = "В списке 'купить': " + ', '.join(items_to_buy)
        else:
            response_text = "Список 'купить' пуст."
    elif command == 'что не забыть':
        items_to_remember = get_list_by_category('не-забыть')
        if items_to_remember:
            response_text = "В списке 'не забыть': " + ', '.join(items_to_remember)
        else:
            response_text = "Список 'не забыть' пуст."
    elif command == 'что в холодильнике':
        items_in_fridge = get_list_by_category('холодос')
        if items_in_fridge:
            response_text = "В холодильнике: " + ', '.join(items_in_fridge)
        else:
            response_text = "В холодильнике пусто."
    elif command == 'что приготовить':
        items_in_fridge = get_list_by_category('холодос')
        if items_in_fridge:
            print(f"Продукты в холодильнике: {items_in_fridge}")
            prompt = f"Что можно приготовить из таких продуктов: {', '.join(items_in_fridge)}? Назови только 5 названий блюд."
            try:
                response_from_gpt = g4f.ChatCompletion.create(model='gpt-4', messages=[{"role": "user", "content": prompt}])
                print(f"Ответ от GPT: {response_from_gpt}")
                response_text = f"{response_from_gpt}"
            except Exception as e:
                print(f"Ошибка при обращении к GPT: {e}")
                response_text = "Извините, произошла ошибка при запросе рецепта."
        else:
            response_text = "В холодильнике пусто, нечего приготовить."
    else:
        response_text = "Извините, я не понимаю эту команду."

    response = {
        "version": "1.0",
        "session": {
            "message_id": data['session']['message_id'],
            "session_id": data['session']['session_id'],
            "skill_id": data['session']['skill_id'],
            "user_id": data['session']['user_id']
        },
        "response": {
            "text": response_text,
            "end_session": False
        }
    }
    
    print("Response to be sent: ", response)
    
    return jsonify(response)

if __name__ == '__main__':
    context = ('/etc/letsencrypt/live/kpalch.ru/fullchain.pem', '/etc/letsencrypt/live/kpalch.ru/privkey.pem')
    app.run(host='0.0.0.0', port=2112, ssl_context=context)

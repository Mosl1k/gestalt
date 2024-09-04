from flask import Flask, request, jsonify
import requests

app = Flask(__name__)

def add_to_shopping_list(item_name, category):
    url = 'http://geshtalt.ddns.net:8080/add'
    payload = {
        "name": item_name,
        "category": category
    }
    headers = {
        "Content-Type": "application/json"
    }
    response = requests.post(url, json=payload, headers=headers)
    return response.json()

def fetch_shopping_list():
    url = 'http://geshtalt.ddns.net:8080/list'
    response = requests.get(url)
    if response.status_code == 200:
        data = response.json()
        print(f"Полученные данные: {data}")  # Логируем полученные данные для отладки
        return data
    else:
        print(f"Ошибка получения данных: {response.status_code}, текст ошибки: {response.text}")
        return []

def get_list_by_category(category):
    shopping_list = fetch_shopping_list()
    print(f"Список покупок: {shopping_list}")  # Логируем полный список покупок
    # Приводим категорию и названия к нижнему регистру для корректного сравнения
    filtered_items = [item['name'] for item in shopping_list if item.get('category', '').lower() == category.lower()]
    print(f"Элементы в категории '{category}': {filtered_items}")  # Логируем отфильтрованные элементы
    return filtered_items

@app.route('/', methods=['POST'])
def webhook():
    data = request.json
    print("Request received: ", data)
    
    command = data['request']['command'].strip().lower()
    
    if command == '':
        response_text = "Привет, что нужно сделать?"
    elif command.startswith('запиши'):
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
    
    print("Response to be sent: ", response)  # Логируем формируемый ответ
    
    return jsonify(response)

if __name__ == '__main__':
    context = ('cert.pem', 'key.pem')
    app.run(host='0.0.0.0', port=2112, ssl_context=context)
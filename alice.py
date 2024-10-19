from flask import Flask, request, jsonify
import requests
import g4f

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
            # Формируем запрос к gpt через g4f
            prompt = f"Что можно приготовить из таких продуктов: {', '.join(items_in_fridge)}?"
            try:
                # Отправляем запрос через g4f
                response_from_gpt = g4f.ChatCompletion.create(model='gpt-4', messages=[{"role": "user", "content": prompt}])
                print(f"Ответ от GPT: {response_from_gpt}")
                
                # Проверим формат ответа
                if isinstance(response_from_gpt, dict) and 'choices' in response_from_gpt:
                    recipe = response_from_gpt['choices'][0].get('message', {}).get('content', 'Не удалось получить рецепт.')
                else:
                    recipe = "Не удалось получить корректный ответ от GPT."
                
                print(f"Рецепт: {recipe}")
                response_text = f"Вот что можно приготовить: {recipe}"
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
    
    print("Response to be sent: ", response)  # Логируем формируемый ответ
    
    return jsonify(response)

if __name__ == '__main__':
    context = ('cert.pem', 'key.pem')
    app.run(host='0.0.0.0', port=2112, ssl_context=context)
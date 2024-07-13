from flask import Flask, request, jsonify
import requests
import os

app = Flask(__name__)

def add_to_shopping_list(item_name, category):
    url = 'https://geshtalt.ddns.net:443/add'
    payload = {
        "name": item_name,
        "category": category
    }
    headers = {
        "Content-Type": "application/json"
    }
    response = requests.post(url, json=payload, headers=headers)
    return response.json()

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
        
        api_response = add_to_shopping_list(item_to_record, 'не забыть')
        print("API response:", api_response)
        
        response_text = f"Записано в список 'не забыть': '{item_to_record}'"
    elif command.startswith('купить'):
        item_to_buy = ' '.join(data['request']['nlu']['tokens'][1:])
        print(f"Записано в список 'купить': '{item_to_buy}'")
        
        api_response = add_to_shopping_list(item_to_buy, 'купить')
        print("API response:", api_response)
        
        response_text = f"Записано в список 'купить': '{item_to_buy}'"
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
    
    return jsonify(response)

if __name__ == '__main__':
    context = ('cert.pem', 'key.pem')
    app.run(host='0.0.0.0', port=2112, ssl_context=context)
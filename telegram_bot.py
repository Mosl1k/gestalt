import os
import json
import requests
from base64 import b64encode
from dotenv import load_dotenv
from telegram import InlineKeyboardButton, InlineKeyboardMarkup, Update
from telegram.ext import Application, CommandHandler, CallbackQueryHandler, MessageHandler, filters

# Загружаем переменные из .env
load_dotenv()

# Конфигурация из .env
TELEGRAM_TOKEN = os.getenv("TELEGRAM_TOKEN")  # TELEGRAM_TOKEN=your_bot_token
API_URL = os.getenv("API_URL", "http://gestalt:8080")  # API_URL=http://gestalt:8080
USERNAME = os.getenv("USERNAME")  # USERNAME=your_username
PASSWORD = os.getenv("PASSWORD")  # PASSWORD=your_password

# Категории для списков
LISTS = {
    "купить": "Покупки",
    "не-забыть": "Не забыть",
    "холодос": "Холодильник",
    "дом": "Дом"
}

def get_main_keyboard():
    """Возвращает основную клавиатуру с категориями, Добавить и Рестарт."""
    keyboard = [
        [InlineKeyboardButton(name, callback_data=f"list:{key}")]
        for key, name in LISTS.items()
    ]
    keyboard.append([InlineKeyboardButton("Добавить", callback_data="add")])
    keyboard.append([InlineKeyboardButton("Рестарт", callback_data="restart")])
    return InlineKeyboardMarkup(keyboard)

async def start(update: Update, context):
    """Обработчик команды /start. Показывает основную клавиатуру."""
    reply_markup = get_main_keyboard()
    if isinstance(update, Update) and update.message:
        await update.message.reply_text("Выберите категорию:", reply_markup=reply_markup)
    else:
        await update.edit_message_text("Выберите категорию:", reply_markup=reply_markup)

async def add_start(update: Update, context):
    """Обработчик кнопки Добавить. Запрашивает текст элемента."""
    query = update.callback_query
    await query.answer()
    context.user_data["awaiting_item"] = True
    await query.message.reply_text("Введите название элемента для добавления:")

async def handle_item_text(update: Update, context):
    """Обработчик ввода текста элемента. Показывает кнопки для выбора категории."""
    if not context.user_data.get("awaiting_item"):
        return

    item_name = update.message.text
    context.user_data["item_name"] = item_name
    context.user_data["awaiting_item"] = False
    context.user_data["awaiting_category"] = True

    keyboard = [
        [InlineKeyboardButton(name, callback_data=f"add_to:{key}")]
        for key, name in LISTS.items()
    ]
    reply_markup = InlineKeyboardMarkup(keyboard)
    await update.message.reply_text(f"Куда добавить '{item_name}'?", reply_markup=reply_markup)

async def add_to_category(update: Update, context):
    """Обработчик выбора категории для добавления."""
    query = update.callback_query
    await query.answer()

    if not context.user_data.get("awaiting_category"):
        return

    data = query.data
    if not data.startswith("add_to:"):
        return

    category = data.split(":")[1]
    if category not in LISTS:
        await query.message.reply_text(f"Неизвестная категория: {category}")
        return

    item_name = context.user_data.get("item_name")
    context.user_data["awaiting_category"] = False
    context.user_data.pop("item_name", None)

    try:
        # Формируем Basic Auth заголовок
        auth_str = f"{USERNAME}:{PASSWORD}"
        auth_header = {"Authorization": f"Basic {b64encode(auth_str.encode()).decode()}", "Content-Type": "application/json"}

        # Формируем тело запроса
        data = {
            "name": item_name,
            "category": category,
            "bought": False,
            "priority": 2
        }

        # Отправляем запрос к API
        response = requests.post(f"{API_URL}/add", headers=auth_header, json=data)
        if response.status_code != 201:
            await query.message.reply_text(f"Ошибка добавления: {response.status_code}")
            return

        reply_markup = get_main_keyboard()
        await query.message.reply_text(f"Добавлено '{item_name}' в {LISTS[category]}", reply_markup=reply_markup)
    except requests.RequestException as e:
        await query.message.reply_text(f"Ошибка подключения к API: {e}")

async def button_callback(update: Update, context):
    """Обработчик нажатий на кнопки."""
    query = update.callback_query
    await query.answer()

    data = query.data
    if data == "restart":
        await start(query, context)
        return
    if data == "add":
        await add_start(update, context)
        return
    if data.startswith("add_to:"):
        await add_to_category(update, context)
        return
    if data.startswith("list:"):
        list_type = data.split(":")[1]
        if list_type not in LISTS:
            await query.message.reply_text(f"Неизвестная категория: {list_type}")
            return

        try:
            # Формируем Basic Auth заголовок
            auth_str = f"{USERNAME}:{PASSWORD}"
            auth_header = {"Authorization": f"Basic {b64encode(auth_str.encode()).decode()}"}

            # Запрос к API
            response = requests.get(f"{API_URL}/list?category={list_type}", headers=auth_header)
            if response.status_code != 200:
                await query.message.reply_text(f"Ошибка API: {response.status_code}")
                return

            items = response.json()
            if not items:
                response_text = f"{LISTS[list_type]} пуст."
            else:
                response_text = f"{LISTS[list_type]}:\n" + "\n".join([f"- {item['name']}" for item in items])

            reply_markup = get_main_keyboard()
            await query.message.reply_text(response_text, reply_markup=reply_markup)
        except requests.RequestException as e:
            await query.message.reply_text(f"Ошибка подключения к API: {e}")
        except json.JSONDecodeError:
            await query.message.reply_text("Ошибка: неверный формат данных от API.")

def main():
    """Запуск бота."""
    if not TELEGRAM_TOKEN:
        print("Ошибка: TELEGRAM_TOKEN не указан в .env")
        return
    if not USERNAME or not PASSWORD:
        print("Ошибка: USERNAME или PASSWORD не указаны в .env")
        return
    if not API_URL:
        print("Ошибка: API_URL не указан в .env")
        return

    application = Application.builder().token(TELEGRAM_TOKEN).build()

    # Добавляем обработчики
    application.add_handler(CommandHandler("start", start))
    application.add_handler(CallbackQueryHandler(button_callback))
    application.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_item_text))

    # Запускаем бота
    print("Бот запущен...")
    application.run_polling(allowed_updates=Update.ALL_TYPES)

if __name__ == "__main__":
    main()
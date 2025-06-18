import os
import json
import requests
from base64 import b64encode
from dotenv import load_dotenv
from telegram import InlineKeyboardButton, InlineKeyboardMarkup, Update
from telegram.ext import Application, CommandHandler, CallbackQueryHandler, MessageHandler, filters
import logging

# Настройка логирования
logging.basicConfig(
    filename='bot.log',
    level=logging.ERROR,
    format='%(asctime)s - %(levelname)s - %(message)s'
)

# Загружаем переменные из .env
load_dotenv()

# Конфигурация из .env
TELEGRAM_TOKEN = os.getenv("TELEGRAM_TOKEN")
API_URL = os.getenv("API_URL", "http://geshtalt:8080")
USERNAME = os.getenv("USERNAME")
PASSWORD = os.getenv("PASSWORD")

# Категории для списков
LISTS = {
    "купить": "Покупки",
    "не-забыть": "Не забыть",
    "холодос": "Холодильник",
    "дом": "Дом",
    "машина": "Машина"
}

# Эмодзи для приоритетов
PRIORITY_EMOJI = {
    3: "🔥",  # Высокий
    2: "🟡",  # Средний
    1: "🟢"   # Низкий
}

# Порядок категорий для переключения
CATEGORIES = list(LISTS.keys())

def get_main_keyboard():
    """Возвращает основную клавиатуру с категориями."""
    keyboard = [
        [InlineKeyboardButton(name, callback_data=f"list:{key}")]
        for key, name in LISTS.items()
    ]
    keyboard.append([InlineKeyboardButton("Рестарт", callback_data="restart")])
    return InlineKeyboardMarkup(keyboard)

def get_list_keyboard(current_category):
    """Возвращает клавиатуру для списка с кнопками Назад, Предыдущий, Следующий, Добавить."""
    current_index = CATEGORIES.index(current_category)
    prev_category = CATEGORIES[(current_index - 1) % len(CATEGORIES)]
    next_category = CATEGORIES[(current_index + 1) % len(CATEGORIES)]

    keyboard = [
        [InlineKeyboardButton("Назад", callback_data="restart")],
        [
            InlineKeyboardButton("Предыдущий", callback_data=f"list:{prev_category}"),
            InlineKeyboardButton("Следующий", callback_data=f"list:{next_category}")
        ],
        [InlineKeyboardButton("Добавить", callback_data=f"add:{current_category}")]
    ]
    return InlineKeyboardMarkup(keyboard)

async def start(update: Update, context):
    """Обработчик команды /start. Показывает основную клавиатуру."""
    reply_markup = get_main_keyboard()
    if isinstance(update, Update) and update.message:
        await update.message.reply_text("Выберите категорию:", reply_markup=reply_markup)
    else:
        await update.callback_query.message.edit_text("Выберите категорию:", reply_markup=reply_markup)

async def add_start(update: Update, context):
    """Обработчик кнопки Добавить. Запрашивает текст элемента."""
    query = update.callback_query
    await query.answer()
    data = query.data
    category = data.split(":")[1] if data.startswith("add:") else None
    context.user_data["awaiting_item"] = True
    context.user_data["category"] = category
    await query.message.reply_text(
        f"Введите название элемента для добавления в {LISTS[category]}:"
    )

async def handle_item_text(update: Update, context):
    """Обработчик ввода текста элемента."""
    if not context.user_data.get("awaiting_item"):
        return

    item_name = update.message.text
    context.user_data["item_name"] = item_name
    context.user_data["awaiting_item"] = False

    if context.user_data.get("category"):
        await add_to_category(update, context)
    else:
        context.user_data["awaiting_category"] = True
        keyboard = [
            [InlineKeyboardButton(name, callback_data=f"add_to:{key}")]
            for key, name in LISTS.items()
        ]
        reply_markup = InlineKeyboardMarkup(keyboard)
        await update.message.reply_text(f"Куда добавить '{item_name}'?", reply_markup=reply_markup)

async def add_to_category(update: Update, context):
    """Обработчик выбора категории для добавления."""
    category = context.user_data.get("category")
    if not category:
        query = update.callback_query
        await query.answer()
        data = query.data
        if not data.startswith("add_to:"):
            return
        category = data.split(":")[1]

    if category not in LISTS:
        await update.message.reply_text(f"Неизвестная категория: {category}")
        return

    item_name = context.user_data.get("item_name")
    context.user_data["awaiting_category"] = False
    context.user_data.pop("item_name", None)
    context.user_data.pop("category", None)

    try:
        auth_str = f"{USERNAME}:{PASSWORD}"
        auth_header = {"Authorization": f"Basic {b64encode(auth_str.encode()).decode()}", "Content-Type": "application/json"}

        data = {
            "name": item_name,
            "category": category,
            "bought": False,
            "priority": 2
        }

        response = requests.post(f"{API_URL}/add", headers=auth_header, json=data)
        if response.status_code != 201:
            error_msg = f"Ошибка добавления: {response.status_code} - {response.text}"
            logging.error(error_msg)
            await update.message.reply_text(error_msg)
            return

        reply_markup = get_list_keyboard(category)
        await update.message.reply_text(f"Добавлено '{item_name}' в {LISTS[category]}", reply_markup=reply_markup)
    except requests.RequestException as e:
        error_msg = f"Ошибка подключения к API: {e}"
        logging.error(error_msg)
        await update.message.reply_text(error_msg)
    except Exception as e:
        error_msg = f"Произошла ошибка: {str(e)}"
        logging.error(error_msg)
        await update.message.reply_text(error_msg)

async def show_item_actions(update: Update, context):
    """Показывает действия для выбранного элемента."""
    query = update.callback_query
    await query.answer()

    data = query.data
    if not data.startswith("item:"):
        return

    item_name, category = data.split(":")[1:3]
    context.user_data["item_name"] = item_name
    context.user_data["category"] = category

    keyboard = [
        [InlineKeyboardButton("Удалить", callback_data=f"item_action:delete:{item_name}:{category}")],
        [InlineKeyboardButton("Сменить категорию", callback_data=f"item_action:change_cat:{item_name}:{category}")],
        [InlineKeyboardButton("Сменить приоритет", callback_data=f"item_action:change_pri:{item_name}:{category}")],
        [InlineKeyboardButton("Назад", callback_data=f"list:{category}")]
    ]
    reply_markup = InlineKeyboardMarkup(keyboard)
    await query.message.reply_text(f"Что сделать с '{item_name}' в {LISTS[category]}?", reply_markup=reply_markup)

async def handle_item_action(update: Update, context):
    """Обработчик действий с элементом."""
    query = update.callback_query
    await query.answer()

    data = query.data
    if not data.startswith("item_action:"):
        return

    action, item_name, category = data.split(":")[1:4]
    context.user_data["item_name"] = item_name
    context.user_data["category"] = category

    if action == "delete":
        try:
            auth_str = f"{USERNAME}:{PASSWORD}"
            auth_header = {"Authorization": f"Basic {b64encode(auth_str.encode()).decode()}"}

            response = requests.delete(f"{API_URL}/delete/{item_name}?category={category}", headers=auth_header)
            if response.status_code != 200:
                error_msg = f"Ошибка удаления: {response.status_code} - {response.text}"
                logging.error(error_msg)
                await query.message.reply_text(error_msg)
                return

            reply_markup = get_list_keyboard(category)
            await query.message.reply_text(f"Удалено '{item_name}' из {LISTS[category]}", reply_markup=reply_markup)
        except requests.RequestException as e:
            error_msg = f"Ошибка подключения к API: {e}"
            logging.error(error_msg)
            await query.message.reply_text(error_msg)
        except Exception as e:
            error_msg = f"Произошла ошибка: {str(e)}"
            logging.error(error_msg)
            await query.message.reply_text(error_msg)
    elif action == "change_cat":
        context.user_data["awaiting_new_category"] = True
        keyboard = [
            [InlineKeyboardButton(name, callback_data=f"change_cat_to:{key}")]
            for key, name in LISTS.items() if key != category
        ]
        reply_markup = InlineKeyboardMarkup(keyboard)
        await query.message.reply_text(f"В какую категорию перенести '{item_name}'?", reply_markup=reply_markup)
    elif action == "change_pri":
        context.user_data["awaiting_priority"] = True
        keyboard = [
            [InlineKeyboardButton(f"Высокий {PRIORITY_EMOJI[3]}", callback_data="pri:3")],
            [InlineKeyboardButton(f"Средний {PRIORITY_EMOJI[2]}", callback_data="pri:2")],
            [InlineKeyboardButton(f"Низкий {PRIORITY_EMOJI[1]}", callback_data="pri:1")]
        ]
        reply_markup = InlineKeyboardMarkup(keyboard)
        await query.message.reply_text(f"Выберите новый приоритет для '{item_name}' в {LISTS[category]}:", reply_markup=reply_markup)

async def change_category_to(update: Update, context):
    """Обработчик выбора новой категории."""
    query = update.callback_query
    await query.answer()

    if not context.user_data.get("awaiting_new_category"):
        return

    data = query.data
    if not data.startswith("change_cat_to:"):
        return

    new_category = data.split(":")[1]
    if new_category not in LISTS:
        await query.message.reply_text(f"Неизвестная категория: {new_category}")
        return

    item_name = context.user_data.get("item_name")
    old_category = context.user_data.get("category")
    context.user_data["awaiting_new_category"] = False
    context.user_data.pop("item_name", None)
    context.user_data.pop("category", None)

    try:
        auth_str = f"{USERNAME}:{PASSWORD}"
        auth_header = {"Authorization": f"Basic {b64encode(auth_str.encode()).decode()}", "Content-Type": "application/json"}

        response = requests.get(f"{API_URL}/list?category={old_category}", headers=auth_header)
        if response.status_code != 200:
            error_msg = f"Ошибка получения данных: {response.status_code} - {response.text}"
            logging.error(error_msg)
            await query.message.reply_text(error_msg)
            return

        items = response.json()
        item = next((i for i in items if i["name"] == item_name), None)
        if not item:
            await query.message.reply_text(f"Элемент '{item_name}' не найден в {LISTS[old_category]}")
            return

        data = {
            "name": item_name,
            "category": new_category,
            "bought": item["bought"],
            "priority": item["priority"]
        }

        response = requests.put(f"{API_URL}/edit/{item_name}?oldCategory={old_category}", headers=auth_header, json=data)
        if response.status_code != 200:
            error_msg = f"Ошибка смены категории: {response.status_code} - {response.text}"
            logging.error(error_msg)
            await query.message.reply_text(error_msg)
            return

        reply_markup = get_list_keyboard(new_category)
        await query.message.reply_text(f"Элемент '{item_name}' перенесен из {LISTS[old_category]} в {LISTS[new_category]}", reply_markup=reply_markup)
    except requests.RequestException as e:
        error_msg = f"Ошибка подключения к API: {e}"
        logging.error(error_msg)
        await query.message.reply_text(error_msg)
    except Exception as e:
        error_msg = f"Произошла ошибка: {str(e)}"
        logging.error(error_msg)
        await query.message.reply_text(error_msg)

async def change_priority_to(update: Update, context):
    """Обработчик выбора нового приоритета."""
    query = update.callback_query
    await query.answer()

    if not context.user_data.get("awaiting_priority"):
        return

    data = query.data
    if not data.startswith("pri:"):
        return

    new_priority = int(data.split(":")[1])
    item_name = context.user_data.get("item_name")
    category = context.user_data.get("category")
    context.user_data["awaiting_priority"] = False
    context.user_data.pop("item_name", None)
    context.user_data.pop("category", None)

    try:
        auth_str = f"{USERNAME}:{PASSWORD}"
        auth_header = {"Authorization": f"Basic {b64encode(auth_str.encode()).decode()}", "Content-Type": "application/json"}

        response = requests.get(f"{API_URL}/list?category={category}", headers=auth_header)
        if response.status_code != 200:
            error_msg = f"Ошибка получения данных: {response.status_code} - {response.text}"
            logging.error(error_msg)
            await query.message.reply_text(error_msg)
            return

        items = response.json()
        item = next((i for i in items if i["name"] == item_name), None)
        if not item:
            await query.message.reply_text(f"Элемент '{item_name}' не найден в {LISTS[category]}")
            return

        data = {
            "name": item_name,
            "category": category,
            "bought": item["bought"],
            "priority": new_priority
        }

        response = requests.put(f"{API_URL}/edit/{item_name}?oldCategory={category}", headers=auth_header, json=data)
        if response.status_code != 200:
            error_msg = f"Ошибка смены приоритета: {response.status_code} - {response.text}"
            logging.error(error_msg)
            await query.message.reply_text(error_msg)
            return

        reply_markup = get_list_keyboard(category)
        await query.message.reply_text(f"Приоритет для '{item_name}' в {LISTS[category]} изменен на {PRIORITY_EMOJI[new_priority]}", reply_markup=reply_markup)
    except requests.RequestException as e:
        error_msg = f"Ошибка подключения к API: {e}"
        logging.error(error_msg)
        await query.message.reply_text(error_msg)
    except Exception as e:
        error_msg = f"Произошла ошибка: {str(e)}"
        logging.error(error_msg)
        await query.message.reply_text(error_msg)

async def button_callback(update: Update, context):
    """Обработчик нажатий на кнопки."""
    query = update.callback_query
    await query.answer()

    data = query.data
    if data == "restart":
        await start(update, context)
        return
    if data.startswith("add"):
        await add_start(update, context)
        return
    if data.startswith("add_to:"):
        await add_to_category(update, context)
        return
    if data.startswith("item:"):
        await show_item_actions(update, context)
        return
    if data.startswith("item_action:"):
        await handle_item_action(update, context)
        return
    if data.startswith("change_cat_to:"):
        await change_category_to(update, context)
        return
    if data.startswith("pri:"):
        await change_priority_to(update, context)
        return
    if data.startswith("list:"):
        list_type = data.split(":")[1]
        if list_type not in LISTS:
            await query.message.reply_text(f"Неизвестная категория: {list_type}")
            return

        try:
            auth_str = f"{USERNAME}:{PASSWORD}"
            auth_header = {"Authorization": f"Basic {b64encode(auth_str.encode()).decode()}"}

            response = requests.get(f"{API_URL}/list?category={list_type}", headers=auth_header)
            if response.status_code != 200:
                error_msg = f"Ошибка API: {response.status_code} - {response.text}"
                logging.error(error_msg)
                await query.message.reply_text(error_msg)
                return

            items = response.json()
            if not items:
                response_text = f"{LISTS[list_type]} пуст."
                reply_markup = get_list_keyboard(list_type)
                await query.message.reply_text(response_text, reply_markup=reply_markup)
                return

            response_text = f"{LISTS[list_type]}:\n"
            keyboard = []
            for item in items:
                priority = item["priority"]
                emoji = PRIORITY_EMOJI.get(priority, "🟡")
                response_text += f"- {emoji} {item['name']}\n"
                max_name_length = 50
                safe_name = item['name'][:max_name_length].encode('utf-8').decode('utf-8', 'ignore')
                callback_data = f"item:{safe_name}:{list_type}"
                if len(callback_data.encode('utf-8')) > 64:
                    logging.error(f"Callback data too long for item: {item['name']} in category: {list_type}")
                    continue
                keyboard.append([InlineKeyboardButton(item['name'], callback_data=callback_data)])

            keyboard.extend(get_list_keyboard(list_type).inline_keyboard)
            reply_markup = InlineKeyboardMarkup(keyboard)
            await query.message.reply_text(response_text, reply_markup=reply_markup)
        except requests.RequestException as e:
            error_msg = f"Ошибка подключения к API: {e}"
            logging.error(error_msg)
            await query.message.reply_text(error_msg)
        except json.JSONDecodeError:
            error_msg = "Ошибка: неверный формат данных от API."
            logging.error(error_msg)
            await query.message.reply_text(error_msg)
        except Exception as e:
            error_msg = f"Произошла ошибка: {str(e)}"
            logging.error(error_msg)
            await query.message.reply_text(error_msg)

def main():
    """Запуск бота."""
    if not TELEGRAM_TOKEN:
        logging.error("Ошибка: TELEGRAM_TOKEN не указан в .env")
        print("Ошибка: TELEGRAM_TOKEN не указан в .env")
        return
    if not USERNAME or not PASSWORD:
        logging.error("Ошибка: USERNAME или PASSWORD не указаны в .env")
        print("Ошибка: USERNAME или PASSWORD не указаны в .env")
        return
    if not API_URL:
        logging.error("Ошибка: API_URL не указан в .env")
        print("Ошибка: API_URL не указан в .env")
        return

    application = Application.builder().token(TELEGRAM_TOKEN).build()

    application.add_handler(CommandHandler("start", start))
    application.add_handler(CallbackQueryHandler(button_callback))
    application.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_item_text))

    print("Бот запущен...")
    application.run_polling(allowed_updates=Update.ALL_TYPES)

if __name__ == "__main__":
    main()
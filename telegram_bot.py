import os
from dotenv import load_dotenv
from telegram import InlineKeyboardButton, InlineKeyboardMarkup, Update
from telegram.ext import Application, CommandHandler, CallbackQueryHandler
import redis

# Загружаем переменные из .env
load_dotenv()

# Конфигурация из .env
TELEGRAM_TOKEN = os.getenv("TELEGRAM_TOKEN")  
REDIS_HOST = os.getenv("REDIS_HOST", "redis")
REDIS_PORT = int(os.getenv("REDIS_PORT", 6379))
REDIS_DB = int(os.getenv("REDIS_DB", 0))

# Подключение к Redis
redis_client = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, db=REDIS_DB, decode_responses=True)

async def start(update: Update, context):
    """Обработчик команды /start. Показывает кнопки для выбора списков."""
    keyboard = [
        [InlineKeyboardButton("Список покупок", callback_data="buy")],
        [InlineKeyboardButton("Список не забыть", callback_data="remember")],
    ]
    reply_markup = InlineKeyboardMarkup(keyboard)
    await update.message.reply_text("Выберите список:", reply_markup=reply_markup)

async def button_callback(update: Update, context):
    """Обработчик нажатий на кнопки."""
    query = update.callback_query
    await query.answer()

    list_type = query.data
    key = list_type  # Предполагаем, что ключи в Redis называются 'buy' или 'remember'

    try:
        # Получаем элементы из Redis (предполагаем, что это список)
        items = redis_client.lrange(key, 0, -1)
        if not items:
            await query.message.reply_text(f"Список '{list_type}' пуст.")
            return

        # Формируем ответ
        response = f"{list_type.capitalize()} список:\n" + "\n".join([f"- {item}" for item in items])
        await query.message.reply_text(response)
    except redis.RedisError as e:
        await query.message.reply_text(f"Ошибка при получении списка: {e}")

def main():
    """Запуск бота."""
    if not TELEGRAM_TOKEN:
        print("Ошибка: TELEGRAM_TOKEN не указан в .env")
        return

    application = Application.builder().token(TELEGRAM_TOKEN).build()

    # Добавляем обработчики
    application.add_handler(CommandHandler("start", start))
    application.add_handler(CallbackQueryHandler(button_callback))

    # Запускаем бота
    print("Бот запущен...")
    application.run_polling(allowed_updates=Update.ALL_TYPES)

if __name__ == "__main__":
    main()

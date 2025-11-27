#!/bin/bash

# Скрипт для запуска docker-compose с правильной загрузкой .env

set -e

cd "$(dirname "$0")/.."

# Загружаем переменные из .env
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

# Запускаем docker-compose из папки docker
cd docker
docker-compose "$@"


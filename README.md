Список покупок или того, что надо не забыть.
main.go - бекэнд сайта с api, работает в связке с redis.
alice.py - flask приложение для навыков Алисы, умеет добавить в определённый список, или прочитать всё из списка "купить/не забыть".
Всё это дело разворачивается через docker-compose, а
второй компоуз чисто для репликации редиса.

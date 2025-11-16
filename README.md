# Сервис назначения ревьюеров для Pull Request’ов
Внутри команды требуется единый микросервис, который автоматически назначает ревьюеров на Pull Request’ы (PR), 
а также позволяет управлять командами и участниками. 
Взаимодействие происходит исключительно через HTTP API.

## Технологии
- Go
- PostgreSQL
- Goose migrations
- Docker
- linter: golangci-lint

## Работа с проектом
Запуск проекта: генерация кода из openapi.yml, запуск линтера, запуск сервиса и его зависимостей в docker
```bash
make all
```
Запуск приложения
```bash
make run
```
Запуск тестов
```bash
make test-e2e
```

## Вопросы и проблемы, с которыми столкнулся по ходу решения
  1. Изучая openapi, узнал, что можно сгенерировать код из openapi.yml, и, чтобы сэкономить время, решил воспользоваться этим, в качестве генератора использовал oapi-codegen 
  2. В openapi.yml у метода /users/getReview нет ошибки 'Пользователь не найден', в работе над реальным проектом я бы уточнил требования (не надо ли добавить возможный ответ 404)
  3. Провёл нагрузочные тесты только над GET методами, так как не успел сделать генерацию данных для POST запросов, где требуются уникальные данные. Также я предположил, что основная нагрузка на сервис будет приходиться на GET методы
  4. В openapi.yml нет отдельного кода для случая, когда отправлен неверный запрос. Я решил возвращать http код 400, а в качестве кода ошибки использовал NOT_FOUND, в работе над реальным проектом я бы предложил добавить новый код ошибки в          openapi
  5. Метод GetPullRequestsByReviewer реализован не самым оптимальным способом с множественными запросами в БД (для каждого PR пользователя я делаю отдельный запрос в БД). Проверил в нагрузочных тестах, что данная реализация удовлетворяет            требованиям по SLI
  6. Когда создаётся команда с существующими пользователями, которые уже привязаны к другой команде, не понятно какую ошибку надо возвращать. Сейчас возвращается ошибка 500, в работе над реальным проектом я бы уточнил требования. 
  7. Разные названия полей в структуре и в примере для /pullRequest/reassign: в одном случае old_user_id, в другом old_reviewer_id. Решил, что ошибка в примере и использовал old_user_id

## Результаты при нагрузочных тестах
Провёл нагрузочные тесты локально с использованием инструмента vegeta для GET метода /team/get:
Для RPS=5:
```
echo "GET http://localhost:8080/team/get?team_name=team-1758733517832148421" | ./vegeta attack -duration=10s -rate=5 | tee results.bin | ./vegeta report
Requests      [total, rate, throughput]         50, 5.10, 5.10
Duration      [total, attack, wait]             9.801s, 9.8s, 1.706ms
Latencies     [min, mean, 50, 90, 95, 99, max]  1.027ms, 2.764ms, 1.885ms, 4.7ms, 7.597ms, 15.763ms, 15.763ms
Bytes In      [total, mean]                     6600, 132.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:50  
```
Для RPS=100:
```
echo "GET http://localhost:8080/team/get?team_name=team-1758733517832148421" | ./vegeta attack -duration=10s -rate=100 | tee results.bin | ./vegeta report
Requests      [total, rate, throughput]         1000, 100.09, 100.08
Duration      [total, attack, wait]             9.992s, 9.991s, 1.467ms
Latencies     [min, mean, 50, 90, 95, 99, max]  629.25µs, 1.675ms, 1.395ms, 2.597ms, 3.427ms, 6.937ms, 12.431ms
Bytes In      [total, mean]                     132000, 132.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:1000  
```
Для RPS=300:
```
echo "GET http://localhost:8080/team/get?team_name=team-1758733517832148421" | ./vegeta attack -duration=10s -rate=300 | tee results.bin | ./vege
ta report
Requests      [total, rate, throughput]         3000, 300.02, 299.98
Duration      [total, attack, wait]             10.001s, 9.999s, 1.315ms
Latencies     [min, mean, 50, 90, 95, 99, max]  503.564µs, 4.092ms, 1.403ms, 4.025ms, 7.421ms, 94.385ms, 187.251ms
Bytes In      [total, mean]                     396000, 132.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:3000
```

Для RPS=365:
```
echo "GET http://localhost:8080/team/get?team_name=team-1758733517832148421" | ./vegeta attack -duration=10s -rate=365 | tee results.bin | ./vegeta report
Requests      [total, rate, throughput]         3650, 365.15, 212.22
Duration      [total, attack, wait]             12.374s, 9.996s, 2.378s
Latencies     [min, mean, 50, 90, 95, 99, max]  651.279µs, 1.168s, 910.295ms, 2.436s, 2.978s, 8.669s, 9.179s
Bytes In      [total, mean]                     440840, 120.78
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           71.95%
Status Codes  [code:count]                      200:2626  500:1024  
Error Set:
500 Internal Server Error
```
По результатам тестов, можно увидеть, что требования задачи выполняются успешно с запасом, в данных условиях сервис поддерживает примерно 300 RPS 

Также провёл нагрузочные тесты локально с использованием инструмента vegeta для GET метода /users/getReview:
Для RPS=5:
```
echo "GET http://localhost:8080/users/getReview?user_id=u2" | ./vegeta attack -duration=10s -rate=5 | tee results.bin | ./vegeta report
 Requests      [total, rate, throughput]         50, 5.10, 5.10
Duration      [total, attack, wait]             9.807s, 9.799s, 7.621ms
Latencies     [min, mean, 50, 90, 95, 99, max]  3.658ms, 8.877ms, 7.614ms, 10.458ms, 17.938ms, 43.203ms, 43.203ms
Bytes In      [total, mean]                     49750, 995.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:50
```
Для RPS=10:
```
echo "GET http://localhost:8080/users/getReview?user_id=u2" | ./vegeta attack -duration=10s -rate=10 | tee results.bin | ./vegeta report
Requests      [total, rate, throughput]         100, 10.10, 10.09
Duration      [total, attack, wait]             9.908s, 9.9s, 7.184ms
Latencies     [min, mean, 50, 90, 95, 99, max]  3.504ms, 7.931ms, 7.43ms, 10.816ms, 11.198ms, 13.145ms, 13.725ms
Bytes In      [total, mean]                     99500, 995.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:100
```
Для RPS=100:
```
echo "GET http://localhost:8080/users/getReview?user_id=u2" | ./vegeta attack -duration=10s -rate=100 | tee results.bin | ./vegeta report
Requests      [total, rate, throughput]         1000, 100.10, 100.04
Duration      [total, attack, wait]             9.996s, 9.99s, 6.36ms
Latencies     [min, mean, 50, 90, 95, 99, max]  2.542ms, 8.147ms, 6.8ms, 9.896ms, 12.643ms, 55.296ms, 97.447ms
Bytes In      [total, mean]                     995000, 995.00
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           100.00%
Status Codes  [code:count]                      200:1000
```
Для RPS=150:  
```
echo "GET http://localhost:8080/users/getReview?user_id=u2" | ./vegeta attack -duration=10s -rate=150 | tee results.bin | ./vegeta report
Requests      [total, rate, throughput]         1492, 148.96, 137.54
Duration      [total, attack, wait]             10.557s, 10.016s, 540.986ms
Latencies     [min, mean, 50, 90, 95, 99, max]  2.89ms, 384.795ms, 273.762ms, 930.304ms, 1.156s, 1.791s, 2.316s
Bytes In      [total, mean]                     1448420, 970.79
Bytes Out     [total, mean]                     0, 0.00
Success       [ratio]                           97.32%
Status Codes  [code:count]                      200:1452  500:40  
Error Set:
500 Internal Server Error
```
По результатам тестов, можно увидеть, что требования задачи выполняются успешно с запасом, в данных условиях сервис поддерживает примерно 100 RPS. Более низкие показатели RPS объясняется тем, что в этом методе используется неоптимальный поиск в БД( [Подробнее в Вопросы и проблемы, с которыми столкнулся по ходу решения](#вопросы-и-проблемы-с-которыми-столкнулся-по-ходу-решения) )


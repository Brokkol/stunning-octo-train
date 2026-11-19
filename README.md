# Mini Imgur

Учебный проект: загружаешь картинку, сервер делает из неё превью, всё видно в приложении (Android, iOS, web).

## Как это работает

Flutter (Android / iOS / web) -> Cloudflare -> Nginx -> Go API -> RabbitMQ -> воркер

1. В приложении жму "+", фото уходит на `POST /images`.
2. API кладёт оригинал в SeaweedFS (S3), пишет запись в MongoDB со статусом `pending` и кидает задачу в RabbitMQ.
3. Воркер уменьшает картинку до 300px, кладёт превью в S3 и ставит статус `done`.
4. Воркер публикует событие в Redis (канал `image_events`), API рассылает его по WebSocket (`/ws`) только владельцу картинки, и `pending` в приложении сам превращается в превью без опроса.

Вход через Google (Firebase): приложение шлёт ID-токен в заголовке `Authorization: Bearer ...`, API проверяет его по ключам Google. У каждого пользователя своя галерея, чужие картинки отдают 404. Для WebSocket токен передаётся параметром `?token=`.

Redis используется для кэша пользователей и как шина событий (pub/sub) между воркером и API.

## Что внутри (Docker Compose)

- Приложение: `nginx` (раздаёт Flutter web и проксирует API), `app` (Go API), `worker`, `mongo`, `redis`, `s3` (SeaweedFS), `rabbitmq`
- Метрики: Prometheus, Grafana, redis/mongo/node exporters
- Логи: Vector -> Logstash -> Elasticsearch -> Kibana, плюс `log-dumper` (складывает логи в S3)
- Алерты: Zabbix (диск, память и прочие проблемы сервера) и Logstash (ошибки приложения) шлют сообщения в Telegram
- Доступ снаружи: `cloudflared` (Cloudflare Tunnel) + Cloudflare Access

## Как запустить

1. Создать `.env` рядом с `docker-compose.yml`:
   ```
   TUNNEL_TOKEN=...
   KIBANA_ENCRYPTION_KEY=...    # секрет, от 32 символов: openssl rand -hex 32
   TELEGRAM_BOT_TOKEN=...       # токен бота из @BotFather
   TELEGRAM_CHAT_ID=...         # куда слать алерты (число)
   ```
   Все значения секретные, `.env` в git не попадает. Пример без значений лежит в `.env.example`.
2. Запустить:
   ```
   docker compose up -d
   ```
3. Сайт: http://localhost (Nginx на 80 порту), статика Flutter web лежит в `web-dist/`.

Приложение (Android/iOS), из папки `frontend`:
```
cp secrets.example.json secrets.json   # заполнить CF_ID, CF_SECRET, BASE_URL
flutter run --dart-define-from-file=secrets.json
```
APK: `flutter build apk --release --dart-define-from-file=secrets.json`
Web: `flutter build web --release`, результат копировать в `web-dist/`.

`secrets.json` и `.env` в git не попадают.

## Доступ и безопасность

Всё снаружи закрыто Cloudflare Access: API открывается по сервисному токену (приложение шлёт заголовки `CF-Access-Client-Id` / `CF-Access-Client-Secret`), мониторинг (Grafana, Kibana, Zabbix, Prometheus, RabbitMQ) по почте через App Launcher. Сайт и web-версия работают с того же домена, без заголовков. `/metrics` снаружи закрыт (404).

## Вход через Google (web и Android)

На web вход открывает всплывающее окно (`signInWithPopup`), домен сайта должен быть в Firebase -> Authentication -> Settings -> Authorised domains.
На Android всплывающее окно не работает, поэтому используется нативный `google_sign_in`: он получает ID-токен и передаёт его в Firebase. Для этого в Firebase нужны отпечатки SHA-1 и SHA-256 ключа, которым подписана сборка, и свежий `google-services.json` после их добавления.

## Метрики и дашборд

Prometheus собирает метрики API и воркера, Grafana рисует их. Свои метрики:

- `users_total` - сколько пользователей зарегистрировано (растёт при первом `GET /me`)
- `images_total` - сколько картинок в базе
- `images_uploaded_total{format}` - сколько загружено, по форматам
- `http_requests_total` и `http_request_duration_seconds` - запросы и время ответа по маршрутам

Дашборд "Mini Imgur" лежит в `grafana/dashboard.json`. Импорт: Grafana -> Dashboards -> Import -> загрузить файл (при том же uid старый дашборд перезапишется). Дашборды для Mongo, Redis и сервера импортированы с grafana.com. Источники данных подключаются сами (`grafana/provisioning/datasources/`).

## Алерты

- **Zabbix -> Telegram.** Тип уведомления Telegram включён с токеном бота, у пользователя Admin добавлен Telegram с chat id, правило "Send problems to Telegram" (Alerts -> Actions) шлёт проблемы. Проверка: временно занизить порог диска `{$VFS.FS.PUSED.MAX.WARN}` на хосте, потом вернуть.
- **Ошибки приложения -> Telegram.** Приложение пишет логи в JSON (`slog`), они идут Vector -> Logstash. В `logstash/pipeline.conf` строки с `level == "ERROR"` дополнительно отправляются в Telegram через выход `http`. Токен и chat id берутся из `.env`. Проверка (изнутри сети Docker): `docker compose exec logstash bash -c "echo '{\"level\":\"ERROR\",\"msg\":\"test\"}' > /dev/tcp/127.0.0.1/5044"`.
- Kibana: для Rules и Connectors нужен ключ шифрования (`KIBANA_ENCRYPTION_KEY`). Коннектор Webhook в Kibana платный, поэтому алерты на ошибки сделаны через Logstash.

## Что умеет API

Все запросы, кроме `/metrics`, требуют токен Firebase.

- `GET /me` - профиль текущего пользователя (при первом входе создаёт запись в Mongo)
- `POST /images` - загрузить картинку (jpg, png, heic/heif, до 20 МБ)
- `GET /images` - список своих картинок
- `GET /images/{id}` - одна запись (только владельцу)
- `GET /ws` - WebSocket: события о готовности картинок
- `GET /images/{id}/original` - оригинал
- `GET /images/{id}/thumbnail` - превью
- `GET /metrics` - метрики для Prometheus (только внутри сети): запросы и их длительность, `users_total`, `images_total`, `images_uploaded_total{format}`

Документация API: Swagger UI на `/swagger/` (описание в `docs/openapi.yml`, кнопка Authorize принимает токен).

## Тесты и CI

```
MONGO_TEST_URI=mongodb://localhost:27017 go test -count=1 -v ./...
```

Тесты с базой создают временную Mongo-базу и удаляют её; без `MONGO_TEST_URI` они пропускаются.
GitHub Actions на каждый пуш запускает `go vet` и тесты (с контейнером Mongo), а при мерже в `main` собирает образ и кладёт его в `ghcr.io/brokkol/mini-imgur`. Как развернуть на сервере, читай в `DEPLOY.md`.

## Бэкапы и откаты

Контейнеры `mongo-backup` и `s3-backup` раз в сутки делают архивы в `./backups`. Образы тегируются версией (`TAG=$(git describe --always --dirty)`), откат на старую версию: `TAG=<версия> docker compose up -d`. Откат возвращает только код, данные не трогает.

HEIC декодируется на чистом Go (`gen2brain/heic`), CGO не нужен.

## Что не доделано

- В nginx нет заголовков `Cache-Control` (`no-cache` для сайта, `no-store` для API), и в Cloudflare не включено "Respect Existing Headers": после обновления браузер и Cloudflare могут показывать старую версию. После `git pull` на сервере: `docker compose up -d --force-recreate nginx`.
- Дашборд Grafana загружается руками (Import), а не из репозитория автоматически.
- Вход на Android с новой сборкой надо проверить на телефоне.
- Нет удаления картинок, удалял руками через mongosh.
- Если RabbitMQ перезапустится, API теряет соединение, помогает `docker compose restart app worker`. Надо сделать переподключение.
- У запросов к Mongo нет таймаутов (`context.WithTimeout`), и у `http.Server` нет graceful shutdown.
- Нет шаринга: картинку видит только её владелец.
- Пароли по умолчанию (Grafana, Zabbix, RabbitMQ) надо вынести в `.env` и сменить.
- Порты сервисов проброшены наружу, на сервере надо привязать к `127.0.0.1`.
- ELK тяжёлый, на 8 ГБ оперативки запускать с осторожностью.

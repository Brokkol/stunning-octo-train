# Mini Imgur

Учебный проект: загружаешь картинку, сервер делает из неё превью, всё видно в приложении (Android, iOS, web).

## Как это работает

Flutter (Android / iOS / web) -> Cloudflare -> Nginx -> Go API -> RabbitMQ -> воркер

1. В приложении жму "+", фото уходит на `POST /images`.
2. API кладёт оригинал в SeaweedFS (S3), пишет запись в MongoDB со статусом `pending` и кидает задачу в RabbitMQ.
3. Воркер уменьшает картинку до 300px, кладёт превью в S3 и ставит статус `done`.
4. Приложение каждые 3 секунды спрашивает список, поэтому `pending` сам превращается в превью.

Redis пока используется только для кэша пользователей.

## Что внутри (Docker Compose)

- Приложение: `nginx` (раздаёт Flutter web и проксирует API), `app` (Go API), `worker`, `mongo`, `redis`, `s3` (SeaweedFS), `rabbitmq`
- Метрики: Prometheus, Grafana, redis/mongo/node exporters
- Логи: Vector -> Logstash -> Elasticsearch -> Kibana, плюс `log-dumper` (складывает логи в S3)
- Алерты: Zabbix
- Доступ снаружи: `cloudflared` (Cloudflare Tunnel) + Cloudflare Access

## Как запустить

1. Создать `.env` рядом с `docker-compose.yml`:
   ```
   TUNNEL_TOKEN=...
   ```
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

## Что умеет API

- `POST /images` - загрузить картинку (jpg, png, heic/heif, до 20 МБ)
- `GET /images` - список картинок
- `GET /images/{id}` - одна запись
- `GET /images/{id}/original` - оригинал
- `GET /images/{id}/thumbnail` - превью
- `GET /metrics` - метрики для Prometheus (только внутри сети)

HEIC декодируется на чистом Go (`gen2brain/heic`), CGO не нужен.

## Что не доделано

- Нет удаления картинок, удалял руками через mongosh.
- Если RabbitMQ перезапустится, API теряет соединение, помогает `docker compose restart app worker`. Надо сделать переподключение.
- У запросов к Mongo нет таймаутов (`context.WithTimeout`), и у `http.Server` нет graceful shutdown.
- Доступ по общему ключу, а не по пользователям.
- Приложение всё время опрашивает сервер, даже когда все картинки готовы.
- Пароли по умолчанию (Grafana, Zabbix, RabbitMQ) надо вынести в `.env` и сменить.
- Порты сервисов проброшены наружу, на сервере надо привязать к `127.0.0.1`.
- ELK тяжёлый, на 8 ГБ оперативки запускать с осторожностью.

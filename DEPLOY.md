# Как развернуть на сервере

Образ приложения собирает GitHub Actions и кладёт в ghcr (`ghcr.io/brokkol/mini-imgur`)
при каждом мерже в `main`. Сервер ничего не собирает, а только скачивает готовый образ.
Остальные файлы (nginx.conf, prometheus, grafana, scripts, web-dist) берутся из репозитория.

## 1. Подготовка сервера (Ubuntu Server)

```
sudo apt update && sudo apt install -y git
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER   # потом выйти и зайти по SSH заново
```

## 2. Файрвол (важно)

В `docker-compose.yml` многие порты проброшены наружу (Mongo 27017, Redis 6379, RabbitMQ, Grafana, Kibana...).
Docker обходит `ufw`, поэтому закрывать надо на уровне хостера: в Hetzner Cloud Console -> Firewalls
оставить входящими только `22/tcp` (SSH). Снаружи сайт открывается через Cloudflare Tunnel,
ему входящие порты не нужны.

## 3. Код и секреты

```
git clone https://github.com/Brokkol/stunning-octo-train.git mini-imgur
cd mini-imgur
cp .env.example .env
nano .env        # вписать TUNNEL_TOKEN; IMAGE и TAG уже заданы под ghcr
```

`.env` и `secrets.json` в git не попадают, на сервер их кладём руками.

## 4. Запуск

```
docker compose pull app          # скачать образ из ghcr (worker и log-dumper используют тот же)
docker compose up -d --no-build  # --no-build: ничего не собирать на сервере
docker compose ps
```

Если `pull` просит логин, значит пакет приватный: GitHub -> Packages -> mini-imgur ->
Package settings -> Change visibility -> Public (или `docker login ghcr.io`).

## 5. Обновление и откат

Новая версия: смержить PR в `main`, дождаться зелёного `publish`, потом на сервере

```
docker compose pull app && docker compose up -d --no-build
```

Откат на конкретную версию (тег = первые 7 символов коммита из ghcr):

```
TAG=2ad8811 docker compose up -d --no-build
```

Откат возвращает только код, данные в базе остаются как есть.

## 6. Бэкапы

Контейнеры `mongo-backup` и `s3-backup` пишут архивы в `./backups` раз в сутки (Mongo хранит 7, S3 хранит 3).
Восстановление Mongo:

```
docker compose exec -T mongo mongorestore --archive --gzip --drop < backups/app_db-<дата>.gz
```

## 7. Перенос на другой сервер

Настройки, сделанные мышкой в интерфейсах (Zabbix, Kibana, Grafana), лежат в volumes, а не в репозитории.
Перед переездом сделать их копию, иначе на новом месте всё будет пустым.
Дашборды Grafana держим в репозитории (`grafana/`), источники данных уже подключаются автоматически.

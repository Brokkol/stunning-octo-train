#!/bin/sh
# Раз в сутки делает дамп Mongo и хранит последние 7 файлов
while true; do
  FILE="/backups/app_db-$(date +%F-%H%M).gz"
  echo "backup -> $FILE"
  if mongodump --host mongo --db app_db --archive="$FILE" --gzip; then
    echo "ok"
  else
    echo "FAILED"
    rm -f "$FILE"   # не оставляем битый файл
  fi
  # оставить 7 самых новых, остальные удалить
  ls -1t /backups/app_db-*.gz | tail -n +8 | xargs -r rm --
  sleep 86400   # сутки
done
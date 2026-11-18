#!/bin/sh
# Раз в сутки упаковывает файлы S3 в архив, хранит последние 3
while true; do
  FILE="/backups/s3-$(date +%F-%H%M).tar.gz"
  echo "backup -> $FILE"
  # Без дампов логов: они лежат отдельными томами logs_*, быстро раздувают архив
  FILES=$(cd /s3 && ls -A | grep -v '^logs_')
  tar czf "$FILE" -C /s3 $FILES
  RC=$?
  if [ "$RC" -le 1 ]; then   # 1 = файл менялся во время чтения, не авария
    echo "ok"
  else
    echo "FAILED"
    rm -f "$FILE"
  fi
  ls -1t /backups/s3-*.tar.gz | tail -n +4 | xargs -r rm --   # оставить 3 новых
  sleep 86400
done
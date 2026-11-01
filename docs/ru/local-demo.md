# Приватное локальное демо

Демо запускается на вашем компьютере. Общего сервера и личной музыки в нём нет.

1. Установите Docker Desktop и скачайте исходники по [быстрому старту](quickstart.md). Выберите отдельную пустую папку для демонстрационной музыки.
2. Из папки исходников создайте оригинальные короткие записи и процедурные обложки:

```sh
mkdir -p demo-music
docker build --platform linux/amd64 -f Dockerfile.family-doctor -t family-doctor:local .
docker run --rm --entrypoint python3 -v "$PWD:/src:ro" -v "$PWD/demo-music:/demo" family-doctor:local /src/scripts/create-demo-library.py /demo
```

Генератор не перезаписывает существующее аудио. Для повторного запуска используйте новую папку. Он создаёт шесть вымышленных альбомов, двенадцать коротких MP3/FLAC и файл происхождения. Сгенерированные записи и обложки — CC0; код генератора — GPLv3.

3. Укажите абсолютный путь `demo-music` как `FMH_MUSIC_DIR` в `.env` и запустите публичный Docker-профиль по Quick Start. Если другой экземпляр уже существует, задайте отдельное имя проекта: `docker compose -p fmh-demo -f compose.family.yml up -d --build`. Если порт 4533 занят, предварительно измените опубликованный порт.
4. Прочитайте setup-token именно этого экземпляра, создайте вымышленного администратора и участника. Явно выдайте участнику доступ к демобиблиотеке.
5. Создайте полку, вымышленное воспоминание и запрос по [руководствам модулей](features/index.md). Не импортируйте личные копии ради скриншотов.

Остановка: `docker compose -p fmh-demo -f compose.family.yml down`. Том и музыка сохранятся. Удаляйте их отдельно только при осознанном отказе от демо. Общедоступных паролей и онлайн-демо проект не предоставляет.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Неофициальный форк Navidrome; GPLv3.

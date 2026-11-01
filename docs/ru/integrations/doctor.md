# Library Doctor Companion на Mac

Сервер выполняет ограниченные проверки метаданных. SHA-256, decoded PCM и Chromaprint анализируются только после явного запуска Mac companion. Это не постоянный облачный worker; оригинальная музыка не исправляется, не удаляется и не перетегируется.

1. Установите Docker Desktop и соберите инструмент из того же релиза: `docker build --platform linux/amd64 -f Dockerfile.family-doctor -t navidrome-family-doctor:local .`. Образ остаётся локальным; companion не скачивает отсутствующий образ автоматически.
2. Как администратор откройте **Family Home → More → Library Doctor**. Выберите библиотеки, лимиты и проверки, прочитайте объём работ и создайте ограниченное deep-задание. Скопируйте ID.
3. Companion использует существующий пароль администратора в macOS Keychain — тот же service, что [Family Sync](mac-sync.md). Не передавайте пароль аргументом или JSON-файлом. При необходимости сохраните существующий credential защищённым интерактивным установщиком Sync; новый токен не требуется.
4. Из папки релиза выполните:

```sh
python3 macos/navidrome_family_doctor.py --server https://music.example.invalid --owner admin --job YOUR_JOB_ID
```

Замените endpoint, имя и ID своими. Если Sync настроен, сервер и имя по умолчанию берутся оттуда. Образ по умолчанию — `navidrome-family-doctor:local`.

5. Следите за заданием в WebUI. Каждый локальный analysis-контейнер не имеет сети, доступен только для чтения, лишён capabilities и ограничен 2 CPU, 512 MB, 64 процессами и небольшим tmpfs. Авторизованный companion приватно получает исходное аудио; серверу возвращаются только ограниченные результаты анализа.
6. Ctrl-C запрашивает паузу; прогресс хранится в SQLite сервера. Возобновите той же командой с `--resume`; `--retry` используйте после проверки причины сбоя. Не запускайте два companion для одной работы.

Совпадение названия не доказывает акустический дубль. Высокая частота не доказывает настоящий high-res источник. Смотрите confidence и unknown/incomplete в [паспорте](../features/passport.md). Недоступность оригинала, провайдера, декодера или дедлайн дают неполный результат, а не выдуманный вердикт. При сбое проверьте Docker Desktop, labels/версию локального образа, Keychain и задание. Не прикладывайте личное аудио, секреты и кеши анализа к issue.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Неофициальный форк Navidrome; GPLv3.

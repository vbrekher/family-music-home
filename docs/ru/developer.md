# Руководство разработчика

Это неофициальный GPLv3-форк Navidrome. Начните с [различий upstream](../../UPSTREAM_DIFFERENCES.ru.md), [архитектуры](architecture.md), [правил участия](../../CONTRIBUTING.md) и [дизайн-системы](../design/DESIGN_SYSTEM.md). Сохраняйте исходные copyright и сторонние лицензии.

Релиз собирается локально в Docker Desktop для `linux/amd64`:

```sh
docker build --platform linux/amd64 -f Dockerfile.family -t family-music-home:local .
```

Dockerfile с фиксированными базовыми образами компилирует React и Go из этих исходников. Python-runtime поддерживает первый запуск и необязательные Bunny-копии. Для публичного запуска не нужны приватные deployment helpers, старая Git-история или Bunny.

Зависимости UI зафиксированы в `ui/package-lock.json`. В локальном Node Docker-окружении выполните `npm ci --legacy-peer-deps`, затем `npm test -- --run --maxWorkers=1` и `npm run build`. UUID-alias для старого импорта JSONForms намеренный. Локальная security-правка deepmerge описана в `ui/vendor/deepmerge/PATCH.md`; не заменяйте её уязвимой upstream-версией без проверки advisory.

Go требует CGO, C toolchain и SQLite FTS5. В локальном Go Docker-окружении установите gcc/musl-dev/zlib-dev/git и выполните `go test -mod=readonly -p 1 -tags netgo,sqlite_fts5 ./...`. При необходимости соберите WASM test fixtures из их Makefile; не подставляйте приватные плагины. Python-тестам нужны requirements соответствующей интеграции.

Используйте синтетические аккаунты, музыку и disposable volumes. Не подключайте production SQLite к коду разработки. Проверяйте права постороннего участника и защищённого ребёнка, не только администратора. Локальный тест не доказывает проверку физического NAS/телефона или production.

Публичные обновления проходят [безопасный экспорт](../../scripts/export_public_release.py): фиксированные committed blobs, проверенный allowlist путей/хешей, локальный denylist приватных значений, secret scan, независимую сборку и новую публичную историю. Не зеркалируйте private refs. GitHub Actions, облачные сборки и автоматическое назначение reviewers не требуются.

---
Project Owner / Maintainer: [@aitishnyk](https://github.com/aitishnyk). Project Co-author: [@krisbrekher](https://github.com/krisbrekher). Неофициальный форк Navidrome; GPLv3.

## Данные регрессии upstream

Релиз не переиздаёт upstream-аудио/обложки и опубликованные TLS test keys. Перед Go-тестами явно выполните `python3 scripts/fetch-upstream-test-fixtures.py` в локальном Docker dev-окружении. Скрипт получает фиксированную публичную версию Navidrome, проверяет SHA-256 lock, отказывает при несовпадении/перезаписи и восстанавливает только проверенные тестовые symlinks внутри fixtures. Это данные разработки, не демомузыка и не production-сертификаты. Сборке приложения и синтетическому демо скачивание не нужно. Не включайте полученные fixtures в публичные архивы и коммиты.

## Воспроизводимые локальные тесты в Docker

```sh
docker build --platform linux/amd64 -f Dockerfile.test -t family-music-home-tests:local .
docker run --rm --platform linux/amd64 --cpus 2 --memory 2g family-music-home-tests:local
docker build --platform linux/amd64 --target ui -f Dockerfile.family -t family-music-home-ui:local .
docker run --rm --platform linux/amd64 --cpus 2 --memory 2g family-music-home-ui:local npx vitest run --maxWorkers=2 --testTimeout=60000
docker run --rm --platform linux/amd64 --entrypoint python3 -v "$PWD:/src:ro" -w /src family-music-home:local -m unittest discover -s tests/bunny_ops -v
```

Go test image получает проверенные upstream fixtures и собирает WASM test plugins из исходников. Он работает с UID 502: проверки недоступных файлов остаются значимыми. Не публикуйте этот образ: полученные sample media и публичные test keys не являются релизными файлами. Последняя команда предполагает уже собранный выше `family-music-home:local`. Публичный экспорт исключает приватные deployment-guard tests, зависящие от инструментов владельца. Расширенный унаследованный cross-compilation рецепт сохранён как `Dockerfile.upstream`; основной `Dockerfile` использует тот же защищённый self-host профиль, что `Dockerfile.family`.

## Реальная приёмка чистой установки

На компьютере нужны Docker Desktop и Python 3. Используйте **новый одноразовый том** и исходное демо из шести альбомов. Проверка отказывается работать с контейнером без специальной метки, внешним портом или существующей установкой без setup-токена. Она создаёт только вымышленные аккаунты, проверяет MP3/FLAC Range и отзыв доступа, перезапускает приложение и при указанном флаге полностью восстанавливает зашифрованную копию остановленного тома. Сгенерированные пароли и временный ключ остаются в памяти. Не подключайте личные данные.

```sh
mkdir -p demo-music
docker run --rm --platform linux/amd64 --entrypoint python3 -v "$PWD:/src:ro" -v "$PWD/demo-music:/demo" family-music-home:local /src/scripts/create-demo-library.py /demo
docker run -d --platform linux/amd64 --name family-clean-check --label net.family-music-home.disposable=true --cpus 2 --memory 1g -p 127.0.0.1:45549:4533 -v family-clean-check-data:/data -v "$PWD/demo-music:/music:ro" family-music-home:local
python3 tests/selfhost_acceptance.py --container family-clean-check --restore-check
```

Исходный тестовый контейнер и том сохраняются для осмотра. Временная восстановленная копия удаляется. Для повторного запуска выберите новые имена. Используется стандартный пользователь образа, как в Compose. Если задаёте собственные UID/GID, предварительно настройте владельца нового тома: Docker не выдаёт ему запись в пустой том с владельцем root автоматически.

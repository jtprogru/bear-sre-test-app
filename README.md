# bear-sre-test-app

Основная цель проекта - тестовое задание для DevOps/SRE, направленное на проверку знаний по траблшутингу и устранению найденных проблем.

## Запуск

- Положить в файл `.env` переменную `SRV_ADDR=8000`;
- Выполнить `make run.cmd`;

## Список проблем

Основной список проблем, которые потенциальный кандидат должен решить:
- Не корректный путь подключаемой `.so` библиотеки;
- Пример SO можно глянуть вот [тут](https://github.com/hANSIc99/library_sample);
- Положить правильный конфигурационный файл;
- В конфигурационном файле должны быть заданы правильные поля и правильные значения;
- Поправить файл конфигурации Nginx;
- Решить проблему с SSL и самоподписанным сертификатом в приложении;
- etc...


Список ендпоинтов:
- `/` - Home page;
- `/ping` - Если приложение запущено, то отдается `pong`;
- `/selfcheck` - Если в конфиге правильные данные, то отдает `OK`, иначе `NOK`;
- `/pub` - Если в конфиге выставлен правильно параметр, отдает ссылку на публичный чат и на tg-канал, а так же на discord-сервер;
- `/secret` - Если передается заголовок `X-SRE-TEST` со значением `foobar`, отдает ссылку на приватный чат;


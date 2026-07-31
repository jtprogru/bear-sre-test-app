// Package conformance — чёрный ящик поверх собранного сервиса.
//
// Набор ничего не знает о внутреннем устройстве приложения: он запускает
// бинарь и разговаривает с ним по HTTP. Благодаря этому он одинаково применим
// и к эталонной реализации в main, и к тому, что напишет кандидат на ветке
// task/*, — контракт зафиксирован на границе HTTP, а не на сигнатурах Go.
//
// Запуск:
//
//	CONFORMANCE_BINARY=./dist/bear-sre-test-app go test -tags conformance -v ./conformance/...
//
// или просто `task test:conformance`.
package conformance

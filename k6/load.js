// Нагрузочный профиль для проверки p99 и стабильности под нагрузкой.
//
// Запуск (сервис должен быть уже поднят):
//   task load
//   BASE_URL=http://127.0.0.1:8080 k6 run k6/load.js
import http from 'k6/http';
import { check, group } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8080';

export const options = {
  stages: [
    { duration: '10s', target: 20 },
    { duration: '30s', target: 50 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    // Порог намеренно жёсткий: ручки не делают ничего тяжёлого, и любой
    // выход за 50мс на p99 означает проблему — блокировку, аллокации
    // на горячем пути или отсутствие переиспользования соединений.
    'http_req_duration{expected_response:true}': ['p(99)<50'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  group('ping', () => {
    const res = http.get(`${BASE_URL}/ping`);
    check(res, {
      'ping is 200': (r) => r.status === 200,
      'ping is json': (r) => (r.headers['Content-Type'] || '').includes('application/json'),
    });
  });

  group('secret', () => {
    const res = http.get(`${BASE_URL}/secret`, {
      headers: { 'X-IAM-SRE': 'sre' },
    });
    check(res, {
      'secret is not 401': (r) => r.status !== 401,
    });
  });

  group('not found', () => {
    const res = http.get(`${BASE_URL}/definitely-not-a-route`);
    check(res, {
      'unknown path is 404': (r) => r.status === 404,
    });
  });
}
